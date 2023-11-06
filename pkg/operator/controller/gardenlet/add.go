// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gardenlet

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "gardenlet"
)

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	virtualCluster cluster.Cluster,
) error {
	if r.RuntimeCluster == nil {
		r.RuntimeCluster = mgr
	}
	if r.RuntimeClient == nil {
		r.RuntimeClient = mgr.GetClient()
	}
	if r.VirtualConfig == nil {
		r.VirtualConfig = virtualCluster.GetConfig()
	}
	if r.VirtualAPIReader == nil {
		r.VirtualAPIReader = virtualCluster.GetAPIReader()
	}
	if r.VirtualClient == nil {
		r.VirtualClient = virtualCluster.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.GardenNamespaceTarget == "" {
		r.GardenNamespaceTarget = v1beta1constants.GardenNamespace
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(ControllerName + "-controller")
	}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
		}).
		For(&operatorv1alpha1.Gardenlet{}, builder.WithPredicates(&predicate.GenerationChangedPredicate{})).
		Build(r)
	if err != nil {
		return err
	}

	return c.Watch(
		source.Kind(virtualCluster.GetCache(), &gardencorev1beta1.Seed{}),
		mapper.EnqueueRequestsFrom(ctx, virtualCluster.GetCache(), mapper.MapFunc(r.MapSeedToGardenlet), mapper.UpdateWithNew, c.GetLogger()),
		r.SeedOfGardenletPredicate(ctx),
	)
}

// SeedOfGardenletPredicate returns the predicate for Seed events.
func (r *Reconciler) SeedOfGardenletPredicate(ctx context.Context) predicate.Predicate {
	return &seedOfGardenletPredicate{
		ctx:    ctx,
		reader: r.VirtualClient,
	}
}

type seedOfGardenletPredicate struct {
	ctx    context.Context
	reader client.Reader
}

func (p *seedOfGardenletPredicate) Create(e event.CreateEvent) bool {
	return p.filterSeedOfGardenlet(e.Object)
}

func (p *seedOfGardenletPredicate) Update(e event.UpdateEvent) bool {
	return p.filterSeedOfGardenlet(e.ObjectNew)
}

func (p *seedOfGardenletPredicate) Delete(e event.DeleteEvent) bool {
	return p.filterSeedOfGardenlet(e.Object)
}

func (p *seedOfGardenletPredicate) Generic(_ event.GenericEvent) bool { return false }

// filterSeedOfGardenlet is filtering func for Seeds that checks if the Seed is owned by a Gardenlet.
func (p *seedOfGardenletPredicate) filterSeedOfGardenlet(obj client.Object) bool {
	seed, ok := obj.(*gardencorev1beta1.Seed)
	if !ok {
		return false
	}

	return p.reader.Get(p.ctx, kubernetesutils.Key(seed.Name), &operatorv1alpha1.Gardenlet{}) == nil
}

// MapSeedToGardenlet is a mapper.MapFunc for mapping a Seed to the owning Gardenlet.
func (r *Reconciler) MapSeedToGardenlet(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName()}}}
}
