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

package channel

import (
	"context"
	"fmt"

	"github.com/Shopify/sarama"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	messaging "knative.dev/eventing-kafka/pkg/apis/messaging/v1beta1"
	"knative.dev/eventing-kafka/pkg/channel/consolidated/utils"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/config"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/contract"
	coreconfig "knative.dev/eventing-kafka-broker/control-plane/pkg/core/config"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/receiver"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/base"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/kafka"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/resource"
)

const (
	// TopicPrefix is the Kafka Channel topic prefix - (topic name: knative-broker-<broker-namespace>-<broker-name>).
	TopicPrefix = "knative-channel-"
)

type Reconciler struct {
	*resource.Reconciler

	Resolver *resolver.URIResolver

	ConfigMapLister corelisters.ConfigMapLister

	Configs *config.Env
}

func (r *Reconciler) ReconcileKind(ctx context.Context, kc *messaging.KafkaChannel) reconciler.Event {
	return r.Reconcile(ctx, kc, kc.Status.SetAddress)
}

func (r *Reconciler) FinalizeKind(ctx context.Context, kc *messaging.KafkaChannel) reconciler.Event {
	return r.Finalize(ctx, kc)
}

func (r *Reconciler) topicConfig(_ *zap.Logger, obj base.Object) (*kafka.TopicConfig, error) {
	kc := obj.(*messaging.KafkaChannel)

	kafkaConfig, err := r.getKafkaConfig(obj)
	if err != nil {
		return nil, err
	}

	return &kafka.TopicConfig{
		TopicDetail: sarama.TopicDetail{
			NumPartitions:     kc.Spec.NumPartitions,
			ReplicationFactor: kc.Spec.ReplicationFactor,
		},
		BootstrapServers: kafkaConfig.Brokers,
	}, nil
}

func (r *Reconciler) getKafkaConfig(obj base.Object) (*utils.KafkaConfig, error) {
	cm, err := r.getKafkaConfigMap(obj)
	if err != nil {
		return nil, err
	}

	kafkaConfig, err := utils.GetKafkaConfig(cm.Data)
	if err != nil {
		return nil, fmt.Errorf("faile to extract configuration from ConfigMap %s/%s: %w", cm.Namespace, cm.Name, err)
	}

	return kafkaConfig, nil
}

func (r *Reconciler) getKafkaConfigMap(obj base.Object) (*corev1.ConfigMap, error) {
	if scope, ok := obj.GetAnnotations()[eventing.ScopeAnnotationKey]; ok && scope == eventing.ScopeNamespace {
		cm, err := r.ConfigMapLister.ConfigMaps(obj.GetNamespace()).Get(cmConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to get ConfigMap %s in %s namespace: %w", cmConfig, obj.GetNamespace(), err)
		}
		return cm, nil
	}

	var err error
	cm, err := r.ConfigMapLister.ConfigMaps(r.Configs.SystemNamespace).Get(cmConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap %s in %s namespace: %w", cmConfig, r.Configs.SystemNamespace, err)
	}
	return cm, nil
}

func (r *Reconciler) getChannelResource(ctx context.Context, topic string, obj base.Object, topicConfig *kafka.TopicConfig) (*contract.Resource, error) {
	kc := obj.(*messaging.KafkaChannel)

	egresses := make([]*contract.Egress, 0, len(kc.Spec.Subscribers))
	for _, s := range kc.Spec.Subscribers {
		replyStrategy := &contract.Egress_ReplyUrl{}

		if s.ReplyURI != nil {
			replyStrategy.ReplyUrl = s.ReplyURI.String()
		}

		egressConfig, err := coreconfig.EgressConfigFromDelivery(
			ctx,
			r.Resolver,
			kc.Spec.Delivery,
			r.Configs.DefaultBackoffDelayMs,
			fmt.Sprintf("KafkaChannel.Spec.Subcribers[%s].Delivery", s.UID),
			kc,
		)
		if err != nil {
			return nil, err // Error already decorated with more information
		}

		if s.SubscriberURI == nil {
			return nil, fmt.Errorf("KafkaChannel.Spec.Subscribers[%s].SubscriberURI is nil", s.UID)
		}

		egress := &contract.Egress{
			ConsumerGroup: string(s.UID),
			Destination:   s.SubscriberURI.String(),
			ReplyStrategy: replyStrategy,
			Uid:           string(s.UID),
			EgressConfig:  egressConfig,
		}

		egresses = append(egresses, egress)
	}

	egressConfig, err := coreconfig.EgressConfigFromDelivery(
		ctx,
		r.Resolver,
		kc.Spec.Delivery,
		r.Configs.DefaultBackoffDelayMs,
		"KafkaChannel.Spec.Delivery",
		kc,
	)
	if err != nil {
		return nil, err // Error already decorated with more information
	}

	return &contract.Resource{
		Uid:              string(kc.GetUID()),
		Topics:           []string{topic},
		BootstrapServers: topicConfig.GetBootstrapServers(),
		Ingress: &contract.Ingress{
			ContentMode: contract.ContentMode_BINARY,
			IngressType: &contract.Ingress_Path{
				Path: receiver.PathFromObject(kc),
			},
		},
		EgressConfig: egressConfig,
		Egresses:     egresses,
	}, nil
}
