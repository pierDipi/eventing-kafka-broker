/*
 * Copyright 2020 The Knative Authors
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

package broker

import (
	"context"
	"fmt"
	"sync"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/contract"

	"github.com/Shopify/sarama"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	eventing "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/config"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/log"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/receiver"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/base"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/kafka"
)

const (
	// TopicPrefix is the Kafka Broker topic prefix - (topic name: knative-broker-<broker-namespace>-<broker-name>).
	TopicPrefix = "knative-broker-"

	InternalTopicName = "knative-internal-broker"
)

type Configs struct {
	config.Env

	BootstrapServers string
}

type Reconciler struct {
	*base.Reconciler

	Resolver *resolver.URIResolver

	KafkaDefaultTopicDetails     sarama.TopicDetail
	KafkaDefaultTopicDetailsLock sync.RWMutex

	// TODO change source CM to config-kafka
	// TODO expand liveness probe to check connectivity with Kafka
	bootstrapServers     []string
	bootstrapServersLock sync.RWMutex

	ConfigMapLister corelisters.ConfigMapLister

	// ClusterAdmin creates new sarama ClusterAdmin. It's convenient to add this as Reconciler field so that we can
	// mock the function used during the reconciliation loop.
	ClusterAdmin kafka.NewClusterAdminFunc
	// Producer creates new sarama sarama.SyncProducer. It's convenient to add this as Reconciler field so that we can
	// mock the function used during the reconciliation loop.
	Producer kafka.NewProducerFunc

	Configs *Configs
}

func (r *Reconciler) ReconcileKind(ctx context.Context, broker *eventing.Broker) reconciler.Event {

	logger := log.Logger(ctx, "reconcile", broker)

	statusConditionManager := base.StatusConditionManager{
		Object:     broker,
		SetAddress: broker.Status.SetAddress,
		Configs:    &r.Configs.Env,
		Recorder:   controller.GetEventRecorder(ctx),
	}

	if !r.IsReceiverRunning() || !r.IsDispatcherRunning() {
		return statusConditionManager.DataPlaneNotAvailable()
	}
	statusConditionManager.DataPlaneAvailable()

	topicConfig, err := r.topicConfig(logger, broker)
	if err != nil {
		return statusConditionManager.FailedToResolveConfig(err)
	}
	statusConditionManager.ConfigResolved()

	logger.Debug("config resolved", zap.Any("config", topicConfig))

	r.bootstrapServersLock.RLock()
	managementCluster := r.bootstrapServers
	r.bootstrapServersLock.RUnlock()

	// Create a log compacted topic. (see proto/def/contract.proto)
	tc := &kafka.TopicConfig{
		TopicDetail: sarama.TopicDetail{
			NumPartitions:     r.Configs.InternalTopicNumPartitions,
			ReplicationFactor: r.Configs.InternalTopicReplicationFactor,
		},
		BootstrapServers: managementCluster,
	}
	internalTopic, err := r.ClusterAdmin.CreateLogCompactedTopic(logger, InternalTopicName, tc)
	if err != nil {
		return statusConditionManager.FailedToCreateInternalTopic(internalTopic, err)
	}
	statusConditionManager.InternalTopicCreated(internalTopic)

	logger.Debug("Internal topic created", zap.Any("topic", internalTopic))

	topic, err := r.ClusterAdmin.CreateTopic(logger, kafka.Topic(TopicPrefix, broker), topicConfig)
	if err != nil {
		return statusConditionManager.FailedToCreateTopic(topic, err)
	}
	statusConditionManager.TopicReady(topic)

	logger.Debug("Topic created", zap.Any("topic", topic))

	// Get resource configuration.

	ct := &contract.Object{
		Object: &contract.Object_Ingress{
			Ingress: &contract.Ingress{
				Uid:   string(broker.UID),
				Topic: topic,
				IngressType: &contract.Ingress_Path{
					Path: receiver.PathFromObject(broker),
				},
				ContentMode:      contract.ContentMode_BINARY,
				BootstrapServers: topicConfig.GetBootstrapServers(),
			},
		},
	}

	if err := r.Producer.Add(managementCluster, internalTopic, string(broker.UID), ct); err != nil {
		return statusConditionManager.FailedToSendMessage(err)
	}

	logger.Debug("Resource send", zap.Any("resource", ct))

	return statusConditionManager.Reconciled()
}

func (r *Reconciler) FinalizeKind(ctx context.Context, broker *eventing.Broker) reconciler.Event {

	logger := log.Logger(ctx, "finalize", broker)

	r.bootstrapServersLock.RLock()
	managementCluster := r.bootstrapServers
	r.bootstrapServersLock.RUnlock()

	if err := r.Producer.Remove(managementCluster, InternalTopicName, string(broker.UID)); err != nil {
		return fmt.Errorf("failed to propagate broker deletion: %w", err)
	}

	logger.Debug("Resource deletion propagated")

	topicConfig, err := r.topicConfig(logger, broker)
	if err != nil {
		return fmt.Errorf("failed to resolve broker config: %w", err)
	}

	bootstrapServers := topicConfig.BootstrapServers
	topic, err := r.ClusterAdmin.DeleteTopic(kafka.Topic(TopicPrefix, broker), bootstrapServers)
	if err != nil {
		return err
	}

	logger.Debug("Topic deleted", zap.String("topic", topic))

	return nil
}

func (r *Reconciler) topicConfig(logger *zap.Logger, broker *eventing.Broker) (*kafka.TopicConfig, error) {

	if broker.Spec.Config == nil {
		return r.defaultConfig()
	}

	cm, err := ConfigMap(r.ConfigMapLister, logger, broker)
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	brokerConfig, err := configFromConfigMap(logger, cm)
	if err != nil {
		return nil, err
	}

	return brokerConfig, nil
}

func (r *Reconciler) defaultTopicDetail() sarama.TopicDetail {
	r.KafkaDefaultTopicDetailsLock.RLock()
	defer r.KafkaDefaultTopicDetailsLock.RUnlock()

	// copy the default topic details
	topicDetail := r.KafkaDefaultTopicDetails
	return topicDetail
}

func (r *Reconciler) defaultConfig() (*kafka.TopicConfig, error) {
	bootstrapServers, err := r.getDefaultBootstrapServersOrFail()
	if err != nil {
		return nil, err
	}

	return &kafka.TopicConfig{
		TopicDetail:      r.defaultTopicDetail(),
		BootstrapServers: bootstrapServers,
	}, nil
}

func (r *Reconciler) ConfigMapUpdated(ctx context.Context) func(configMap *corev1.ConfigMap) {

	logger := logging.FromContext(ctx).Desugar()

	return func(configMap *corev1.ConfigMap) {

		topicConfig, err := configFromConfigMap(logger, configMap)
		if err != nil {
			return
		}

		logger.Debug("new defaults",
			zap.Any("topicDetail", topicConfig.TopicDetail),
			zap.String("BootstrapServers", topicConfig.GetBootstrapServers()),
		)

		r.SetDefaultTopicDetails(topicConfig.TopicDetail)
		r.SetBootstrapServers(topicConfig.GetBootstrapServers())
	}
}

// SetBootstrapServers change kafka bootstrap brokers addresses.
// servers: a comma separated list of brokers to connect to.
func (r *Reconciler) SetBootstrapServers(servers string) {
	if servers == "" {
		return
	}

	addrs := kafka.BootstrapServersArray(servers)

	r.bootstrapServersLock.Lock()
	r.bootstrapServers = addrs
	r.bootstrapServersLock.Unlock()
}

func (r *Reconciler) SetDefaultTopicDetails(topicDetail sarama.TopicDetail) {
	r.KafkaDefaultTopicDetailsLock.Lock()
	defer r.KafkaDefaultTopicDetailsLock.Unlock()

	r.KafkaDefaultTopicDetails = topicDetail
}

func (r *Reconciler) getDefaultBootstrapServersOrFail() ([]string, error) {
	r.bootstrapServersLock.RLock()
	defer r.bootstrapServersLock.RUnlock()

	if len(r.bootstrapServers) == 0 {
		return nil, fmt.Errorf("no %s provided", BootstrapServersConfigMapKey)
	}

	return r.bootstrapServers, nil
}
