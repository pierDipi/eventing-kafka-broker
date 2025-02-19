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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
	v1alpha1 "knative.dev/eventing-kafka-broker/control-plane/pkg/apis/internalskafkaeventing/v1alpha1"
)

// FakeConsumerGroups implements ConsumerGroupInterface
type FakeConsumerGroups struct {
	Fake *FakeInternalV1alpha1
	ns   string
}

var consumergroupsResource = v1alpha1.SchemeGroupVersion.WithResource("consumergroups")

var consumergroupsKind = v1alpha1.SchemeGroupVersion.WithKind("ConsumerGroup")

// Get takes name of the consumerGroup, and returns the corresponding consumerGroup object, and an error if there is any.
func (c *FakeConsumerGroups) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ConsumerGroup, err error) {
	emptyResult := &v1alpha1.ConsumerGroup{}
	obj, err := c.Fake.
		Invokes(testing.NewGetActionWithOptions(consumergroupsResource, c.ns, name, options), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.ConsumerGroup), err
}

// List takes label and field selectors, and returns the list of ConsumerGroups that match those selectors.
func (c *FakeConsumerGroups) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ConsumerGroupList, err error) {
	emptyResult := &v1alpha1.ConsumerGroupList{}
	obj, err := c.Fake.
		Invokes(testing.NewListActionWithOptions(consumergroupsResource, consumergroupsKind, c.ns, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ConsumerGroupList{ListMeta: obj.(*v1alpha1.ConsumerGroupList).ListMeta}
	for _, item := range obj.(*v1alpha1.ConsumerGroupList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested consumerGroups.
func (c *FakeConsumerGroups) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchActionWithOptions(consumergroupsResource, c.ns, opts))

}

// Create takes the representation of a consumerGroup and creates it.  Returns the server's representation of the consumerGroup, and an error, if there is any.
func (c *FakeConsumerGroups) Create(ctx context.Context, consumerGroup *v1alpha1.ConsumerGroup, opts v1.CreateOptions) (result *v1alpha1.ConsumerGroup, err error) {
	emptyResult := &v1alpha1.ConsumerGroup{}
	obj, err := c.Fake.
		Invokes(testing.NewCreateActionWithOptions(consumergroupsResource, c.ns, consumerGroup, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.ConsumerGroup), err
}

// Update takes the representation of a consumerGroup and updates it. Returns the server's representation of the consumerGroup, and an error, if there is any.
func (c *FakeConsumerGroups) Update(ctx context.Context, consumerGroup *v1alpha1.ConsumerGroup, opts v1.UpdateOptions) (result *v1alpha1.ConsumerGroup, err error) {
	emptyResult := &v1alpha1.ConsumerGroup{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateActionWithOptions(consumergroupsResource, c.ns, consumerGroup, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.ConsumerGroup), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeConsumerGroups) UpdateStatus(ctx context.Context, consumerGroup *v1alpha1.ConsumerGroup, opts v1.UpdateOptions) (result *v1alpha1.ConsumerGroup, err error) {
	emptyResult := &v1alpha1.ConsumerGroup{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceActionWithOptions(consumergroupsResource, "status", c.ns, consumerGroup, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.ConsumerGroup), err
}

// Delete takes name of the consumerGroup and deletes it. Returns an error if one occurs.
func (c *FakeConsumerGroups) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(consumergroupsResource, c.ns, name, opts), &v1alpha1.ConsumerGroup{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeConsumerGroups) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionActionWithOptions(consumergroupsResource, c.ns, opts, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.ConsumerGroupList{})
	return err
}

// Patch applies the patch and returns the patched consumerGroup.
func (c *FakeConsumerGroups) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ConsumerGroup, err error) {
	emptyResult := &v1alpha1.ConsumerGroup{}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithOptions(consumergroupsResource, c.ns, name, pt, data, opts, subresources...), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.ConsumerGroup), err
}

// GetScale takes name of the consumerGroup, and returns the corresponding scale object, and an error if there is any.
func (c *FakeConsumerGroups) GetScale(ctx context.Context, consumerGroupName string, options v1.GetOptions) (result *autoscalingv1.Scale, err error) {
	emptyResult := &autoscalingv1.Scale{}
	obj, err := c.Fake.
		Invokes(testing.NewGetSubresourceActionWithOptions(consumergroupsResource, c.ns, "scale", consumerGroupName, options), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*autoscalingv1.Scale), err
}

// UpdateScale takes the representation of a scale and updates it. Returns the server's representation of the scale, and an error, if there is any.
func (c *FakeConsumerGroups) UpdateScale(ctx context.Context, consumerGroupName string, scale *autoscalingv1.Scale, opts v1.UpdateOptions) (result *autoscalingv1.Scale, err error) {
	emptyResult := &autoscalingv1.Scale{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceActionWithOptions(consumergroupsResource, "scale", c.ns, scale, opts), &autoscalingv1.Scale{})

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*autoscalingv1.Scale), err
}
