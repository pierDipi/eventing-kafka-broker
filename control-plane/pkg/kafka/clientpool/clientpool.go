/*
 * Copyright 2023 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package clientpool

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/kafka"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/prober"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/security"
)

type KafkaClientKey struct{}

var ctxKey = KafkaClientKey{}

func WithKafkaClientPool(ctx context.Context) context.Context {
	cache := prober.NewLocalExpiringCache[clientKey, *client, struct{}](ctx, time.Minute*30)

	clients := &ClientPool{
		cache:                     cache,
		newSaramaClient:           sarama.NewClient,
		newClusterAdminFromClient: sarama.NewClusterAdminFromClient,
	}

	return context.WithValue(ctx, ctxKey, clients)
}

// A comparable struct that includes the info we need to uniquely identify a kafka cluster admin
type clientKey struct {
	secretName       string
	secretNamespace  string
	bootstrapServers string
}

type ClientPool struct {
	lock                      sync.RWMutex
	cache                     prober.Cache[clientKey, *client, struct{}]
	newSaramaClient           kafka.NewClientFunc // use this to mock the function for tests
	newClusterAdminFromClient kafka.NewClusterAdminFromClientFunc
	registeredInformer        bool // use this to track whether the secret informer has been registered yet
}

type GetKafkaClientFunc func(ctx context.Context, bootstrapServers []string, secret *corev1.Secret) (sarama.Client, error)
type GetKafkaClusterAdminFunc func(ctx context.Context, bootstrapServers []string, secret *corev1.Secret) (sarama.ClusterAdmin, error)

func (cp *ClientPool) GetClient(ctx context.Context, bootstrapServers []string, secret *corev1.Secret) (sarama.Client, error) {
	// (bootstrapServers, secret) uniquely identifies a sarama client config with the options we allow users to configure
	key := makeClusterAdminKey(bootstrapServers, secret)

	logger := logging.FromContext(ctx).
		With(zap.String("component", "clientpool")).
		With(zap.Any("key", key))

	logger.Debug("Getting client")

	cp.lock.Lock()
	defer cp.lock.Unlock()

	// if a corresponding connection already exists, lets use it
	val, ok := cp.cache.Get(key)
	if ok && val.secret.ResourceVersion == secret.ResourceVersion {
		logger.Debug("Successfully got a client")
		val.incrementCallers()
		return val, nil
	}
	logger.Debug("No existing client present, creating a new one")

	saramaClient, err := cp.makeSaramaClient(bootstrapServers, secret)
	if err != nil {
		return nil, err
	}

	val = &client{
		client: saramaClient,
		isFatalError: func(err error) bool {
			return strings.Contains(err.Error(), "broken pipe")
		},
		onFatalError: func(err error) {
			cp.cache.Expire(key)
		},
		secret: secret,
	}
	val.incrementCallers()

	cp.cache.UpsertStatus(key, val, struct{}{}, func(_ clientKey, value *client, _ struct{}) {
		// async: to avoid blocking UpsertStatus if the key is present.
		go func() {
			logger.Debugw("Closing client, waiting for callers to finish operations")

			// Wait for callers to finish their operations.
			value.callersWg.Wait()

			logger.Debugw("Closing client")

			// Close the real client.
			if err := value.client.Close(); !errors.Is(err, sarama.ErrClosedClient) {
				logger.Errorw("Failed to close client", zap.Error(err))
			}
		}()
	})

	return val, nil
}

func makeClusterAdminKey(bootstrapServers []string, secret *corev1.Secret) clientKey {
	sort.SliceStable(bootstrapServers, func(i, j int) bool {
		return bootstrapServers[i] < bootstrapServers[j]
	})

	key := clientKey{
		bootstrapServers: strings.Join(bootstrapServers, ","),
	}

	if secret != nil {
		key.secretName = secret.GetName()
		key.secretNamespace = secret.GetNamespace()
	}

	return key
}

func (key clientKey) matchesSecret(secret *corev1.Secret) bool {
	if secret == nil {
		return key.secretName == "" && key.secretNamespace == ""
	}
	return key.secretName == secret.GetName() && key.secretNamespace == secret.GetNamespace()
}

func (key clientKey) getBootstrapServers() []string {
	return strings.Split(key.bootstrapServers, ",")
}

func (cp *ClientPool) makeSaramaClient(bootstrapServers []string, secret *corev1.Secret) (sarama.Client, error) {
	config, err := kafka.GetSaramaConfig(security.NewSaramaSecurityOptionFromSecret(secret), kafka.DisableOffsetAutoCommitConfigOption)
	if err != nil {
		return nil, err
	}

	saramaClient, err := cp.newSaramaClient(bootstrapServers, config)
	if err != nil {
		return nil, err
	}

	return saramaClient, nil
}

func (cp *ClientPool) GetClusterAdmin(ctx context.Context, bootstrapServers []string, secret *corev1.Secret) (sarama.ClusterAdmin, error) {
	c, err := cp.GetClient(ctx, bootstrapServers, secret)
	if err != nil {
		return nil, err
	}
	ca, err := cp.newClusterAdminFromClient(c)
	if err != nil {
		return nil, err
	}
	return ca, nil
}

func Get(ctx context.Context) *ClientPool {
	return ctx.Value(ctxKey).(*ClientPool)
}
