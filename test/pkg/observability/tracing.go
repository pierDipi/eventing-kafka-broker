package observability

import (
	"fmt"
	"regexp"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/openzipkin/zipkin-go/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	tracinghelper "knative.dev/eventing/test/conformance/helpers/tracing"
)

var (
	nonEmpty = regexp.MustCompile("^[a-z]+$")
	// This regex doesn't match all possible <namespace>/<name> patterns. However, it's reasonable to assume
	// that tests use only reasonable names.
	url        = regexp.MustCompile(".+/[a-zA-Z_-]+/[a-zA-Z_-]+")
	httpMethod = regexp.MustCompile(exactly("POST"))
)

func IngressSpan(broker metav1.Object, event cloudevents.Event) *tracinghelper.SpanMatcher {
	server := model.Server
	return &tracinghelper.SpanMatcher{
		Kind: &server,
		Tags: map[string]*regexp.Regexp{
			"http.method":      httpMethod,
			"http.url":         url,
			"messaging.system": nonEmpty,
			// TODO we don't have resource references on the data plane yet.
			// "messaging.destination":    regexp.MustCompile(fmt.Sprintf("^broker:%s.%s$", broker.GetNamespace(), broker.GetName())),
			"messaging.message_id":     regexp.MustCompile("^" + event.ID() + "$"),
			"messaging.message_type":   regexp.MustCompile(exactly(event.Type())),
			"messaging.message_source": regexp.MustCompile(exactly(event.Source())),
			"service.name":             nonEmpty,
			"service.namespace":        nonEmpty,
		},
	}
}

func ProducerSpan(event cloudevents.Event) *tracinghelper.SpanMatcher {
	producer := model.Producer
	return &tracinghelper.SpanMatcher{
		Kind: &producer,
		Tags: map[string]*regexp.Regexp{
			"peer.address":            nonEmpty,
			"message_bus.destination": nonEmpty,
			"peer.hostname":           nonEmpty,
			"peer.port":               nonEmpty,
			"peer.service":            nonEmpty,
			"http.url":                url,
			"service.name":            nonEmpty,
			"service.namespace":       nonEmpty,
			// TODO to be implemented
			// "messaging.message_id":     regexp.MustCompile("^" + event.ID + "$"),
			// "messaging.message_type":   regexp.MustCompile(exactly(event.Type)),
			// "messaging.message_source": regexp.MustCompile(exactly(event.Source)),
		},
	}
}

func ConsumerSpan(event cloudevents.Event) *tracinghelper.SpanMatcher {
	consumer := model.Consumer
	return &tracinghelper.SpanMatcher{
		Kind: &consumer,
		Tags: map[string]*regexp.Regexp{
			"peer.address":             nonEmpty,
			"message_bus.destination":  nonEmpty,
			"peer.hostname":            nonEmpty,
			"peer.port":                nonEmpty,
			"peer.service":             nonEmpty,
			"http.url":                 url,
			"service.name":             nonEmpty,
			"service.namespace":        nonEmpty,
			"messaging.message_id":     regexp.MustCompile("^" + event.ID() + "$"),
			"messaging.message_type":   regexp.MustCompile(exactly(event.Type())),
			"messaging.message_source": regexp.MustCompile(exactly(event.Source())),
		},
	}
}

func TriggerSpan(trigger metav1.Object, event cloudevents.Event) *tracinghelper.SpanMatcher {
	client := model.Client
	return &tracinghelper.SpanMatcher{
		Kind: &client,
		Tags: map[string]*regexp.Regexp{
			"http.method":      httpMethod,
			"http.url":         url,
			"messaging.system": nonEmpty,
			// TODO we don't have resource references on the data plane yet.
			// "messaging.destination":    regexp.MustCompile(fmt.Sprintf("^trigger:%s.%s$", trigger.GetNamespace(), trigger.GetName())),
			"messaging.message_id":     regexp.MustCompile("^" + event.ID() + "$"),
			"messaging.message_type":   regexp.MustCompile(exactly(event.Type())),
			"messaging.message_source": regexp.MustCompile(exactly(event.Source())),
			"service.name":             nonEmpty,
			"service.namespace":        nonEmpty,
		},
	}
}

func exactly(s string) string {
	return fmt.Sprintf("^%s$", s)
}
