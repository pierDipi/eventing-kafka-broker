/*
 * Copyright 2021 The Knative Authors
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

package testing

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	kafkainternals "knative.dev/eventing-kafka-broker/control-plane/pkg/apis/internals/kafka/eventing/v1alpha1"
)

const (
	ConsumerGroupName      = "test-cg"
	ConsumerGroupNamespace = "test-cg-ns"

	ConsumerGroupTestKey = ConsumerGroupNamespace + "/" + ConsumerGroupName
)

var (
	ConsumerLabels = map[string]string{"c": "C"}

	ConsumerSourceLabel = map[string]string{
		kafkainternals.ConsumerLabelSelector: SourceUUID,
	}

	ConsumerTriggerLabel = map[string]string{
		kafkainternals.ConsumerLabelSelector: TriggerUUID,
	}

	ConsumerSubscription1Label = map[string]string{
		kafkainternals.ConsumerLabelSelector: Subscription1UUID,
	}

	ConsumerSubscription2Label = map[string]string{
		kafkainternals.ConsumerLabelSelector: Subscription2UUID,
	}

	OwnerAsTriggerLabel = map[string]string{
		kafkainternals.UserFacingResourceLabelSelector: "trigger",
	}

	OwnerAsSourceLabel = map[string]string{
		kafkainternals.UserFacingResourceLabelSelector: "kafkasource",
	}

	OwnerAsChannelLabel = map[string]string{
		kafkainternals.UserFacingResourceLabelSelector: "kafkachannel",
		kafkainternals.KafkaChannelNameLabel:           ChannelName,
	}
)

type ConsumerGroupOption func(cg *kafkainternals.ConsumerGroup)

func NewConsumerGroup(opts ...ConsumerGroupOption) *kafkainternals.ConsumerGroup {

	cg := &kafkainternals.ConsumerGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConsumerGroupName,
			Namespace: ConsumerGroupNamespace,
		},
		Spec: kafkainternals.ConsumerGroupSpec{
			Template: kafkainternals.ConsumerTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: ConsumerLabels},
				Spec: kafkainternals.ConsumerSpec{
					Subscriber: duckv1.Destination{
						Ref: &duckv1.KReference{
							Kind:       "Service",
							Namespace:  ServiceNamespace,
							Name:       ServiceName,
							APIVersion: "v1",
						},
					},
				},
			},
		},
	}

	for _, opt := range opts {
		opt(cg)
	}

	return cg
}

func ConsumerGroupSubscriber(d duckv1.Destination) ConsumerGroupOption {
	return func(cg *kafkainternals.ConsumerGroup) {
		cg.Spec.Template.Spec.Subscriber = d
	}
}

func ConsumerGroupReady(cg *kafkainternals.ConsumerGroup) {
	cg.InitializeConditions()
	cg.MarkReconcileConsumersSucceeded()
	cg.MarkScheduleSucceeded()
	cg.Status.SubscriberURI = ConsumerSubscriberURI
	if cg.HasDeadLetterSink() {
		cg.Status.DeadLetterSinkURI = ConsumerDeadLetterSinkURI
	}
}

func WithConsumerGroupFailed(reason string, msg string) ConsumerGroupOption {
	return func(cg *kafkainternals.ConsumerGroup) {
		cg.GetConditionSet().Manage(cg.GetStatus()).MarkFalse(kafkainternals.ConditionConsumerGroupConsumers, reason, msg)
	}
}

func WithConsumerGroupName(name string) ConsumerGroupOption {
	return func(cg *kafkainternals.ConsumerGroup) {
		cg.ObjectMeta.Name = name
	}
}

func WithConsumerGroupNamespace(namespace string) ConsumerGroupOption {
	return func(cg *kafkainternals.ConsumerGroup) {
		cg.ObjectMeta.Namespace = namespace
	}
}

func WithConsumerGroupOwnerRef(ownerref *metav1.OwnerReference) ConsumerGroupOption {
	return func(cg *kafkainternals.ConsumerGroup) {
		cg.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerref}
	}
}

func WithConsumerGroupMetaLabels(labels map[string]string) ConsumerGroupOption {
	return func(cg *kafkainternals.ConsumerGroup) {
		cg.Labels = labels
	}
}

func WithConsumerGroupLabels(labels map[string]string) ConsumerGroupOption {
	return func(cg *kafkainternals.ConsumerGroup) {
		cg.Spec.Template.Labels = labels
	}
}

func ConsumerGroupReplicas(replicas int32) ConsumerGroupOption {
	return func(cg *kafkainternals.ConsumerGroup) {
		cg.Spec.Replicas = pointer.Int32Ptr(replicas)
	}
}

func ConsumerGroupStatusReplicas(replicas int32) ConsumerGroupOption {
	return func(cg *kafkainternals.ConsumerGroup) {
		cg.Status.Replicas = pointer.Int32Ptr(replicas)
	}
}

func ConsumerGroupReplicasStatus(replicas int32) ConsumerGroupOption {
	return func(cg *kafkainternals.ConsumerGroup) {
		cg.Status.Replicas = pointer.Int32Ptr(replicas)
	}
}

func ConsumerGroupSelector(selector map[string]string) ConsumerGroupOption {
	return func(cg *kafkainternals.ConsumerGroup) {
		cg.Spec.Selector = selector
	}
}

func ConsumerGroupConsumerSpec(spec kafkainternals.ConsumerSpec) ConsumerGroupOption {
	return func(cg *kafkainternals.ConsumerGroup) {
		cg.Spec.Template.Spec = spec
	}
}

func WithDeadLetterSinkURI(uri string) func(cg *kafkainternals.ConsumerGroup) {
	return func(cg *kafkainternals.ConsumerGroup) {
		u, err := apis.ParseURL(uri)
		if err != nil {
			panic(err)
		}
		cg.Status.DeadLetterSinkURI = u
	}
}

func ConsumerGroupOwnerRef(reference metav1.OwnerReference) ConsumerGroupOption {
	return func(cg *kafkainternals.ConsumerGroup) {
		cg.OwnerReferences = append(cg.OwnerReferences, reference)
	}
}
