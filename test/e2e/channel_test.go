// +build e2e

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

package e2e

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	confhelpers "knative.dev/eventing/test/conformance/helpers"
	"knative.dev/eventing/test/e2e/helpers"
	testlib "knative.dev/eventing/test/lib"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/kafka"
)

var channelTestRunner = testlib.ComponentsTestRunner{
	ComponentFeatureMap: map[metav1.TypeMeta][]testlib.Feature{
		{
			APIVersion: "messaging.knative.dev/v1beta1",
			Kind:       "KafkaChannel",
		}: {
			testlib.FeatureBasic,
			testlib.FeatureRedelivery,
			testlib.FeaturePersistence,
		},
		{
			APIVersion: "messaging.knative.dev/v1alpha1",
			Kind:       "KafkaChannel",
		}: {
			testlib.FeatureBasic,
			testlib.FeatureRedelivery,
			testlib.FeaturePersistence,
		},
	},
	ComponentsToTest: []metav1.TypeMeta{
		{
			APIVersion: "messaging.knative.dev/v1alpha1",
			Kind:       "KafkaChannel",
		},
		{
			APIVersion: "messaging.knative.dev/v1beta1",
			Kind:       "KafkaChannel",
		},
	},
}

// ------------------ Conformance -------------------------------------

func TestChannelAddressableResolverClusterRoleTest(t *testing.T) {
	confhelpers.TestChannelAddressableResolverClusterRoleTestRunner(t, channelTestRunner, testlib.SetupClientOptionNoop)
}

func TestChannelChannelableManipulatorClusterRoleTest(t *testing.T) {
	confhelpers.TestChannelChannelableManipulatorClusterRoleTestRunner(context.Background(), t, channelTestRunner, testlib.SetupClientOptionNoop)
}

func TestChannelCRDMetadata(t *testing.T) {
	confhelpers.ChannelCRDMetadataTestHelperWithChannelTestRunner(t, channelTestRunner, testlib.SetupClientOptionNoop)
}

func TestChannelCRDName(t *testing.T) {
	confhelpers.ChannelCRDNameTestHelperWithChannelTestRunner(t, channelTestRunner, testlib.SetupClientOptionNoop)
}

func TestChannelDataPlaneSuccess(t *testing.T) {
	confhelpers.ChannelDataPlaneSuccessTestRunner(context.Background(), t, channelTestRunner, testlib.SetupClientOptionNoop)
}

func TestChannelDataPlaneFailure(t *testing.T) {
	confhelpers.ChannelDataPlaneFailureTestRunner(context.Background(), t, channelTestRunner, testlib.SetupClientOptionNoop)
}

func TestChannelSpec(t *testing.T) {
	confhelpers.ChannelSpecTestHelperWithChannelTestRunner(t, channelTestRunner, testlib.SetupClientOptionNoop)
}

func TestChannelStatusSubscriber(t *testing.T) {
	confhelpers.ChannelStatusSubscriberTestHelperWithChannelTestRunner(context.Background(), t, channelTestRunner, testlib.SetupClientOptionNoop)
}

func TestChannelStatus(t *testing.T) {
	confhelpers.ChannelStatusTestHelperWithChannelTestRunner(t, channelTestRunner, testlib.SetupClientOptionNoop)
}

func TestChannelTracingWithReply(t *testing.T) {
	t.Skip("unsupported https://github.com/knative-sandbox/eventing-kafka-broker/issues/115")
	confhelpers.ChannelTracingTestHelperWithChannelTestRunner(context.Background(), t, channelTestRunner, testlib.SetupClientOptionNoop)
}

// ------------------ End Conformance -------------------------------------

func TestChannelChainV1beta1(t *testing.T) {
	helpers.ChannelChainTestHelper(context.Background(), t, helpers.SubscriptionV1beta1, channelTestRunner)
}

func TestChannelChainV1(t *testing.T) {
	helpers.ChannelChainTestHelper(context.Background(), t, helpers.SubscriptionV1, channelTestRunner)
}

// TestChannelClusterDefaulter tests a cluster defaulted channel can be created with the template specified through configmap.
func TestChannelClusterDefaulter(t *testing.T) {
	helpers.ChannelClusterDefaulterTestHelper(context.Background(), t, channelTestRunner)
}

// TestChannelNamespaceDefaulter tests a namespace defaulted channel can be created with the template specified through configmap.
func TestChannelNamespaceDefaulter(t *testing.T) {
	helpers.ChannelNamespaceDefaulterTestHelper(context.Background(), t, channelTestRunner)
}

// TestChannelDeadLetterSink tests DeadLetterSink
func TestChannelDeadLetterSinkV1Beta1(t *testing.T) {
	helpers.ChannelDeadLetterSinkTestHelper(context.Background(), t, helpers.SubscriptionV1beta1, channelTestRunner)
}

func TestChannelDeadLetterSinkV1(t *testing.T) {
	helpers.ChannelDeadLetterSinkTestHelper(context.Background(), t, helpers.SubscriptionV1, channelTestRunner)
}

/*
TestEventTransformationForSubscription tests the following scenario:
             1            2                 5            6                  7
EventSource ---> Channel ---> Subscription ---> Channel ---> Subscription ----> Service(Logger)
                                   |  ^
                                 3 |  | 4
                                   |  |
                                   |  ---------
                                   -----------> Service(Transformation)
*/
func TestEventTransformationForSubscriptionV1Beta1(t *testing.T) {
	helpers.EventTransformationForSubscriptionTestHelper(context.Background(), t, helpers.SubscriptionV1beta1, channelTestRunner)
}

func TestEventTransformationForSubscriptionV1(t *testing.T) {
	helpers.EventTransformationForSubscriptionTestHelper(context.Background(), t, helpers.SubscriptionV1, channelTestRunner)
}

/*
SingleEventForChannelTestHelper tests the following scenario:
EventSource ---> Channel ---> Subscription ---> Service(Logger)
*/

func TestSingleBinaryEventForChannelV1Beta1(t *testing.T) {
	helpers.SingleEventForChannelTestHelper(
		context.Background(),
		t,
		cloudevents.EncodingBinary,
		helpers.SubscriptionV1beta1,
		"",
		channelTestRunner,
	)
}

func TestSingleStructuredEventForChannelV1Beta1(t *testing.T) {
	helpers.SingleEventForChannelTestHelper(
		context.Background(),
		t,
		cloudevents.EncodingStructured,
		helpers.SubscriptionV1beta1,
		"",
		channelTestRunner,
	)
}

func TestSingleBinaryEventForChannelV1(t *testing.T) {
	helpers.SingleEventForChannelTestHelper(
		context.Background(),
		t,
		cloudevents.EncodingBinary,
		helpers.SubscriptionV1,
		"",
		channelTestRunner,
	)
}

func TestSingleStructuredEventForChannelV1(t *testing.T) {
	helpers.SingleEventForChannelTestHelper(
		context.Background(),
		t,
		cloudevents.EncodingStructured,
		helpers.SubscriptionV1,
		"",
		channelTestRunner,
	)
}

func TestSequence(t *testing.T) {
	helpers.SequenceTestHelper(context.Background(), t, channelTestRunner)
}

func TestSequenceV1(t *testing.T) {
	helpers.SequenceV1TestHelper(context.Background(), t, channelTestRunner)
}

func TestParallel(t *testing.T) {
	helpers.ParallelTestHelper(context.Background(), t, channelTestRunner)
}

func TestParallelV1(t *testing.T) {
	helpers.ParallelV1TestHelper(context.Background(), t, channelTestRunner)
}

/*
TestBrokerChannelFlow tests the following topology:
                   ------------- ----------------------
                   |           | |                    |
                   v	       | v                    |
EventSource ---> Broker ---> Trigger1 -------> Service(Transformation)
                   |
                   |
                   |-------> Trigger2 -------> Service(Logger1)
                   |
                   |
                   |-------> Trigger3 -------> Channel --------> Subscription --------> Service(Logger2)
Explanation:
Trigger1 filters the original event and transforms it to a new event,
Trigger2 logs all events,
Trigger3 filters the transformed event and sends it to Channel.
*/
func TestBrokerChannelFlowTriggerV1BrokerV1(t *testing.T) {
	helpers.BrokerChannelFlowWithTransformation(context.Background(), t, kafka.BrokerClass, "v1", "v1", channelTestRunner)
}
func TestBrokerChannelFlowV1Beta1BrokerV1(t *testing.T) {
	helpers.BrokerChannelFlowWithTransformation(context.Background(), t, kafka.BrokerClass, "v1", "v1beta1", channelTestRunner)
}
func TestBrokerChannelFlowTriggerV1Beta1BrokerV1Beta1(t *testing.T) {
	helpers.BrokerChannelFlowWithTransformation(context.Background(), t, kafka.BrokerClass, "v1beta1", "v1beta1", channelTestRunner)
}
func TestBrokerChannelFlowTriggerV1BrokerV1Beta1(t *testing.T) {
	helpers.BrokerChannelFlowWithTransformation(context.Background(), t, kafka.BrokerClass, "v1beta1", "v1", channelTestRunner)
}
