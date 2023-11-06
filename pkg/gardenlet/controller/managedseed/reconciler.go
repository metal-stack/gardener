// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package managedseed

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controller/gardenletdeployer"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler reconciles the ManagedSeed.
type Reconciler struct {
	GardenConfig          *rest.Config
	GardenAPIReader       client.Reader
	GardenClient          client.Client
	SeedClient            client.Client
	Config                config.GardenletConfiguration
	Clock                 clock.Clock
	Recorder              record.EventRecorder
	ShootClientMap        clientmap.ClientMap
	GardenNamespaceGarden string
	GardenNamespaceShoot  string
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, r.Config.Controllers.ManagedSeed.SyncPeriod.Duration)
	defer cancel()

	ms := &seedmanagementv1alpha1.ManagedSeed{}
	if err := r.GardenClient.Get(ctx, request.NamespacedName, ms); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if ms.DeletionTimestamp != nil {
		return r.delete(ctx, log, ms)
	}
	return r.reconcile(ctx, log, ms)
}

func (r *Reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	ms *seedmanagementv1alpha1.ManagedSeed,
) (
	result reconcile.Result,
	err error,
) {
	// Ensure gardener finalizer
	if !controllerutil.ContainsFinalizer(ms, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.GardenClient, ms, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	var status *seedmanagementv1alpha1.ManagedSeedStatus
	defer func() {
		// Update status, on failure return the update error unless there is another error
		if updateErr := r.updateStatus(ctx, ms, status); updateErr != nil && err == nil {
			err = fmt.Errorf("could not update status: %w", updateErr)
		}
	}()

	// Get shoot
	shoot := &gardencorev1beta1.Shoot{}
	if err := r.GardenAPIReader.Get(ctx, kubernetesutils.Key(ms.Namespace, ms.Spec.Shoot.Name), shoot); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not get shoot %s/%s: %w", ms.Namespace, ms.Spec.Shoot.Name, err)
	}

	log = log.WithValues("shootName", shoot.Name)

	// Check if shoot is reconciled and update ShootReconciled condition
	if !shootReconciled(shoot) {
		log.Info("Waiting for shoot to be reconciled")

		msg := fmt.Sprintf("Waiting for shoot %q to be reconciled", client.ObjectKeyFromObject(shoot).String())
		r.Recorder.Event(ms, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, msg)
		updateCondition(r.Clock, status, seedmanagementv1alpha1.ManagedSeedShootReconciled, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconciling, msg)

		return reconcile.Result{RequeueAfter: r.Config.Controllers.ManagedSeed.WaitSyncPeriod.Duration}, nil
	}
	updateCondition(r.Clock, status, seedmanagementv1alpha1.ManagedSeedShootReconciled, gardencorev1beta1.ConditionTrue, gardencorev1beta1.EventReconciled,
		fmt.Sprintf("Shoot %q has been reconciled", client.ObjectKeyFromObject(shoot).String()))

	// Reconcile creation or update
	log.V(1).Info("Reconciling")
	if err := r.newActuator(shoot).Reconcile(ctx, log, ms, ms.Status.Conditions, ms.Spec.Gardenlet); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not reconcile ManagedSeed %s creation or update: %w", kubernetesutils.ObjectName(ms), err)
	}
	log.V(1).Info("Reconciliation finished")

	// Return success result
	return reconcile.Result{RequeueAfter: r.Config.Controllers.ManagedSeed.SyncPeriod.Duration}, nil
}

func (r *Reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	ms *seedmanagementv1alpha1.ManagedSeed,
) (
	result reconcile.Result,
	err error,
) {
	// Check gardener finalizer
	if !controllerutil.ContainsFinalizer(ms, gardencorev1beta1.GardenerName) {
		log.V(1).Info("Skipping deletion as object does not have a finalizer")
		return reconcile.Result{}, nil
	}

	var status *seedmanagementv1alpha1.ManagedSeedStatus
	var wait, removeFinalizer bool
	defer func() {
		// Only update status if the finalizer is not removed to prevent errors if the object is already gone
		if !removeFinalizer {
			// Update status, on failure return the update error unless there is another error
			if updateErr := r.updateStatus(ctx, ms, status); updateErr != nil && err == nil {
				err = fmt.Errorf("could not update status: %w", updateErr)
			}
		}
	}()

	// Get shoot
	shoot := &gardencorev1beta1.Shoot{}
	if err := r.GardenAPIReader.Get(ctx, kubernetesutils.Key(ms.Namespace, ms.Spec.Shoot.Name), shoot); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not get shoot %s/%s: %w", ms.Namespace, ms.Spec.Shoot.Name, err)
	}

	// Reconcile deletion
	log.V(1).Info("Deletion")
	if wait, removeFinalizer, err = r.newActuator(shoot).Delete(ctx, log, ms, ms.Status.Conditions, ms.Spec.Gardenlet); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not reconcile ManagedSeed %s deletion: %w", kubernetesutils.ObjectName(ms), err)
	}
	log.V(1).Info("Deletion finished")

	// If waiting, requeue after WaitSyncPeriod
	if wait {
		return reconcile.Result{RequeueAfter: r.Config.Controllers.ManagedSeed.WaitSyncPeriod.Duration}, nil
	}

	// Remove gardener finalizer if requested by the actuator
	if removeFinalizer {
		if controllerutil.ContainsFinalizer(ms, gardencorev1beta1.GardenerName) {
			log.Info("Removing finalizer")
			if err := controllerutils.RemoveFinalizers(ctx, r.GardenClient, ms, gardencorev1beta1.GardenerName); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
			}
		}
		return reconcile.Result{}, nil
	}

	// Return success result
	return reconcile.Result{RequeueAfter: r.Config.Controllers.ManagedSeed.SyncPeriod.Duration}, nil
}

func (r *Reconciler) newActuator(shoot *gardencorev1beta1.Shoot) gardenletdeployer.Interface {
	return &gardenletdeployer.Actuator{
		GardenConfig:    r.GardenConfig,
		GardenAPIReader: r.GardenAPIReader,
		GardenClient:    r.GardenClient,
		GetTargetClientFunc: func(ctx context.Context) (kubernetes.Interface, error) {
			return r.ShootClientMap.GetClient(ctx, keys.ForShoot(shoot))
		},
		CheckIfVPAAlreadyExists: func(ctx context.Context) (bool, error) {
			if err := r.SeedClient.Get(ctx, kubernetesutils.Key(shoot.Status.TechnicalID, "vpa-admission-controller"), &appsv1.Deployment{}); err != nil {
				if apierrors.IsNotFound(err) {
					return false, nil
				}
				return false, err
			}
			return true, nil
		},
		GetKubeconfigSecret: func(ctx context.Context) (*corev1.Secret, error) {
			if !pointer.BoolDeref(shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig, false) {
				return nil, nil
			}

			secret := &corev1.Secret{}
			if err := r.GardenClient.Get(ctx, kubernetesutils.Key(shoot.Namespace, gardenerutils.ComputeShootProjectSecretName(shoot.Name, gardenerutils.ShootProjectSecretSuffixKubeconfig)), secret); err != nil {
				return nil, err
			}
			return secret, nil
		},
		GetInfrastructureSecret: func(ctx context.Context) (*corev1.Secret, error) {
			secretBinding := &gardencorev1beta1.SecretBinding{}
			if shoot.Spec.SecretBindingName == nil {
				return nil, fmt.Errorf("secretbinding name is nil for the Shoot: %s/%s", shoot.Namespace, shoot.Name)
			}
			if err := r.GardenClient.Get(ctx, kubernetesutils.Key(shoot.Namespace, *shoot.Spec.SecretBindingName), secretBinding); err != nil {
				return nil, err
			}
			return kubernetesutils.GetSecretByReference(ctx, r.GardenClient, &secretBinding.SecretRef)
		},
		GetTargetDomain: func() string {
			if shoot.Spec.DNS != nil && shoot.Spec.DNS.Domain != nil {
				return *shoot.Spec.DNS.Domain
			}
			return ""
		},
		Clock:                 r.Clock,
		ValuesHelper:          gardenletdeployer.NewValuesHelper(&r.Config),
		Recorder:              r.Recorder,
		GardenNamespaceTarget: r.GardenNamespaceShoot,
	}
}

func (r *Reconciler) updateStatus(ctx context.Context, ms *seedmanagementv1alpha1.ManagedSeed, status *seedmanagementv1alpha1.ManagedSeedStatus) error {
	if status == nil {
		return nil
	}
	patch := client.StrategicMergeFrom(ms.DeepCopy())
	ms.Status = *status
	return r.GardenClient.Status().Patch(ctx, ms, patch)
}

func shootReconciled(shoot *gardencorev1beta1.Shoot) bool {
	lastOp := shoot.Status.LastOperation
	return shoot.Generation == shoot.Status.ObservedGeneration && lastOp != nil && lastOp.State == gardencorev1beta1.LastOperationStateSucceeded
}

func updateCondition(clock clock.Clock, status *seedmanagementv1alpha1.ManagedSeedStatus, ct gardencorev1beta1.ConditionType, cs gardencorev1beta1.ConditionStatus, reason, message string) {
	condition := v1beta1helper.GetOrInitConditionWithClock(clock, status.Conditions, ct)
	condition = v1beta1helper.UpdatedConditionWithClock(clock, condition, cs, reason, message)
	status.Conditions = v1beta1helper.MergeConditions(status.Conditions, condition)
}
