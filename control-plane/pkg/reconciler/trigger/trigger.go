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

package trigger

import (
	"context"
	"fmt"
	"sync"

	"github.com/golang/protobuf/ptypes/empty"
	corelisters "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/configmap"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/contract"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/kafka"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventing "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingclientset "knative.dev/eventing/pkg/client/clientset/versioned"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/config"
	coreconfig "knative.dev/eventing-kafka-broker/control-plane/pkg/core/config"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/log"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/base"
	brokerreconciler "knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/broker"
)

type Reconciler struct {
	*base.Reconciler

	BrokerLister   eventinglisters.BrokerLister
	EventingClient eventingclientset.Interface
	Resolver       *resolver.URIResolver

	// TODO change source CM to config-kafka
	// TODO expand liveness probe to check connectivity with Kafka
	bootstrapServers     []string
	bootstrapServersLock sync.RWMutex

	ConfigMapLister corelisters.ConfigMapLister

	// Producer creates new sarama sarama.SyncProducer. It's convenient to add this as Reconciler field so that we can
	// mock the function used during the reconciliation loop.
	Producer kafka.NewProducerFunc

	Configs *config.Env
}

func (r *Reconciler) ReconcileKind(ctx context.Context, trigger *eventing.Trigger) reconciler.Event {

	logger := log.Logger(ctx, "reconcile", trigger)

	statusConditionManager := statusConditionManager{
		Trigger:  trigger,
		Configs:  r.Configs,
		Recorder: controller.GetEventRecorder(ctx),
	}

	broker, err := r.BrokerLister.Brokers(trigger.Namespace).Get(trigger.Spec.Broker)
	if err != nil && !apierrors.IsNotFound(err) {
		return statusConditionManager.failedToGetBroker(err)
	}

	if apierrors.IsNotFound(err) {

		// Actually check if the broker doesn't exist.
		// Note: do not introduce another `broker` variable with `:`
		broker, err = r.EventingClient.EventingV1().Brokers(trigger.Namespace).Get(ctx, trigger.Spec.Broker, metav1.GetOptions{})

		if apierrors.IsNotFound(err) {

			logger.Debug("broker not found", zap.String("finalizeDuringReconcile", "notFound"))
			// The associated broker doesn't exist anymore, so clean up Trigger resources.
			return r.FinalizeKind(ctx, trigger)
		}
	}

	// Ignore Triggers that are associated with a Broker we don't own.
	if isOur, brokerClass := isOurBroker(broker); !isOur {
		logger.Debug("Ignoring Trigger", zap.String(eventing.BrokerClassAnnotationKey, brokerClass))
		return nil
	}

	if !broker.GetDeletionTimestamp().IsZero() {

		logger.Debug("broker deleted", zap.String("finalizeDuringReconcile", "deleted"))

		// The associated broker doesn't exist anymore, so clean up Trigger resources.
		return r.FinalizeKind(ctx, trigger)
	}
	statusConditionManager.propagateBrokerCondition(broker)

	if !broker.Status.IsReady() {
		return nil
	}

	ct, err := r.getTriggerConfig(ctx, logger, broker, trigger)
	if err != nil {
		return statusConditionManager.failedToResolveTriggerConfig(err)
	}

	r.bootstrapServersLock.RLock()
	managementCluster := r.bootstrapServers
	r.bootstrapServersLock.RUnlock()

	if err := r.Producer.Add(managementCluster, brokerreconciler.InternalTopicName, string(trigger.UID), ct); err != nil {
		return statusConditionManager.FailedToSendMessage(err)
	}

	return statusConditionManager.reconciled()
}

func (r *Reconciler) FinalizeKind(ctx context.Context, trigger *eventing.Trigger) reconciler.Event {

	logger := log.Logger(ctx, "finalize", trigger)

	broker, err := r.BrokerLister.Brokers(trigger.Namespace).Get(trigger.Spec.Broker)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get broker from lister: %w", err)
	}

	r.bootstrapServersLock.RLock()
	managementCluster := r.bootstrapServers
	r.bootstrapServersLock.RUnlock()

	if err := r.Producer.Remove(managementCluster, brokerreconciler.InternalTopicName, string(broker.UID)); err != nil {
		return fmt.Errorf("failed to propagate trigger deletion: %w", err)
	}

	logger.Debug("Resource deletion propagated")

	return nil
}

func (r *Reconciler) getTriggerConfig(ctx context.Context, logger *zap.Logger, broker *eventing.Broker, trigger *eventing.Trigger) (*contract.Object, error) {
	destination, err := r.Resolver.URIFromDestinationV1(ctx, trigger.Spec.Subscriber, trigger)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Trigger.Spec.Subscriber: %w", err)
	}
	trigger.Status.SubscriberURI = destination

	var filter *contract.Filter
	if trigger.Spec.Filter != nil && trigger.Spec.Filter.Attributes != nil {
		filter = &contract.Filter{
			Attributes: trigger.Spec.Filter.Attributes,
		}
	}

	deadLetterSinkURL := ""
	retry := r.Configs.DefaultRetry
	backoffPolicy := contract.BackoffPolicy_Exponential
	backoffDelay := r.Configs.DefaultBackoffDelayMs

	if broker.Spec.Delivery != nil {
		if broker.Spec.Delivery.DeadLetterSink != nil {
			deadLetter, err := r.Resolver.URIFromDestinationV1(ctx, *broker.Spec.Delivery.DeadLetterSink, broker)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve Broker.Spec.Delivery.DeadLetterSink: %w", err)
			}
			deadLetterSinkURL = deadLetter.String()
		}

		if broker.Spec.Delivery.Retry != nil {
			retry = *broker.Spec.Delivery.Retry
		}

		if broker.Spec.Delivery.BackoffPolicy != nil {
			backoffPolicy = coreconfig.BackoffPolicyFromString(broker.Spec.Delivery.BackoffPolicy)
		}

		backoffDelay, err = coreconfig.BackoffDelayFromISO8601String(broker.Spec.Delivery.BackoffDelay, r.Configs.DefaultBackoffDelayMs)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve Broker.Spec.Delivery.BackoffDelay: %w", err)
		}
	}

	cm, err := brokerreconciler.ConfigMap(r.ConfigMapLister, logger, broker)
	if err != nil {
		return nil, fmt.Errorf("failed to get broker config: %w", err)
	}

	bootstrapServers := ""
	err = configmap.Parse(cm.Data,
		configmap.AsString(brokerreconciler.BootstrapServersConfigMapKey, &bootstrapServers),
	)
	if err != nil || bootstrapServers == "" {
		return nil, fmt.Errorf("failed to parse broker config (%s: %s): %w", brokerreconciler.BootstrapServersConfigMapKey, bootstrapServers, err)
	}

	egress := &contract.Egress{
		ConsumerGroup:    string(trigger.UID),
		Destination:      destination.String(),
		ReplyStrategy:    &contract.Egress_ReplyToOriginalTopic{ReplyToOriginalTopic: &empty.Empty{}},
		Filter:           filter,
		Uid:              string(trigger.UID),
		Topics:           []string{kafka.Topic(brokerreconciler.TopicPrefix, broker)},
		BootstrapServers: bootstrapServers,
		EgressConfig: &contract.EgressConfig{
			DeadLetter:    deadLetterSinkURL,
			Retry:         uint32(retry),
			BackoffPolicy: backoffPolicy,
			BackoffDelay:  backoffDelay,
		},
	}

	return &contract.Object{Object: &contract.Object_Egress{Egress: egress}}, nil
}

func isOurBroker(broker *eventing.Broker) (bool, string) {
	brokerClass := broker.GetAnnotations()[eventing.BrokerClassAnnotationKey]
	return brokerClass == kafka.BrokerClass, brokerClass
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

