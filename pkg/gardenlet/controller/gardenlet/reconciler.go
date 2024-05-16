// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler reconciles the Gardenlet.
type Reconciler struct {
	GardenClient    client.Client
	Actuator        Actuator
	Config          config.GardenletConfiguration
	Clock           clock.Clock
	GardenNamespace string
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, r.Config.Controllers.ManagedSeed.SyncPeriod.Duration)
	defer cancel()

	gardenlet := &seedmanagementv1alpha1.Gardenlet{}
	if err := r.GardenClient.Get(ctx, request.NamespacedName, gardenlet); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	return r.reconcile(ctx, log, gardenlet)
}

func (r *Reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	gardenlet *seedmanagementv1alpha1.Gardenlet,
) (
	result reconcile.Result,
	err error,
) {
	var status *seedmanagementv1alpha1.GardenletStatus
	defer func() {
		// Update status, on failure return the update error unless there is another error
		if updateErr := r.updateStatus(ctx, gardenlet, status); updateErr != nil && err == nil {
			err = fmt.Errorf("could not update status: %w", updateErr)
		}
	}()

	// Reconcile creation or update
	log.V(1).Info("Reconciling")
	var wait bool
	if status, wait, err = r.Actuator.Reconcile(ctx, log, gardenlet); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not reconcile Gardenlet %s creation or update: %w", kubernetesutils.ObjectName(gardenlet), err)
	}
	log.V(1).Info("Reconciliation finished")

	// If waiting, requeue after WaitSyncPeriod
	if wait {
		return reconcile.Result{RequeueAfter: r.Config.Controllers.ManagedSeed.WaitSyncPeriod.Duration}, nil
	}

	// Return success result
	return reconcile.Result{RequeueAfter: r.Config.Controllers.ManagedSeed.SyncPeriod.Duration}, nil
}

func (r *Reconciler) updateStatus(ctx context.Context, gardenlet *seedmanagementv1alpha1.Gardenlet, status *seedmanagementv1alpha1.GardenletStatus) error {
	if status == nil {
		return nil
	}
	patch := client.StrategicMergeFrom(gardenlet.DeepCopy())
	gardenlet.Status = *status
	return r.GardenClient.Status().Patch(ctx, gardenlet, patch)
}
