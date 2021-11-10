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
	"log"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/config"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/broker"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/trigger"
)

const (
	component = "kafka-broker-controller"
)

func main() {

	brokerEnv, err := config.GetEnvConfig("BROKER", broker.ValidateDefaultBackoffDelayMs)
	if err != nil {
		log.Fatal("cannot process environment variables with prefix BROKER", err)
	}

	// we just want the contollers for Broker/Trigger on 0.26
	//sinkEnv, err := config.GetEnvConfig("SINK")
	//if err != nil {
	//	log.Fatal("cannot process environment variables with prefix SINK", err)
	//}

	sharedmain.Main(
		component,

		// Broker controller
		func(ctx context.Context, watcher configmap.Watcher) *controller.Impl {
			return broker.NewController(ctx, watcher, &broker.Configs{Env: *brokerEnv})
		},

		// Trigger controller
		func(ctx context.Context, watcher configmap.Watcher) *controller.Impl {
			return trigger.NewController(ctx, watcher, brokerEnv)
		},

		// we just want the contollers for Broker/Trigger on 0.26
		//// KafkaSink controller
		//func(ctx context.Context, watcher configmap.Watcher) *controller.Impl {
		//	return sink.NewController(ctx, watcher, sinkEnv)
		//},
	)
}
