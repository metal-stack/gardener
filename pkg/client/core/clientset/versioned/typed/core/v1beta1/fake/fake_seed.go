/*
Copyright (c) SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

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

package fake

import (
	"context"

	v1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeSeeds implements SeedInterface
type FakeSeeds struct {
	Fake *FakeCoreV1beta1
}

var seedsResource = schema.GroupVersionResource{Group: "core.gardener.cloud", Version: "v1beta1", Resource: "seeds"}

var seedsKind = schema.GroupVersionKind{Group: "core.gardener.cloud", Version: "v1beta1", Kind: "Seed"}

// Get takes name of the seed, and returns the corresponding seed object, and an error if there is any.
func (c *FakeSeeds) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.Seed, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(seedsResource, name), &v1beta1.Seed{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Seed), err
}

// List takes label and field selectors, and returns the list of Seeds that match those selectors.
func (c *FakeSeeds) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.SeedList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(seedsResource, seedsKind, opts), &v1beta1.SeedList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.SeedList{ListMeta: obj.(*v1beta1.SeedList).ListMeta}
	for _, item := range obj.(*v1beta1.SeedList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested seeds.
func (c *FakeSeeds) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(seedsResource, opts))
}

// Create takes the representation of a seed and creates it.  Returns the server's representation of the seed, and an error, if there is any.
func (c *FakeSeeds) Create(ctx context.Context, seed *v1beta1.Seed, opts v1.CreateOptions) (result *v1beta1.Seed, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(seedsResource, seed), &v1beta1.Seed{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Seed), err
}

// Update takes the representation of a seed and updates it. Returns the server's representation of the seed, and an error, if there is any.
func (c *FakeSeeds) Update(ctx context.Context, seed *v1beta1.Seed, opts v1.UpdateOptions) (result *v1beta1.Seed, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(seedsResource, seed), &v1beta1.Seed{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Seed), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeSeeds) UpdateStatus(ctx context.Context, seed *v1beta1.Seed, opts v1.UpdateOptions) (*v1beta1.Seed, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(seedsResource, "status", seed), &v1beta1.Seed{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Seed), err
}

// Delete takes name of the seed and deletes it. Returns an error if one occurs.
func (c *FakeSeeds) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(seedsResource, name, opts), &v1beta1.Seed{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeSeeds) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(seedsResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1beta1.SeedList{})
	return err
}

// Patch applies the patch and returns the patched seed.
func (c *FakeSeeds) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.Seed, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(seedsResource, name, pt, data, subresources...), &v1beta1.Seed{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Seed), err
}
