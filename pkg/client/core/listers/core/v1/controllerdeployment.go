// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/gardener/gardener/pkg/apis/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// ControllerDeploymentLister helps list ControllerDeployments.
// All objects returned here must be treated as read-only.
type ControllerDeploymentLister interface {
	// List lists all ControllerDeployments in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.ControllerDeployment, err error)
	// Get retrieves the ControllerDeployment from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1.ControllerDeployment, error)
	ControllerDeploymentListerExpansion
}

// controllerDeploymentLister implements the ControllerDeploymentLister interface.
type controllerDeploymentLister struct {
	indexer cache.Indexer
}

// NewControllerDeploymentLister returns a new ControllerDeploymentLister.
func NewControllerDeploymentLister(indexer cache.Indexer) ControllerDeploymentLister {
	return &controllerDeploymentLister{indexer: indexer}
}

// List lists all ControllerDeployments in the indexer.
func (s *controllerDeploymentLister) List(selector labels.Selector) (ret []*v1.ControllerDeployment, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.ControllerDeployment))
	})
	return ret, err
}

// Get retrieves the ControllerDeployment from the index for a given name.
func (s *controllerDeploymentLister) Get(name string) (*v1.ControllerDeployment, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("controllerdeployment"), name)
	}
	return obj.(*v1.ControllerDeployment), nil
}
