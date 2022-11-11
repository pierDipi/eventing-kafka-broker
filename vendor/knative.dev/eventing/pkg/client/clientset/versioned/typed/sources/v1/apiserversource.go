/*
Copyright 2021 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	scheme "knative.dev/eventing/pkg/client/clientset/versioned/scheme"
)

// ApiServerSourcesGetter has a method to return a ApiServerSourceInterface.
// A group's client should implement this interface.
type ApiServerSourcesGetter interface {
	ApiServerSources(namespace string) ApiServerSourceInterface
}

// ApiServerSourceInterface has methods to work with ApiServerSource resources.
type ApiServerSourceInterface interface {
	Create(ctx context.Context, apiServerSource *v1.ApiServerSource, opts metav1.CreateOptions) (*v1.ApiServerSource, error)
	Update(ctx context.Context, apiServerSource *v1.ApiServerSource, opts metav1.UpdateOptions) (*v1.ApiServerSource, error)
	UpdateStatus(ctx context.Context, apiServerSource *v1.ApiServerSource, opts metav1.UpdateOptions) (*v1.ApiServerSource, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.ApiServerSource, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.ApiServerSourceList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.ApiServerSource, err error)
	ApiServerSourceExpansion
}

// apiServerSources implements ApiServerSourceInterface
type apiServerSources struct {
	client rest.Interface
	ns     string
}

// newApiServerSources returns a ApiServerSources
func newApiServerSources(c *SourcesV1Client, namespace string) *apiServerSources {
	return &apiServerSources{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the apiServerSource, and returns the corresponding apiServerSource object, and an error if there is any.
func (c *apiServerSources) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.ApiServerSource, err error) {
	result = &v1.ApiServerSource{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("apiserversources").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ApiServerSources that match those selectors.
func (c *apiServerSources) List(ctx context.Context, opts metav1.ListOptions) (result *v1.ApiServerSourceList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.ApiServerSourceList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("apiserversources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested apiServerSources.
func (c *apiServerSources) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("apiserversources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a apiServerSource and creates it.  Returns the server's representation of the apiServerSource, and an error, if there is any.
func (c *apiServerSources) Create(ctx context.Context, apiServerSource *v1.ApiServerSource, opts metav1.CreateOptions) (result *v1.ApiServerSource, err error) {
	result = &v1.ApiServerSource{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("apiserversources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(apiServerSource).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a apiServerSource and updates it. Returns the server's representation of the apiServerSource, and an error, if there is any.
func (c *apiServerSources) Update(ctx context.Context, apiServerSource *v1.ApiServerSource, opts metav1.UpdateOptions) (result *v1.ApiServerSource, err error) {
	result = &v1.ApiServerSource{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("apiserversources").
		Name(apiServerSource.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(apiServerSource).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *apiServerSources) UpdateStatus(ctx context.Context, apiServerSource *v1.ApiServerSource, opts metav1.UpdateOptions) (result *v1.ApiServerSource, err error) {
	result = &v1.ApiServerSource{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("apiserversources").
		Name(apiServerSource.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(apiServerSource).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the apiServerSource and deletes it. Returns an error if one occurs.
func (c *apiServerSources) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("apiserversources").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *apiServerSources) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("apiserversources").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched apiServerSource.
func (c *apiServerSources) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.ApiServerSource, err error) {
	result = &v1.ApiServerSource{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("apiserversources").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
