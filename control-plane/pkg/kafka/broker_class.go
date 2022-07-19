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

package kafka

import (
	corev1 "k8s.io/api/core/v1"
	pkgreconciler "knative.dev/pkg/reconciler"

	brokerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/broker"
)

const (
	// Kafka broker class annotation value.
	BrokerClass           = "Kafka"
	NamespacedBrokerClass = "KafkaNamespaced"

	NamespacedBrokerDataplaneLabelKey   = "eventing.knative.dev/namespaced"
	NamespacedBrokerDataplaneLabelValue = "true"
)

func BrokerClassFilter() func(interface{}) bool {
	return pkgreconciler.AnnotationFilterFunc(
		brokerreconciler.ClassAnnotationKey,
		BrokerClass,
		false, // allowUnset
	)
}

func NamespacedBrokerClassFilter() func(interface{}) bool {
	return pkgreconciler.AnnotationFilterFunc(
		brokerreconciler.ClassAnnotationKey,
		NamespacedBrokerClass,
		false, // allowUnset
	)
}

func NamespacedDataplaneLabelConfigmapOption(cm *corev1.ConfigMap) {
	if len(cm.Labels) == 0 {
		cm.Labels = make(map[string]string, 1)
	}
	cm.Labels[NamespacedBrokerDataplaneLabelKey] = NamespacedBrokerDataplaneLabelValue
}
