// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package required

import (
	"context"
	"fmt"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	"github.com/gardener/gardener/pkg/operator"
)

// ControllerName is the name of this controller.
const (
	ControllerName = "extensions-required"

	// ExtensionRequired is a condition type for indicating that the respective extension controller is
	// still required on the seed cluster as corresponding extension resources still exist.
	ExtensionRequired gardencorev1beta1.ConditionType = "Required"
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
	r.Lock = &sync.RWMutex{}
	r.KindToRequiredTypes = make(map[string]sets.Set[string])

	// It's not possible to call builder.Build() without adding atleast one watch, and without this, we can't get the controller logger.
	// Hence, we have to build up the controller manually.
	c, err := controller.New(
		ControllerName,
		mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: ptr.Deref(r.Config.Controllers.ExtensionRequiredConfig.ConcurrentSyncs, 0),
		},
	)
	if err != nil {
		return err
	}

	for _, obj := range []struct {
		objectKind        string
		object            client.Object
		newObjectListFunc func() client.ObjectList
	}{
		{extensionsv1alpha1.BackupBucketResource, &extensionsv1alpha1.BackupBucket{}, func() client.ObjectList { return &extensionsv1alpha1.BackupBucketList{} }},
		{extensionsv1alpha1.DNSRecordResource, &extensionsv1alpha1.DNSRecord{}, func() client.ObjectList { return &extensionsv1alpha1.DNSRecordList{} }},
	} {
		eventHandler := mapper.EnqueueRequestsFrom(
			ctx,
			mgr.GetCache(),
			r.MapObjectKindToExtensions(obj.objectKind, obj.newObjectListFunc),
			mapper.UpdateWithNew,
			c.GetLogger(),
		)

		// Execute the mapper function at least once to initialize the `KindToRequiredTypes` map.
		// This is necessary for extension kinds which are registered but for which no extension objects exist in the
		// seed (e.g. when backups are disabled). In such cases, no regular watch event would be triggered. Hence, the
		// mapping function would never be executed. Hence, the extension kind would never be part of the
		// `KindToRequiredTypes` map. Hence, the reconciler would not be able to decide whether the
		// Extensions is required.
		if err = c.Watch(controllerutils.HandleOnce, eventHandler); err != nil {
			return err
		}

		if err := c.Watch(source.Kind(r.RuntimeClientSet.Cache(), obj.object), eventHandler, r.ObjectPredicate()); err != nil {
			return err
		}
	}

	return nil
}

// ObjectPredicate returns true for 'create' and 'update' events. For updates, it only returns true when the extension
// type has changed.
func (r *Reconciler) ObjectPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// enqueue on periodic cache resyncs
			if e.ObjectOld.GetResourceVersion() == e.ObjectNew.GetResourceVersion() {
				return true
			}

			extensionObj, ok := e.ObjectNew.(extensionsv1alpha1.Object)
			if !ok {
				return false
			}

			oldExtensionObj, ok := e.ObjectOld.(extensionsv1alpha1.Object)
			if !ok {
				return false
			}

			return oldExtensionObj.GetExtensionSpec().GetExtensionType() != extensionObj.GetExtensionSpec().GetExtensionType()
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

// MapObjectKindToExtensions returns a mapper function for the given extension kind that lists all existing
// extension resources of the given kind and stores the respective types in the `KindToRequiredTypes` map. Afterwards,
// it enqueues all Extensions responsible for the given kind.
// The returned reconciler doesn't care about which object was created/updated/deleted, it just cares about being
// triggered when some object of the kind, it is responsible for, is created/updated/deleted.
func (r *Reconciler) MapObjectKindToExtensions(objectKind string, newObjectListFunc func() client.ObjectList) mapper.MapFunc {
	return func(ctx context.Context, log logr.Logger, _ client.Reader, _ client.Object) []reconcile.Request {
		log = log.WithValues("extensionKind", objectKind)

		listObj := newObjectListFunc()
		if err := r.RuntimeClientSet.Client().List(ctx, listObj); err != nil && !meta.IsNoMatchError(err) {
			// Let's ignore bootstrap situations where extension CRDs were not yet applied. They will be deployed
			// eventually by the seed controller.
			log.Error(err, "Failed to list extension objects")
			return nil
		}

		r.Lock.RLock()
		oldRequiredTypes, kindCalculated := r.KindToRequiredTypes[objectKind]
		r.Lock.RUnlock()
		newRequiredTypes := sets.New[string]()

		if err := meta.EachListItem(listObj, func(o runtime.Object) error {
			obj, err := extensions.Accessor(o)
			if err != nil {
				return err
			}

			newRequiredTypes.Insert(obj.GetExtensionSpec().GetExtensionType())
			return nil
		}); err != nil {
			log.Error(err, "Failed while iterating over extension objects")
			return nil
		}

		// if there is no difference compared to before then exit early
		if kindCalculated && oldRequiredTypes.Equal(newRequiredTypes) {
			return nil
		}

		r.Lock.Lock()
		r.KindToRequiredTypes[objectKind] = newRequiredTypes
		r.Lock.Unlock()

		// Step 2: List all existing extensions and filter for those that are supporting resources for the
		// extension kind this particular reconciler is responsible for.

		extensionList := &operatorv1alpha1.ExtensionList{}
		if err := r.RuntimeClientSet.Client().List(ctx, extensionList); err != nil {
			log.Error(err, "Failed to list Extensions")
			return nil
		}
		for i, ext := range extensionList.Items {
			mergedSpec, err := operator.MergeExtensionSpecs(ext.Name, ext.Spec)
			if err != nil {
				return nil
			}
			extensionList.Items[i].Spec = mergedSpec
		}

		extensionNamesForKind := sets.New[string]()
		for _, ext := range extensionList.Items {
			for _, resource := range ext.Spec.Resources {
				if resource.Kind == objectKind {
					extensionNamesForKind.Insert(ext.Name)
					break
				}
			}
		}

		// Step 3: Filter for those that reference extensions collected above. Then requeue those extensions for
		// the other reconciler to decide whether it is required or not.

		var requests []reconcile.Request

		for _, ext := range extensionList.Items {
			if !extensionNamesForKind.Has(ext.Name) {
				continue
			}

			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: ext.Name}})
		}

		return requests
	}
}
