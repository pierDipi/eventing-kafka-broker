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

package main

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"

	sourcesv1beta1 "knative.dev/eventing-kafka/pkg/apis/sources/v1beta1"
	eventingcorev1beta1 "knative.dev/eventing/pkg/apis/eventing/v1"

	messagingv1beta1 "knative.dev/eventing-kafka/pkg/apis/messaging/v1beta1"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/apis/core"
	eventingv1 "knative.dev/eventing-kafka-broker/control-plane/pkg/apis/eventing/v1"
	eventingv1alpha1 "knative.dev/eventing-kafka-broker/control-plane/pkg/apis/eventing/v1alpha1"
)

const (
	defaultWebhookPort = 8443
)

var types = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	eventingv1alpha1.SchemeGroupVersion.WithKind("KafkaSink"):    &eventingv1alpha1.KafkaSink{},
	sourcesv1beta1.SchemeGroupVersion.WithKind("KafkaSource"):    &sourcesv1beta1.KafkaSource{},
	messagingv1beta1.SchemeGroupVersion.WithKind("KafkaChannel"): &messagingv1beta1.KafkaChannel{},
}

var validationCallbacks = map[schema.GroupVersionKind]validation.Callback{
	eventingcorev1beta1.SchemeGroupVersion.WithKind("Broker"): eventingv1.BrokerValidationCallback(),
}

var defaultingCallbacks = map[schema.GroupVersionKind]defaulting.Callback{
	corev1.SchemeGroupVersion.WithKind("Pod"): core.DispatcherPodsDefaulting(),
}

func NewDefaultingAdmissionController(ctx context.Context, _ configmap.Watcher) *controller.Impl {

	// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
	ctxFunc := func(ctx context.Context) context.Context {
		return ctx
	}

	return defaulting.NewAdmissionController(ctx,
		// Name of the resource webhook.
		"defaulting.webhook.kafka.eventing.knative.dev",

		// The path on which to serve the webhook.
		"/defaulting",

		// The resources to default.
		types,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		ctxFunc,

		// Whether to disallow unknown fields.
		false,
	)
}

func NewPodDefaultingAdmissionController(ctx context.Context, _ configmap.Watcher) *controller.Impl {

	// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
	ctxFunc := func(ctx context.Context) context.Context {
		return ctx
	}

	return defaulting.NewAdmissionController(ctx,
		// Name of the resource webhook.
		"pods.defaulting.webhook.kafka.eventing.knative.dev",

		// The path on which to serve the webhook.
		"/pods-defaulting",

		// The resources to default.
		// We use only defaulting callbacks for pods.
		map[schema.GroupVersionKind]resourcesemantics.GenericCRD{},

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		ctxFunc,

		// Whether to disallow unknown fields.
		false,

		// Extra defaulting callbacks to be applied to resources.
		defaultingCallbacks,
	)
}

func NewValidationAdmissionController(ctx context.Context, _ configmap.Watcher) *controller.Impl {
	return validation.NewAdmissionController(ctx,
		// Name of the resource webhook.
		"validation.webhook.kafka.eventing.knative.dev",

		// The path on which to serve the webhook.
		"/resource-validation",

		// The resources to validate.
		types,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			return ctx
		},

		// Whether to disallow unknown fields.
		true,

		// Extra validating callbacks to be applied to resources.
		validationCallbacks,
	)
}

func main() {

	// Set up a signal context with our webhook options
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: webhook.NameFromEnv(),
		Port:        webhook.PortFromEnv(defaultWebhookPort),
		// SecretName must match the name of the Secret created in the configuration.
		SecretName: "kafka-webhook-eventing-certs",
	})

	sharedmain.MainWithContext(ctx, webhook.NameFromEnv(),
		certificates.NewController,
		NewDefaultingAdmissionController,
		NewPodDefaultingAdmissionController,
		NewValidationAdmissionController,
	)
}
