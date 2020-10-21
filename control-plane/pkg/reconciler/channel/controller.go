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

	"github.com/Shopify/sarama"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap"
	podinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/pod"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/resolver"

	channelinformer "knative.dev/eventing-kafka/pkg/client/injection/informers/messaging/v1beta1/kafkachannel"
	channelreconciler "knative.dev/eventing-kafka/pkg/client/injection/reconciler/messaging/v1beta1/kafkachannel"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/config"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/base"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/resource"
)

const (
	cmConfig = "kafka-config"
)

func NewController(ctx context.Context, _ configmap.Watcher, configs *config.Env) *controller.Impl {

	configmapInformer := configmapinformer.Get(ctx)

	reconciler := &Reconciler{
		Reconciler: &resource.Reconciler{
			Reconciler: &base.Reconciler{
				KubeClient:                  kubeclient.Get(ctx),
				PodLister:                   podinformer.Get(ctx).Lister(),
				DataPlaneConfigMapNamespace: configs.DataPlaneConfigMapNamespace,
				DataPlaneConfigMapName:      configs.DataPlaneConfigMapName,
				DataPlaneConfigFormat:       configs.DataPlaneConfigFormat,
				SystemNamespace:             configs.SystemNamespace,
				DispatcherLabel:             base.ChannelDispatcherLabel,
				ReceiverLabel:               base.ChannelReceiverLabel,
			},
			TopicConfigProvider: nil,
			TopicPrefix:         "",
			ResourceCreator:     nil,
			ClusterAdmin:        sarama.NewClusterAdmin,
			Configs:             configs,
		},
		Resolver:        nil,
		Configs:         configs,
		ConfigMapLister: configmapInformer.Lister(),
	}

	impl := channelreconciler.NewImpl(ctx, reconciler)

	reconciler.Reconciler.TopicConfigProvider = reconciler.topicConfig
	reconciler.Reconciler.ResourceCreator = reconciler.ResourceCreator
	reconciler.Reconciler.TopicPrefix = TopicPrefix
	reconciler.Resolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	channelinformer.Get(ctx).Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	return impl
}
