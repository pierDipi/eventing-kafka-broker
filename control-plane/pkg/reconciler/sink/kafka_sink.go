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

package sink

import (
	"context"
	"fmt"
	"sync"

	"github.com/Shopify/sarama"
	"go.uber.org/zap"
	corelisters "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/reconciler"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/contract"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/receiver"

	eventing "knative.dev/eventing-kafka-broker/control-plane/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/config"
	coreconfig "knative.dev/eventing-kafka-broker/control-plane/pkg/core/config"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/log"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/base"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/kafka"
)

const (
	ExternalTopicOwner   = "external"
	ControllerTopicOwner = "kafkasink-controller"

	InternalTopicName = "knative-internal-sinks"
)

type Reconciler struct {
	*base.Reconciler

	ConfigMapLister corelisters.ConfigMapLister

	// TODO change source CM to config-kafka
	// TODO expand readiness check to check connectivity with Kafka
	bootstrapServers     []string
	bootstrapServersLock sync.RWMutex

	// ClusterAdmin creates new sarama ClusterAdmin. It's convenient to add this as Reconciler field so that we can
	// mock the function used during the reconciliation loop.
	ClusterAdmin kafka.NewClusterAdminFunc
	// Producer creates new sarama sarama.SyncProducer. It's convenient to add this as Reconciler field so that we can
	// mock the function used during the reconciliation loop.
	Producer kafka.NewProducerFunc

	Configs *config.Env
}

func (r *Reconciler) ReconcileKind(ctx context.Context, ks *eventing.KafkaSink) reconciler.Event {

	logger := log.Logger(ctx, "reconcile", ks)

	statusConditionManager := base.StatusConditionManager{
		Object:     ks,
		SetAddress: ks.Status.SetAddress,
		Configs:    r.Configs,
		Recorder:   controller.GetEventRecorder(ctx),
	}

	if !r.IsReceiverRunning() {
		return statusConditionManager.DataPlaneNotAvailable()
	}
	statusConditionManager.DataPlaneAvailable()

	r.bootstrapServersLock.RLock()
	managementCluster := r.bootstrapServers
	r.bootstrapServersLock.RUnlock()

	// Create a log compacted topic. (see proto/def/contract.proto#Object)
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

	if ks.GetStatus().Annotations == nil {
		ks.GetStatus().Annotations = make(map[string]string, 1)
	}

	if ks.Spec.NumPartitions != nil && ks.Spec.ReplicationFactor != nil {

		ks.GetStatus().Annotations[base.TopicOwnerAnnotation] = ControllerTopicOwner

		topicConfig := topicConfigFromSinkSpec(&ks.Spec)

		topic, err := r.ClusterAdmin.CreateTopic(logger, ks.Spec.Topic, topicConfig)
		if err != nil {
			return statusConditionManager.FailedToCreateTopic(topic, err)
		}
	} else {

		// If the topic is externally managed, we need to make sure that the topic exists and it's valid.

		ks.GetStatus().Annotations[base.TopicOwnerAnnotation] = ExternalTopicOwner

		isPresentAndValid, err := r.ClusterAdmin.IsTopicPresentAndValid(ks.Spec.Topic, ks.Spec.BootstrapServers)
		if err != nil {
			return statusConditionManager.TopicNotPresentOrInvalidErr(err)
		}
		if !isPresentAndValid {
			// The topic might be invalid.
			return statusConditionManager.TopicNotPresentOrInvalid()
		}
	}
	statusConditionManager.TopicReady(ks.Spec.Topic)

	logger.Debug("Topic created", zap.Any("topic", ks.Spec.Topic))

	// Get sink configuration.
	ct := &contract.Object{
		Object: &contract.Object_Ingress{
			Ingress: &contract.Ingress{
				Uid:              string(ks.UID),
				BootstrapServers: kafka.BootstrapServersCommaSeparated(ks.Spec.BootstrapServers),
				ContentMode:      coreconfig.ContentModeFromString(*ks.Spec.ContentMode),
				IngressType:      &contract.Ingress_Path{Path: receiver.PathFromObject(ks)},
				Topic:            ks.Spec.Topic,
			},
		},
	}
	statusConditionManager.ConfigResolved()

	if err := r.Producer.Add(managementCluster, internalTopic, string(ks.UID), ct); err != nil {
		return statusConditionManager.FailedToSendMessage(err)
	}

	logger.Debug("Resource send", zap.Any("resource", ct))

	return statusConditionManager.Reconciled()
}

func (r *Reconciler) FinalizeKind(ctx context.Context, ks *eventing.KafkaSink) reconciler.Event {

	logger := log.Logger(ctx, "finalize", ks)

	// Get contract config map.
	contractConfigMap, err := r.GetOrCreateDataPlaneConfigMap(ctx)
	if err != nil {
		return fmt.Errorf("failed to get contract config map %s: %w", r.Configs.DataPlaneConfigMapAsString(), err)
	}

	logger.Debug("Got contract config map")

	// Get contract data.
	ct, err := r.GetDataPlaneConfigMapData(logger, contractConfigMap)
	if err != nil {
		return fmt.Errorf("failed to get contract: %w", err)
	}

	logger.Debug(
		"Got contract data from config map",
		zap.Any("contract", (*log.ContractMarshaller)(ct)),
	)

	sinkIndex := coreconfig.FindResource(ct, ks.UID)
	if sinkIndex != coreconfig.NoResource {
		coreconfig.DeleteResource(ct, sinkIndex)

		logger.Debug("Sink deleted", zap.Int("index", sinkIndex))

		// Update the configuration map with the new contract data.
		if err := r.UpdateDataPlaneConfigMap(ctx, ct, contractConfigMap); err != nil {
			return err
		}

		logger.Debug("Sinks config map updated")
	}

	if ks.GetStatus().Annotations[base.TopicOwnerAnnotation] == ControllerTopicOwner {
		topic, err := r.ClusterAdmin.DeleteTopic(ks.Spec.Topic, ks.Spec.BootstrapServers)
		if err != nil {
			return err
		}
		logger.Debug("Topic deleted", zap.String("topic", topic))
	}

	return nil
}

func topicConfigFromSinkSpec(kss *eventing.KafkaSinkSpec) *kafka.TopicConfig {
	return &kafka.TopicConfig{
		TopicDetail: sarama.TopicDetail{
			NumPartitions:     *kss.NumPartitions,
			ReplicationFactor: *kss.ReplicationFactor,
		},
		BootstrapServers: kss.BootstrapServers,
	}
}
