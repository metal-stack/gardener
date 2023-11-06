// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controller/gardenletdeployer"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler reconciles the gardenlet.
type Reconciler struct {
	RuntimeCluster        cluster.Cluster
	RuntimeClient         client.Client
	VirtualConfig         *rest.Config
	VirtualAPIReader      client.Reader
	VirtualClient         client.Client
	Config                config.GardenControllerConfig // TODO
	Clock                 clock.Clock
	Recorder              record.EventRecorder
	GardenNamespaceTarget string
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, r.Config.SyncPeriod.Duration)
	defer cancel()

	gardenlet := &operatorv1alpha1.Gardenlet{}
	if err := r.RuntimeClient.Get(ctx, request.NamespacedName, gardenlet); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if gardenlet.DeletionTimestamp != nil {
		return r.delete(ctx, log, gardenlet)
	}
	return r.reconcile(ctx, log, gardenlet)
}

func (r *Reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	gardenlet *operatorv1alpha1.Gardenlet,
) (
	result reconcile.Result,
	err error,
) {
	// Ensure gardener finalizer
	if !controllerutil.ContainsFinalizer(gardenlet, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.RuntimeClient, gardenlet, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	var status *operatorv1alpha1.GardenletStatus
	defer func() {
		// Update status, on failure return the update error unless there is another error
		if updateErr := r.updateStatus(ctx, gardenlet, status); updateErr != nil && err == nil {
			err = fmt.Errorf("could not update status: %w", updateErr)
		}
	}()

	// Reconcile creation or update
	log.V(1).Info("Reconciling")
	if err := r.newActuator(gardenlet.Spec).Reconcile(ctx, log, gardenlet, gardenlet.Status.Conditions, gardenlet.Spec.Gardenlet); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not reconcile Gardenlet %s creation or update: %w", kubernetesutils.ObjectName(gardenlet), err)
	}
	log.V(1).Info("Reconciliation finished")

	// Return success result
	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}

func (r *Reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	gardenlet *operatorv1alpha1.Gardenlet,
) (
	result reconcile.Result,
	err error,
) {
	// Check gardener finalizer
	if !controllerutil.ContainsFinalizer(gardenlet, gardencorev1beta1.GardenerName) {
		log.V(1).Info("Skipping deletion as object does not have a finalizer")
		return reconcile.Result{}, nil
	}

	var status *operatorv1alpha1.GardenletStatus
	var wait, removeFinalizer bool
	defer func() {
		// Only update status if the finalizer is not removed to prevent errors if the object is already gone
		if !removeFinalizer {
			// Update status, on failure return the update error unless there is another error
			if updateErr := r.updateStatus(ctx, gardenlet, status); updateErr != nil && err == nil {
				err = fmt.Errorf("could not update status: %w", updateErr)
			}
		}
	}()

	// Reconcile deletion
	log.V(1).Info("Deletion")
	if wait, removeFinalizer, err = r.newActuator(gardenlet.Spec).Delete(ctx, log, gardenlet, gardenlet.Status.Conditions, gardenlet.Spec.Gardenlet); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not reconcile Gardenlet %s deletion: %w", kubernetesutils.ObjectName(gardenlet), err)
	}
	log.V(1).Info("Deletion finished")

	// If waiting, requeue after WaitSyncPeriod
	if wait {
		return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil // TODO: r.Config.WaitSyncPeriod.Duration
	}

	// Remove gardener finalizer if requested by the actuator
	if removeFinalizer {
		if controllerutil.ContainsFinalizer(gardenlet, gardencorev1beta1.GardenerName) {
			log.Info("Removing finalizer")
			if err := controllerutils.RemoveFinalizers(ctx, r.RuntimeClient, gardenlet, gardencorev1beta1.GardenerName); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
			}
		}
		return reconcile.Result{}, nil
	}

	// Return success result
	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}

func (r *Reconciler) newActuator(gardenletSpec operatorv1alpha1.GardenletSpec) gardenletdeployer.Interface {
	return &gardenletdeployer.Actuator{
		GardenConfig:    r.VirtualConfig,
		GardenAPIReader: r.VirtualAPIReader,
		GardenClient:    r.VirtualClient,
		GetTargetClientFunc: func(ctx context.Context) (kubernetes.Interface, error) {
			if gardenletSpec.KubeconfigSecretRef == nil {
				return kubernetes.NewWithConfig(
					kubernetes.WithRESTConfig(r.RuntimeCluster.GetConfig()),
					kubernetes.WithRuntimeAPIReader(r.RuntimeCluster.GetAPIReader()),
					kubernetes.WithRuntimeClient(r.RuntimeCluster.GetClient()),
					kubernetes.WithRuntimeCache(r.RuntimeCluster.GetCache()),
				)
			}
			return kubernetes.NewClientFromSecret(ctx, r.RuntimeClient, gardenletSpec.KubeconfigSecretRef.Namespace, gardenletSpec.KubeconfigSecretRef.Name)
		},
		CheckIfVPAAlreadyExists: func(ctx context.Context) (bool, error) {
			return false, nil
		},
		GetKubeconfigSecret: func(ctx context.Context) (*corev1.Secret, error) {
			if gardenletSpec.KubeconfigSecretRef == nil {
				return nil, nil
			}
			return kubernetesutils.GetSecretByReference(ctx, r.RuntimeClient, gardenletSpec.KubeconfigSecretRef)
		},
		GetInfrastructureSecret: func(ctx context.Context) (*corev1.Secret, error) {
			if gardenletSpec.Backup == nil {
				return nil, nil
			}
			return kubernetesutils.GetSecretByReference(ctx, r.RuntimeClient, &gardenletSpec.Backup.SecretRef)
		},
		GetTargetDomain: func() string {
			return ""
		},
		Clock:                 r.Clock,
		ValuesHelper:          gardenletdeployer.NewValuesHelper(nil),
		Recorder:              r.Recorder,
		GardenNamespaceTarget: r.GardenNamespaceTarget,
	}
}

func (r *Reconciler) updateStatus(ctx context.Context, gardenlet *operatorv1alpha1.Gardenlet, status *operatorv1alpha1.GardenletStatus) error {
	if status == nil {
		return nil
	}
	patch := client.StrategicMergeFrom(gardenlet.DeepCopy())
	gardenlet.Status = *status
	return r.VirtualClient.Status().Patch(ctx, gardenlet, patch)
}
