// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet

import (
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/controller/managedseed"
	"github.com/gardener/gardener/pkg/utils/oci"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "gardenlet"

	// GardenletDefaultKubeconfigSecretName is the default name for the field in the Gardenlet component configuration
	// .gardenClientConnection.KubeconfigSecret.Name
	GardenletDefaultKubeconfigSecretName = "gardenlet-kubeconfig"
	// GardenletDefaultKubeconfigBootstrapSecretName is the default name for the field in the Gardenlet component configuration
	// .gardenClientConnection.BootstrapKubeconfig.Name
	GardenletDefaultKubeconfigBootstrapSecretName = "gardenlet-kubeconfig-bootstrap"
)

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(
	mgr manager.Manager,
	gardenCluster cluster.Cluster,
	seedClientSet kubernetes.Interface,
) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}

	if r.Actuator == nil {
		helmRegistry, err := oci.NewHelmRegistry()
		if err != nil {
			return err
		}

		r.Actuator = newActuator(
			gardenCluster.GetConfig(),
			gardenCluster.GetClient(),
			seedClientSet,
			r.Clock,
			managedseed.NewValuesHelper(&r.Config),
			gardenCluster.GetEventRecorderFor(ControllerName+"-controller"),
			helmRegistry,
		)
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			// TODO: introduce dedicated config for Gardenlet controller later
			MaxConcurrentReconciles: ptr.Deref(r.Config.Controllers.ManagedSeed.ConcurrentSyncs, 0),
		}).
		WatchesRawSource(
			source.Kind(gardenCluster.GetCache(), &seedmanagementv1alpha1.Gardenlet{}),
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Complete(r)
}
