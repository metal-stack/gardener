// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package runtimeconfig

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	"github.com/gardener/gardener/pkg/utils/oci"
)

// ControllerName is the name of this controller.
const (
	ControllerName     = "extensions-runtime-config"
	ManagedByLabel     = "app.kubernetes.io/managed-by"
	ExtensionNameLabel = "extension-name"
)

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager) error {
	var err error

	if r.RuntimeClientSet == nil {
		r.RuntimeClientSet, err = kubernetes.NewWithConfig(
			kubernetes.WithRESTConfig(mgr.GetConfig()),
			kubernetes.WithRuntimeAPIReader(mgr.GetAPIReader()),
			kubernetes.WithRuntimeClient(mgr.GetClient()),
			kubernetes.WithRuntimeCache(mgr.GetCache()),
		)
		if err != nil {
			return fmt.Errorf("failed creating runtime clientset: %w", err)
		}
	}
	if r.RuntimeVersion == nil {
		serverVersion, err := r.RuntimeClientSet.DiscoverVersion()
		if err != nil {
			return fmt.Errorf("failed getting server version for runtime cluster: %w", err)
		}
		r.RuntimeVersion, err = semver.NewVersion(serverVersion.GitVersion)
		if err != nil {
			return fmt.Errorf("failed parsing version %q for runtime cluster: %w", serverVersion.GitVersion, err)
		}
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.HelmRegistry == nil {
		helmRegisty, err := oci.NewHelmRegistry()
		if err != nil {
			return err
		}
		r.HelmRegistry = helmRegisty
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(ControllerName + "-controller")
	}

	managedResourceLabelPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			ManagedByLabel: ControllerName,
		},
	})
	if err != nil {
		return err
	}
	// c, err := builder.
	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&operatorv1alpha1.Garden{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.Controllers.ExtensionGardenConfig.ConcurrentSyncs, 0),
		}).
		Watches(
			&v1alpha1.ManagedResource{},
			mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapToAllGardens), mapper.UpdateWithNew, mgr.GetLogger()),
			builder.WithPredicates(managedResourceLabelPredicate)).
		Watches(
			&operatorv1alpha1.Extension{},
			mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapToAllGardens), mapper.UpdateWithNew, mgr.GetLogger()),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Complete(r)
}

// MapToAllGardens returns reconcile.Request objects for all existing gardens in the system.
func (r *Reconciler) MapToAllGardens(ctx context.Context, log logr.Logger, reader client.Reader, _ client.Object) []reconcile.Request {
	gardenList := &metav1.PartialObjectMetadataList{}
	gardenList.SetGroupVersionKind(operatorv1alpha1.SchemeGroupVersion.WithKind("GardenList"))
	if err := reader.List(ctx, gardenList); err != nil {
		log.Error(err, "Failed to list gardens")
		return nil
	}

	return mapper.ObjectListToRequests(gardenList)
}

// // MapToExtension returns reconcile.Request objects for the extension that own a particular managed resource.
// func (r *Reconciler) MapToExtension(ctx context.Context, log logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request {
// 	if _, ok := obj.GetLabels()[ExtensionNameLabel]; !ok {
// 		return nil
// 	}
//
// 	ext := &operatorv1alpha1.Extension{}
// 	err := reader.Get(ctx, types.NamespacedName{Name: obj.GetLabels()[ExtensionNameLabel]}, ext)
// 	if err != nil {
// 		log.Error(err, "Failed to get required extension")
// 		return nil
// 	}
//
// 	return []reconcile.Request{
// 		{types.NamespacedName{Name: obj.GetLabels()[ExtensionNameLabel]}},
// 	}
// }
