// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenerconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	requeueUnhealthyVirtualKubeApiserver = 10 * time.Second
	ConditionReconcileFailed             = "ReconcileFailed"
	ConditionDeleteFailed                = "DeleteFailed"
	ConditionReconcileSuccess            = "ReconcileSuccessful"
)

// Reconciler reconciles Gardens.
type Reconciler struct {
	RuntimeClientSet kubernetes.Interface
	RuntimeVersion   *semver.Version
	Config           config.OperatorConfiguration
	Clock            clock.Clock
	Recorder         record.EventRecorder
	GardenClientMap  clientmap.ClientMap
	GardenNamespace  string
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	extension := &operatorv1alpha1.Extension{}
	if err := r.RuntimeClientSet.Client().Get(ctx, request.NamespacedName, extension); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	gardenList := &operatorv1alpha1.GardenList{}
	// We limit one result because we expect only a single Garden object to be there.
	if err := r.RuntimeClientSet.Client().List(ctx, gardenList, client.Limit(1)); err != nil {
		return reconcile.Result{}, fmt.Errorf("error retrieving garden object: %w", err)
	}
	if len(gardenList.Items) == 0 {
		return reconcile.Result{}, fmt.Errorf("error: garden object not found")
	}

	garden := &gardenList.Items[0]
	virtualKubeAPIServerCond := v1beta1helper.GetOrInitConditionWithClock(r.Clock, garden.Status.Conditions, operatorv1alpha1.VirtualGardenAPIServerAvailable)
	if virtualKubeAPIServerCond.Status != gardencorev1beta1.ConditionTrue {
		log.Info("virtual garden is not ready yet. Requeue...")
		return reconcile.Result{}, &reconciler.RequeueAfterError{
			RequeueAfter: requeueUnhealthyVirtualKubeApiserver,
		}
	}

	gardenClientSet, err := r.GardenClientMap.GetClient(ctx, keys.ForGarden(garden))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error retrieving garden client object: %w", err)
	}

	// init relevant condition list
	conditions := NewGardenConfigConditions(r.Clock, extension.Status)
	var (
		updatedConditions GardenerConfigConditions
	)
	if extension.DeletionTimestamp != nil {
		updatedConditions, err = r.delete(ctx, log, gardenClientSet.Client(), extension, conditions)
	} else {
		updatedConditions, err = r.reconcile(ctx, log, gardenClientSet.Client(), extension, conditions)
	}

	if v1beta1helper.ConditionsNeedUpdate(conditions.ConvertToSlice(), updatedConditions.ConvertToSlice()) {
		log.Info("Updating extension status conditions")
		patch := client.MergeFrom(extension.DeepCopy())
		// Rebuild garden conditions to ensure that only the conditions with the
		// correct types will be updated, and any other conditions will remain intact
		extension.Status.Conditions = v1beta1helper.BuildConditions(extension.Status.Conditions, updatedConditions.ConvertToSlice(), conditions.ConditionTypes())

		if err := r.RuntimeClientSet.Client().Status().Patch(ctx, extension, patch); err != nil {
			log.Error(err, "Could not update extension status")
			return reconcile.Result{}, fmt.Errorf("failed to update extension status", err)
		}
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) reconcile(ctx context.Context, log logr.Logger, gardenClient client.Client, extension *operatorv1alpha1.Extension, conditions GardenerConfigConditions) (GardenerConfigConditions, error) {
	if extension.Spec.Deployment == nil {
		return conditions, nil
	}
	if extension.Spec.Deployment.Extension == nil {
		return conditions, nil
	}

	if extension.Spec.Deployment.Extension.Helm == nil {
		return conditions, nil
	}

	log.Info("Adding finalizer")
	if err := controllerutils.AddFinalizers(ctx, r.RuntimeClientSet.Client(), extension, operatorv1alpha1.FinalizerName); err != nil {
		err := fmt.Errorf("failed to add finalizer: %w", err)
		conditions.gardenConfigReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.gardenConfigReconciled, gardencorev1beta1.ConditionFalse, ConditionReconcileFailed, err.Error())
		return conditions, err
	}

	if err := r.reconcileControllerDeployment(ctx, gardenClient, extension); err != nil {
		err := fmt.Errorf("failed to reconciler controller deployment: %w", err)
		conditions.gardenConfigReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.gardenConfigReconciled, gardencorev1beta1.ConditionFalse, ConditionReconcileFailed, err.Error())
		return conditions, err
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "ControllerDeployment created successfully")

	if err := r.reconcileControllerRegistration(ctx, gardenClient, extension); err != nil {
		err := fmt.Errorf("failed to reconciler controller registration: %w", err)
		conditions.gardenConfigReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.gardenConfigReconciled, gardencorev1beta1.ConditionFalse, ConditionReconcileFailed, err.Error())
		return conditions, err
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "ControllerRegistration created successfully")
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "Successfully reconciled")

	conditions.gardenConfigReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.gardenConfigReconciled, gardencorev1beta1.ConditionTrue, ConditionReconcileSuccess, fmt.Sprintf("Extension %q has been reconciled successfully", extension.Name))
	return conditions, nil
}

func HelmDeployer(helm *operatorv1alpha1.Helm) (*runtime.RawExtension, error) {
	var helmDeployment struct {
		// chart is a Helm chart tarball.
		Chart []byte `json:"chart,omitempty"`
		// Values is a map of values for the given chart.
		Values map[string]interface{} `json:"values,omitempty"`
	}

	if rawChart := helm.RawChart; rawChart != nil {
		helmDeployment.Chart = rawChart
	}
	if values := helm.Values; values != nil {
		if err := json.Unmarshal(values.Raw, helm.Values); err != nil {
			return nil, err
		}
	}

	rawHelm, err := json.Marshal(helm)
	if err != nil {
		return nil, err
	}

	return &runtime.RawExtension{
		Raw: rawHelm,
	}, nil
}

func (r *Reconciler) reconcileControllerDeployment(ctx context.Context, gardenClient client.Client, extension *operatorv1alpha1.Extension) error {
	ctrlDeploy := &gardencorev1beta1.ControllerDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: extension.Name,
		},
	}

	deployMutateFn := func() error {
		ctrlDeploy.Annotations = extension.Spec.Deployment.Extension.Annotations
		ctrlDeploy.Type = "helm"

		var (
			rawHelm *runtime.RawExtension
			err     error
		)
		if helm := extension.Spec.Deployment.Extension.Helm; helm.RawChart != nil {
			rawHelm, err = HelmDeployer(helm)
			if err != nil {
				return err
			}

		}

		ctrlDeploy.ProviderConfig = *rawHelm
		return nil
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, gardenClient, ctrlDeploy, deployMutateFn); err != nil {
		return fmt.Errorf("failed to create or update ControllerInstallation: %w", err)
	}
	return nil
}

func (r *Reconciler) reconcileControllerRegistration(ctx context.Context, gardenClient client.Client, extension *operatorv1alpha1.Extension) error {
	ctrlReg := &gardencorev1beta1.ControllerRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: extension.Name,
		},
	}
	regMutateFn := func() error {
		ctrlReg.Annotations = extension.Spec.Deployment.Extension.Annotations
		ctrlReg.Spec = gardencorev1beta1.ControllerRegistrationSpec{
			Resources: extension.Spec.Resources,
			Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
				Policy: extension.Spec.Deployment.Extension.Policy,
				DeploymentRefs: []gardencorev1beta1.DeploymentRef{
					{
						Name: extension.Name,
					},
				},
			},
		}
		return nil
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, gardenClient, ctrlReg, regMutateFn); err != nil {
		return fmt.Errorf("failed to create or update ControllerRegistration: %w", err)
	}
	return nil
}

func (r *Reconciler) delete(ctx context.Context, log logr.Logger, gardenClient client.Client, extension *operatorv1alpha1.Extension, conditions GardenerConfigConditions) (GardenerConfigConditions, error) {
	log.Info("Deleting extension", "name", extension.Name)
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Deletion", "Deleting extension")
	var (
		ctrlDeploy = &gardencorev1beta1.ControllerDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: extension.Name,
			}}

		ctrlReg = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: extension.Name,
			},
		}
	)

	log.Info("Deleting controller deployment for extension", "extension", extension.Name)
	if err := kubernetesutils.DeleteObject(ctx, gardenClient, ctrlReg); err != nil {
		conditions.gardenConfigReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.gardenConfigReconciled, gardencorev1beta1.ConditionFalse, ConditionDeleteFailed, err.Error())
		return conditions, err
	}
	log.Info("Deleting controller registration for extension", "extension", extension.Name)
	if err := kubernetesutils.DeleteObject(ctx, gardenClient, ctrlDeploy); err != nil {
		conditions.gardenConfigReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.gardenConfigReconciled, gardencorev1beta1.ConditionFalse, ConditionDeleteFailed, err.Error())
		return conditions, err
	}

	log.Info("Waiting until controller registration is gone", "extension", extension.Name)
	if err := kubernetesutils.WaitUntilResourceDeleted(ctx, gardenClient, ctrlReg, 5*time.Second); err != nil {
		conditions.gardenConfigReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.gardenConfigReconciled, gardencorev1beta1.ConditionFalse, ConditionDeleteFailed, err.Error())
		return conditions, err
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Deletion", "Successfully deleted controller registration")

	log.Info("Waiting until controller deployment is gone", "extension", extension.Name)
	if err := kubernetesutils.WaitUntilResourceDeleted(ctx, gardenClient, ctrlDeploy, 5*time.Second); err != nil {
		conditions.gardenConfigReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.gardenConfigReconciled, gardencorev1beta1.ConditionFalse, ConditionDeleteFailed, err.Error())
		return conditions, err
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Deletion", "Successfully deleted controller deployment")

	log.Info("Removing finalizer")
	if err := controllerutils.RemoveFinalizers(ctx, r.RuntimeClientSet.Client(), extension, operatorv1alpha1.FinalizerName); err != nil {
		err := fmt.Errorf("failed to add finalizer: %w", err)
		conditions.gardenConfigReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.gardenConfigReconciled, gardencorev1beta1.ConditionFalse, ConditionDeleteFailed, err.Error())
		return conditions, err
	}

	return conditions, nil
}

// GardenerConfigConditions contains all conditions of the garden status subresource.
type GardenerConfigConditions struct {
	gardenConfigReconciled gardencorev1beta1.Condition
}

// ConvertToSlice returns the garden conditions as a slice.
func (g GardenerConfigConditions) ConvertToSlice() []gardencorev1beta1.Condition {
	return []gardencorev1beta1.Condition{
		g.gardenConfigReconciled,
	}
}

// ConditionTypes returns all garden condition types.
func (g GardenerConfigConditions) ConditionTypes() []gardencorev1beta1.ConditionType {
	return []gardencorev1beta1.ConditionType{
		g.gardenConfigReconciled.Type,
	}
}

// NewGardenConfigConditions returns a new instance of GardenerConfigConditions.
// All conditions are retrieved from the given 'status' or newly initialized.
func NewGardenConfigConditions(clock clock.Clock, status operatorv1alpha1.ExtensionStatus) GardenerConfigConditions {
	return GardenerConfigConditions{
		gardenConfigReconciled: v1beta1helper.GetOrInitConditionWithClock(clock, status.Conditions, operatorv1alpha1.GardenConfigReconciled),
	}
}
