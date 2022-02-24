/*
 * Copyright 2022 The Knative Authors
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

package autoscaler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/reconciler"

	kafkainternals "knative.dev/eventing-kafka-broker/control-plane/pkg/apis/internals/kafka/eventing/v1alpha1"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/consumer"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/kafka"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/security"
)

type Reconciler struct {
	// NewKafkaClient creates new sarama Client. It's convenient to add this as Reconciler field so that we can
	// mock the function used during the reconciliation loop.
	NewKafkaClient kafka.NewClientFunc

	SecretLister corelisters.SecretLister
	KubeClient   kubernetes.Interface

	Measurements Measurements
}

func (r *Reconciler) ReconcileKind(ctx context.Context, cg *kafkainternals.ConsumerGroup) reconciler.Event {
	if !cg.IsReady() {
		return nil
	}

	securityOption, err := r.securityOption(ctx, cg)
	if err != nil {
		return fmt.Errorf("failed to extract security options: %w", err)
	}

	saramaConfig, err := kafka.GetSaramaConfig(securityOption)
	if err != nil {
		return fmt.Errorf("failed to create Kafka client: %w", err)
	}
	bootstrapServers := strings.Split(cg.Spec.Template.Spec.Configs.Configs["bootstrap.servers"], ",")
	kafkaClient, err := sarama.NewClient(bootstrapServers, saramaConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kafka client: %w", err)
	}

	autoOffsetReset := r.autoOffsetReset(cg)
	p := kafka.NewConsumerGroupLagProvider(kafkaClient, sarama.NewClusterAdminFromClient, autoOffsetReset)

	groupId := cg.Spec.Template.Spec.Configs.Configs["group.id"]
	lags := make([]kafka.ConsumerGroupLag, 0, len(cg.Spec.Template.Spec.Topics))
	for _, t := range cg.Spec.Template.Spec.Topics {
		lag, err := p.GetLag(t, groupId)
		if err != nil {
			return fmt.Errorf("failed to get consumer group lag: %w", err)
		}
		lags = append(lags, lag)
	}

	return controller.NewRequeueAfter(10 * time.Minute)
}

func (r *Reconciler) securityOption(ctx context.Context, cg *kafkainternals.ConsumerGroup) (kafka.ConfigOption, error) {
	if cg.Spec.Template.Spec.Auth == nil {
		return kafka.NoOpConfigOption, nil
	}

	if cg.Spec.Template.Spec.Auth.NetSpec != nil {
		authContext, err := security.ResolveAuthContextFromNetSpec(r.SecretLister, cg.GetNamespace(), *cg.Spec.Template.Spec.Auth.NetSpec)
		if err != nil {
			return nil, fmt.Errorf("failed to create auth context: %w", err)
		}
		secret, err := security.Secret(ctx, &consumer.SecretLocator{ConsumerSpec: cg.Spec.Template.Spec}, security.NetSpecSecretProviderFunc(authContext))
		if err != nil {
			return nil, fmt.Errorf("failed to get secret: %w", err)
		}
		return security.NewSaramaSecurityOptionFromSecret(secret), nil
	}

	if cg.Spec.Template.Spec.Auth.AuthSpec != nil &&
		cg.Spec.Template.Spec.Auth.AuthSpec.Secret != nil &&
		cg.Spec.Template.Spec.Auth.AuthSpec.Secret.Ref != nil {

		sl := &security.MTConfigMapSecretLocator{ConfigMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: cg.GetNamespace()},
			Data:       map[string]string{security.AuthSecretNameKey: cg.Spec.Template.Spec.Auth.AuthSpec.Secret.Ref.Name},
		}}
		secret, err := security.Secret(ctx, sl, security.DefaultSecretProviderFunc(r.SecretLister, r.KubeClient))
		if err != nil {
			return nil, fmt.Errorf("failed to get secret: %w", err)
		}
		return security.NewSaramaSecurityOptionFromSecret(secret), nil
	}

	return kafka.NoOpConfigOption, nil
}

func (r *Reconciler) autoOffsetReset(cg *kafkainternals.ConsumerGroup) int64 {
	autoOffsetReset, _ := cg.Spec.Template.Spec.Configs.Configs["auto.offset.reset"]
	if autoOffsetReset == "earliest" {
		return sarama.OffsetNewest
	}
	return sarama.OffsetNewest
}

type Measurements struct {
	measurements map[string]Measurement
	m            sync.Mutex
}

func (m *Measurements) Get(key string) Measurement {
	m.m.Lock()
	defer m.m.Unlock()

	if m, ok := m.measurements[key]; ok {
		return m
	}
	return Measurement{LagSum: 0, LagCount: 0}
}

type Measurement struct {
	LagSum   uint64
	LagCount uint64
}

func (m *Measurement) Add(m *Measurement) Measurement {}

func (m *Measurement) ComputeDesired(cg *kafkainternals.ConsumerGroup, lags []kafka.ConsumerGroupLag) int32 {
	for _, lag := range lags {
		m.LagSum += lag.Total()
		m.LagCount++
	}
	curr := m.avg()

	if curr / *cg.Spec.LagThreshold > 0 {
		return *cg.Spec.Replicas + int32(curr / *cg.Spec.LagThreshold)
	}
	return *cg.Spec.Replicas - 1
}

func (m *Measurement) avg() uint64 {
	return m.LagSum / m.LagCount
}
