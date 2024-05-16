// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/apiserver/registry/seedmanagement/gardenlet"
)

// REST implements a RESTStorage for Gardenlet.
type REST struct {
	*genericregistry.Store
}

// GardenletStorage implements the storage for Gardenlets and their status subresource.
type GardenletStorage struct {
	Gardenlet *REST
	Status    *StatusREST
}

// NewStorage creates a new GardenletStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) GardenletStorage {
	managedSeedRest, managedSeedStatusRest := NewREST(optsGetter)

	return GardenletStorage{
		Gardenlet: managedSeedRest,
		Status:    managedSeedStatusRest,
	}
}

// NewREST returns a RESTStorage object that will work with Gardenlet objects.
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *StatusREST) {
	strategy := gardenlet.NewStrategy()
	statusStrategy := gardenlet.NewStatusStrategy()

	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &seedmanagement.Gardenlet{} },
		NewListFunc:               func() runtime.Object { return &seedmanagement.GardenletList{} },
		DefaultQualifiedResource:  seedmanagement.Resource("gardenlets"),
		SingularQualifiedResource: seedmanagement.Resource("gardenlet"),
		EnableGarbageCollection:   true,

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
		DeleteStrategy: strategy,

		TableConvertor: newTableConvertor(),
	}
	options := &generic.StoreOptions{
		RESTOptions: optsGetter,
	}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	statusStore := *store
	statusStore.UpdateStrategy = statusStrategy

	return &REST{store}, &StatusREST{store: &statusStore}
}

// StatusREST implements the REST endpoint for changing the status of a Gardenlet.
type StatusREST struct {
	store *genericregistry.Store
}

var (
	_ rest.Storage = &StatusREST{}
	_ rest.Getter  = &StatusREST{}
	_ rest.Updater = &StatusREST{}
)

// New creates a new (empty) internal Gardenlet object.
func (r *StatusREST) New() runtime.Object {
	return &seedmanagement.Gardenlet{}
}

// Destroy cleans up its resources on shutdown.
func (r *StatusREST) Destroy() {
	// Given that underlying store is shared with REST,
	// we don't destroy it here explicitly.
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *StatusREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
}

// Implement ShortNamesProvider
var _ rest.ShortNamesProvider = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"glet"}
}
