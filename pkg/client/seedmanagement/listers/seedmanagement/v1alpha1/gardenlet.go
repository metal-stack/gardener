// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// GardenletLister helps list Gardenlets.
// All objects returned here must be treated as read-only.
type GardenletLister interface {
	// List lists all Gardenlets in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.Gardenlet, err error)
	// Gardenlets returns an object that can list and get Gardenlets.
	Gardenlets(namespace string) GardenletNamespaceLister
	GardenletListerExpansion
}

// gardenletLister implements the GardenletLister interface.
type gardenletLister struct {
	indexer cache.Indexer
}

// NewGardenletLister returns a new GardenletLister.
func NewGardenletLister(indexer cache.Indexer) GardenletLister {
	return &gardenletLister{indexer: indexer}
}

// List lists all Gardenlets in the indexer.
func (s *gardenletLister) List(selector labels.Selector) (ret []*v1alpha1.Gardenlet, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.Gardenlet))
	})
	return ret, err
}

// Gardenlets returns an object that can list and get Gardenlets.
func (s *gardenletLister) Gardenlets(namespace string) GardenletNamespaceLister {
	return gardenletNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// GardenletNamespaceLister helps list and get Gardenlets.
// All objects returned here must be treated as read-only.
type GardenletNamespaceLister interface {
	// List lists all Gardenlets in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.Gardenlet, err error)
	// Get retrieves the Gardenlet from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.Gardenlet, error)
	GardenletNamespaceListerExpansion
}

// gardenletNamespaceLister implements the GardenletNamespaceLister
// interface.
type gardenletNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all Gardenlets in the indexer for a given namespace.
func (s gardenletNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.Gardenlet, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.Gardenlet))
	})
	return ret, err
}

// Get retrieves the Gardenlet from the indexer for a given namespace and name.
func (s gardenletNamespaceLister) Get(name string) (*v1alpha1.Gardenlet, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("gardenlet"), name)
	}
	return obj.(*v1alpha1.Gardenlet), nil
}