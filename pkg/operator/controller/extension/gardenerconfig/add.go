// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenerconfig

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	clientmapbuilder "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/builder"
)

// ControllerName is the name of this controller.
const ControllerName = "extensions-gardener-config"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
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
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(ControllerName + "-controller")
	}
	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}
	if r.GardenClientMap == nil {
		var err error
		r.GardenClientMap, err = clientmapbuilder.
			NewGardenClientMapBuilder().
			WithRuntimeClient(mgr.GetClient()).
			WithClientConnectionConfig(&r.Config.VirtualClientConnection).
			WithGardenNamespace(r.GardenNamespace).
			Build(mgr.GetLogger())
		if err != nil {
			return fmt.Errorf("failed to build garden ClientMap: %w", err)
		}
		if err := mgr.Add(r.GardenClientMap); err != nil {
			return err
		}
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&operatorv1alpha1.Extension{}).
		// For(&operatorv1alpha1.Extension{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.Controllers.ExtensionGardenConfig.ConcurrentSyncs, 0),
		}).
		Complete(r)
}
