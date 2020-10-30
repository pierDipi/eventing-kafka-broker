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

// receiver_condition_set.go contains Broker and Kafka Sink logic for status conditions handling.
package base

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"knative.dev/eventing/pkg/reconciler/names"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/reconciler"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/config"
)

const (
	ConditionAddressable             apis.ConditionType = "Addressable"
	ConditionDataPlaneAvailable      apis.ConditionType = "DataPlaneAvailable"
	ConditionTopicReady              apis.ConditionType = "TopicReady"
	ConditionConfigParsed            apis.ConditionType = "ConfigParsed"
	ConditionInternalTopicCreated    apis.ConditionType = "InternalTopicReady"
	ConditionConfigChangedPropagated apis.ConditionType = "ConfigPropagated"
)

var ConditionSet = apis.NewLivingConditionSet(
	ConditionAddressable,
	ConditionDataPlaneAvailable,
	ConditionTopicReady,
	ConditionConfigParsed,
	ConditionInternalTopicCreated,
	ConditionConfigChangedPropagated,
)

const (
	TopicOwnerAnnotation = "eventing.knative.dev/topic.owner"

	ReasonDataPlaneNotAvailable  = "Data plane not available"
	MessageDataPlaneNotAvailable = "Did you install the data plane for this component?"

	ReasonTopicNotPresent = "Topic is not present"
)

type Object interface {
	duckv1.KRShaped
	runtime.Object
}

type StatusConditionManager struct {
	Object Object

	SetAddress func(u *apis.URL)

	Configs *config.Env

	Recorder record.EventRecorder
}

func (manager *StatusConditionManager) DataPlaneAvailable() {
	manager.Object.GetConditionSet().Manage(manager.Object.GetStatus()).MarkTrue(ConditionDataPlaneAvailable)
}

func (manager *StatusConditionManager) DataPlaneNotAvailable() reconciler.Event {

	manager.Object.GetConditionSet().Manage(manager.Object.GetStatus()).MarkFalse(
		ConditionDataPlaneAvailable,
		ReasonDataPlaneNotAvailable,
		MessageDataPlaneNotAvailable,
	)

	return fmt.Errorf("%s: %s", ReasonDataPlaneNotAvailable, MessageDataPlaneNotAvailable)
}

func (manager *StatusConditionManager) FailedToCreateTopic(topic string, err error) reconciler.Event {

	manager.Object.GetConditionSet().Manage(manager.Object.GetStatus()).MarkFalse(
		ConditionTopicReady,
		fmt.Sprintf("Failed to create topic: %s", topic),
		"%v",
		err,
	)

	return fmt.Errorf("failed to create topic: %s: %w", topic, err)
}

func (manager *StatusConditionManager) TopicReady(topic string) {

	if owner, ok := manager.Object.GetStatus().Annotations[TopicOwnerAnnotation]; ok {
		manager.Object.GetConditionSet().Manage(manager.Object.GetStatus()).MarkTrueWithReason(
			ConditionTopicReady,
			fmt.Sprintf("Topic %s (owner %s)", topic, owner),
			"",
		)

		return
	}

	manager.Object.GetConditionSet().Manage(manager.Object.GetStatus()).MarkTrueWithReason(
		ConditionTopicReady,
		fmt.Sprintf("Topic %s created", topic),
		"",
	)
}

func (manager *StatusConditionManager) Reconciled() reconciler.Event {

	object := manager.Object

	manager.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   names.ServiceHostName(manager.Configs.IngressName, manager.Configs.SystemNamespace),
		Path:   fmt.Sprintf("/%s/%s", object.GetNamespace(), object.GetName()),
	})
	object.GetConditionSet().Manage(object.GetStatus()).MarkTrue(ConditionAddressable)

	return nil
}

func (manager *StatusConditionManager) FailedToResolveConfig(err error) reconciler.Event {

	manager.Object.GetConditionSet().Manage(manager.Object.GetStatus()).MarkFalse(
		ConditionConfigParsed,
		fmt.Sprintf("%v", err),
		"",
	)

	return fmt.Errorf("failed to get contract configuration: %w", err)
}

func (manager *StatusConditionManager) ConfigResolved() {
	manager.Object.GetConditionSet().Manage(manager.Object.GetStatus()).MarkTrue(ConditionConfigParsed)
}

func (manager *StatusConditionManager) TopicNotPresentOrInvalidErr(err error) error {
	manager.Object.GetConditionSet().Manage(manager.Object.GetStatus()).MarkFalse(
		ConditionTopicReady,
		ReasonTopicNotPresent,
		err.Error(),
	)

	return fmt.Errorf("topic is not present: %w", err)
}

func (manager *StatusConditionManager) TopicNotPresentOrInvalid() error {

	manager.Object.GetConditionSet().Manage(manager.Object.GetStatus()).MarkFalse(
		ConditionTopicReady,
		ReasonTopicNotPresent,
		"Check topic configuration",
	)

	return fmt.Errorf("topic is not present: check topic configuration")

}

func (manager *StatusConditionManager) FailedToCreateInternalTopic(topic string, err error) reconciler.Event {

	manager.Object.GetConditionSet().Manage(manager.Object.GetStatus()).MarkFalse(
		ConditionInternalTopicCreated,
		fmt.Sprintf("Failed to create internal topic %s", topic),
		err.Error(),
	)

	return fmt.Errorf("failed to create internal topic %s: %w", topic, err)
}

func (manager *StatusConditionManager) InternalTopicCreated(topic string) {
	manager.Object.GetConditionSet().Manage(manager.Object.GetStatus()).MarkTrueWithReason(
		ConditionInternalTopicCreated,
		fmt.Sprintf("Topic %s created", topic),
		"",
	)
}

func (manager *StatusConditionManager) FailedToSendMessage(err error) reconciler.Event {

	manager.Object.GetConditionSet().Manage(manager.Object.GetStatus()).MarkFalse(
		ConditionInternalTopicCreated,
		"Failed to propagate changes to data plane",
		err.Error(),
	)

	return fmt.Errorf("failed to propagate changes to data plane: %w", err)
}
