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
	"fmt"
	"net/http"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/openzipkin/zipkin-go/model"
	"go.opentelemetry.io/otel/api/trace"
	"knative.dev/pkg/test/zipkin"

	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	tracinghelper "knative.dev/eventing/test/conformance/helpers/tracing"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/eventing/test/lib/sender"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/kafka"
	"knative.dev/eventing-kafka-broker/test/pkg/observability"
)

// setupBrokerTracing is the general setup for TestBrokerTracing. It creates the following:
// 1. Broker.
// 2. Trigger on 'foo' events -> K8s Service -> transformer Pod (which replies with a 'bar' event).
// 3. Trigger on 'bar' events -> K8s Service -> eventdetails Pod.
// 4. Sender Pod which sends a 'foo' event.
// It returns a string that is expected to be sent by the SendEvents Pod and should be present in
// the LogEvents Pod logs.
func TestBrokerTracing(t *testing.T) {
	const (
		loggerPodName = "logger"
		etTransformer = "transformer"
		etLogger      = "et-logger"
		senderName    = "sender"
		eventID       = "event-1"
		eventBody     = `{"msg":"TestBrokerTracing event-1"}`
	)

	ctx := context.Background()

	client := testlib.Setup(t, true)
	defer testlib.TearDown(client)

	targetTracker, err := recordevents.NewEventInfoStore(client, loggerPodName, client.Namespace)
	if err != nil {
		t.Fatal("Pod tracker failed:", err)
	}

	broker := client.CreateBrokerV1Beta1OrFail(
		"br",
		resources.WithBrokerClassForBrokerV1Beta1(kafka.BrokerClass),
	)

	// Create a logger (EventRecord) Pod and a K8s Service that points to it.
	_ = recordevents.DeployEventRecordOrFail(ctx, client, loggerPodName)

	// Create a Trigger that receives events (type=bar) and sends them to the logger Pod.
	loggerTrigger := client.CreateTriggerOrFailV1Beta1(
		"logger",
		resources.WithBrokerV1Beta1(broker.Name),
		resources.WithAttributesTriggerFilterV1Beta1(v1beta1.TriggerAnyFilter, etLogger, map[string]interface{}{}),
		resources.WithSubscriberServiceRefForTriggerV1Beta1(loggerPodName),
	)

	// Create a transformer Pod (recordevents with transform reply) that replies with the same event as the input,
	// except the reply's event's type is changed to bar.
	eventTransformerPod := recordevents.DeployEventRecordOrFail(
		ctx,
		client,
		"transformer",
		recordevents.ReplyWithTransformedEvent(
			etLogger,
			senderName,
			eventBody,
		),
	)
	transformedEvent := cloudevents.NewEvent()
	transformedEvent.SetType(etLogger)
	transformedEvent.SetSource(senderName)
	if err := transformedEvent.SetData(cloudevents.ApplicationJSON, []byte(eventBody)); err != nil {
		t.Fatal("Cannot set the payload of the event:", err.Error())
	}

	// Create a Trigger that receives events (type=foo) and sends them to the transformer Pod.
	transformerTrigger := client.CreateTriggerOrFailV1Beta1(
		"transformer",
		resources.WithBrokerV1Beta1(broker.Name),
		resources.WithAttributesTriggerFilterV1Beta1(v1beta1.TriggerAnyFilter, etTransformer, map[string]interface{}{}),
		resources.WithSubscriberServiceRefForTriggerV1Beta1(eventTransformerPod.Name),
	)

	// Wait for all test resources to be ready, so that we can start sending events.
	client.WaitForAllTestResourcesReadyOrFail(ctx)

	// Everything is setup to receive an event. Generate a CloudEvent.
	event := cloudevents.NewEvent()
	event.SetID(eventID)
	event.SetSource(senderName)
	event.SetType(etTransformer)
	if err := event.SetData(cloudevents.ApplicationJSON, []byte(eventBody)); err != nil {
		t.Fatal("Cannot set the payload of the event:", err.Error())
	}

	// Send the CloudEvent
	client.SendEventToAddressable(ctx, senderName, broker.Name, testlib.BrokerTypeMeta, event, sender.EnableTracing())

	// We expect the following spans:
	// 1. Send pod sends event to the Broker Ingress (only if the sending pod generates a span).
	// 2. Broker Ingress receives the event from the sending pod.
	// 3. Broker Filter for the "transformer" trigger sends the event to the transformer pod.
	// 4. Transformer pod receives the event from the Broker Filter for the "transformer" trigger.
	// 5. Broker Filter for the "logger" trigger sends the event to the logger pod.
	// 6. Logger pod receives the event from the Broker Filter for the "logger" trigger.

	// Useful constants we will use below.
	loggerSVCHost := k8sServiceHost(client.Namespace, loggerPodName)
	transformerSVCHost := k8sServiceHost(client.Namespace, eventTransformerPod.Name)

	expected := tracinghelper.TestSpanTree{
		Note: "1. Send pod sends event to the Broker Ingress",
		Span: tracinghelper.MatchHTTPSpanNoReply(
			model.Client,
			tracinghelper.WithLocalEndpointServiceName(senderName),
		),
		Children: []tracinghelper.TestSpanTree{
			{
				Note: "2. Broker Ingress receives the event from the sending pod.",
				Span: observability.IngressSpan(broker, event),
				Children: []tracinghelper.TestSpanTree{
					{
						Note: "3. Broker Filter for the 'transformer' trigger sends the event to the transformer pod.",
						Span: observability.TriggerSpan(transformerTrigger, event),
						Children: []tracinghelper.TestSpanTree{
							{
								Note: "4. Transformer pod receives the event from the Broker Filter for the 'transformer' trigger.",
								Span: tracinghelper.MatchHTTPSpanWithReply(
									model.Server,
									tracinghelper.WithHTTPHostAndPath(transformerSVCHost, "/"),
									tracinghelper.WithLocalEndpointServiceName(eventTransformerPod.Name),
								),
							},
						},
					},
					{
						Note: "5. Producer for reply from the 'transformer'",
						Span: observability.ProducerSpan(transformedEvent),
						Children: []tracinghelper.TestSpanTree{
							{
								Note: "6. Broker Filter for the 'logger' trigger sends the event to the logger pod.",
								Span: observability.TriggerSpan(loggerTrigger, transformedEvent),
								Children: []tracinghelper.TestSpanTree{
									{
										Note: "7. Logger pod receives the event from the Broker Filter for the 'logger' trigger.",
										Span: tracinghelper.MatchHTTPSpanNoReply(
											model.Server,
											tracinghelper.WithHTTPHostAndPath(loggerSVCHost, "/"),
											tracinghelper.WithLocalEndpointServiceName(loggerPodName),
										),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	matches := cetest.AllOf(
		cetest.HasSource(senderName),
		cetest.HasId(eventID),
		cetest.DataContains(eventBody),
	)

	eventInfo := targetTracker.AssertAtLeast(1, recordevents.MatchEvent(matches))

	// Match the trace
	traceID := getTraceIDHeader(t, eventInfo)
	outputTrace, err := zipkin.JSONTracePred(traceID, 5*time.Minute, func(trace []model.SpanModel) bool {
		tree, err := tracinghelper.GetTraceTree(trace)
		if err != nil {
			return false
		}
		// Do not pass t to prevent unnecessary log output.
		return len(expected.MatchesSubtree(nil, tree)) > 0
	})
	if err != nil {
		t.Logf("Unable to get trace %q: %v. Trace so far %+v", traceID, err, tracinghelper.PrettyPrintTrace(outputTrace))
		tree, err := tracinghelper.GetTraceTree(outputTrace)
		if err != nil {
			t.Fatal(err)
		}
		if len(expected.MatchesSubtree(t, tree)) == 0 {
			t.Fatalf("No matching subtree. want: %v got: %v", expected, tree)
		}
	}
}

// getTraceIDHeader gets the TraceID from the passed in events.  It returns the header from the
// first matching event, but registers a fatal error if more than one traceid header is seen
// in that message.
func getTraceIDHeader(t *testing.T, evInfos []recordevents.EventInfo) string {
	for i := range evInfos {
		if nil != evInfos[i].HTTPHeaders {
			sc := trace.RemoteSpanContextFromContext(trace.DefaultHTTPPropagator().Extract(context.TODO(), http.Header(evInfos[i].HTTPHeaders)))
			if sc.HasTraceID() {
				return sc.TraceIDString()
			}
		}
	}
	t.Fatalf("FAIL: No traceid in %d messages: (%v)", len(evInfos), evInfos)
	return ""
}

func k8sServiceHost(namespace, svcName string) string {
	return fmt.Sprintf("%s.%s.svc", svcName, namespace)
}
