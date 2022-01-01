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

// Code generated by informer-gen. DO NOT EDIT.

package v1beta1

import (
	"context"
	time "time"

	corev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	versioned "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	internalinterfaces "github.com/gardener/gardener/pkg/client/core/informers/externalversions/internalinterfaces"
	v1beta1 "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// ControllerDeploymentInformer provides access to a shared informer and lister for
// ControllerDeployments.
type ControllerDeploymentInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1beta1.ControllerDeploymentLister
}

type controllerDeploymentInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewControllerDeploymentInformer constructs a new informer for ControllerDeployment type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewControllerDeploymentInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredControllerDeploymentInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredControllerDeploymentInformer constructs a new informer for ControllerDeployment type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredControllerDeploymentInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CoreV1beta1().ControllerDeployments().List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CoreV1beta1().ControllerDeployments().Watch(context.TODO(), options)
			},
		},
		&corev1beta1.ControllerDeployment{},
		resyncPeriod,
		indexers,
	)
}

func (f *controllerDeploymentInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredControllerDeploymentInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *controllerDeploymentInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&corev1beta1.ControllerDeployment{}, f.defaultInformer)
}

func (f *controllerDeploymentInformer) Lister() v1beta1.ControllerDeploymentLister {
	return v1beta1.NewControllerDeploymentLister(f.Informer().GetIndexer())
}
