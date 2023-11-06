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

package gardenletdeployer

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	seedmanagementv1alpha1constants "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	// GardenletDefaultKubeconfigSecretName is the default name for the field in the Gardenlet component configuration
	// .gardenClientConnection.KubeconfigSecret.Name
	GardenletDefaultKubeconfigSecretName = "gardenlet-kubeconfig"
	// GardenletDefaultKubeconfigBootstrapSecretName is the default name for the field in the Gardenlet component configuration
	// .gardenClientConnection.BootstrapKubeconfig.Name
	GardenletDefaultKubeconfigBootstrapSecretName = "gardenlet-kubeconfig-bootstrap"
)

// Interface deploys gardenlets into target clusters.
type Interface interface {
	// Reconcile deploys or updates a gardenlet in a target cluster.
	Reconcile(context.Context, logr.Logger, client.Object, []gardencorev1beta1.Condition, *seedmanagementv1alpha1.Gardenlet) error
	// Delete deletes a gardenlet from a target cluster.
	Delete(context.Context, logr.Logger, client.Object, []gardencorev1beta1.Condition, *seedmanagementv1alpha1.Gardenlet) (bool, bool, error)
}

// Actuator is a concrete implementation of Interface.
type Actuator struct {
	GardenConfig            *rest.Config
	GardenAPIReader         client.Reader
	GardenClient            client.Client
	GetTargetClientFunc     func(ctx context.Context) (kubernetes.Interface, error)
	CheckIfVPAAlreadyExists func(ctx context.Context) (bool, error)
	GetKubeconfigSecret     func(ctx context.Context) (*corev1.Secret, error)
	GetInfrastructureSecret func(ctx context.Context) (*corev1.Secret, error)
	GetTargetDomain         func() string
	Clock                   clock.Clock
	ValuesHelper            ValuesHelper
	Recorder                record.EventRecorder
	GardenNamespaceTarget   string
}

// Reconcile deploys or updates gardenlets.
func (a *Actuator) Reconcile(
	ctx context.Context,
	log logr.Logger,
	obj client.Object,
	conditions []gardencorev1beta1.Condition,
	gardenlet *seedmanagementv1alpha1.Gardenlet,
) (
	err error,
) {
	defer func() {
		if err != nil {
			log.Error(err, "Error during reconciliation")
			a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, err.Error())
			updateCondition(a.Clock, conditions, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconcileError, err.Error())
		}
	}()

	// Get target client
	targetClient, err := a.GetTargetClientFunc(ctx)
	if err != nil {
		return fmt.Errorf("could not get target client: %w", err)
	}

	// Extract seed template and gardenlet config
	seedTemplate, gardenletConfig, err := helper.ExtractSeedTemplateAndGardenletConfig(gardenlet)
	if err != nil {
		return err
	}

	// Check seed spec
	if err := a.checkSeedSpec(ctx, &seedTemplate.Spec); err != nil {
		return err
	}

	// Create or update garden namespace in the target cluster
	log.Info("Ensuring garden namespace in target cluster")
	a.Recorder.Eventf(obj, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Ensuring garden namespace in target cluster")
	if err := a.ensureGardenNamespace(ctx, targetClient.Client()); err != nil {
		return fmt.Errorf("could not create or update garden namespace in target cluster: %w", err)
	}

	// Create or update seed secrets
	log.Info("Reconciling seed secrets")
	a.Recorder.Event(obj, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Reconciling seed secrets")
	if err := a.reconcileSeedSecrets(ctx, log, obj, &seedTemplate.Spec, gardenlet); err != nil {
		return fmt.Errorf("could not reconcile seed secrets: %w", err)
	}

	seed, err := a.getSeed(ctx, obj.GetName())
	if err != nil {
		return fmt.Errorf("could not read seed %s: %w", obj.GetName(), err)
	}

	// Deploy gardenlet into the target cluster, it will register the seed automatically
	log.Info("Deploying gardenlet into target cluster")
	a.Recorder.Eventf(obj, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Deploying gardenlet into target cluster")
	if err := a.deployGardenlet(ctx, log, obj, targetClient, gardenlet, seed, gardenletConfig); err != nil {
		return fmt.Errorf("could not deploy gardenlet into target cluster: %w", err)
	}

	updateCondition(a.Clock, conditions, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionTrue, gardencorev1beta1.EventReconciled,
		fmt.Sprintf("Seed %s has been registered", obj.GetName()))
	return nil
}

// Delete reconciles ManagedSeed deletion.
func (a *Actuator) Delete(
	ctx context.Context,
	log logr.Logger,
	obj client.Object,
	conditions []gardencorev1beta1.Condition,
	gardenlet *seedmanagementv1alpha1.Gardenlet,
) (
	wait, removeFinalizer bool,
	err error,
) {
	defer func() {
		if err != nil {
			log.Error(err, "Error during deletion")
			a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, err.Error())
			updateCondition(a.Clock, conditions, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleteError, err.Error())
		}
	}()

	// Update SeedRegistered condition
	updateCondition(a.Clock, conditions, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleting,
		fmt.Sprintf("Unregistering seed %s", obj.GetName()))

	// Get target client
	targetClient, err := a.GetTargetClientFunc(ctx)
	if err != nil {
		return false, false, fmt.Errorf("could not get target client: %w", err)
	}

	// Extract seed template and gardenlet config
	seedTemplate, gardenletConfig, err := helper.ExtractSeedTemplateAndGardenletConfig(gardenlet)
	if err != nil {
		return false, false, err
	}

	// Delete seed if it still exists and is not already deleting
	seed, err := a.getSeed(ctx, obj.GetName())
	if err != nil {
		return false, false, fmt.Errorf("could not get seed %s: %w", obj.GetName(), err)
	}

	if seed != nil {
		log = log.WithValues("seedName", seed.Name)

		if seed.DeletionTimestamp == nil {
			log.Info("Deleting seed")
			a.Recorder.Eventf(obj, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting seed %s", obj.GetName())
			if err := a.deleteSeed(ctx, obj.GetName()); err != nil {
				return false, false, fmt.Errorf("could not delete seed %s: %w", obj.GetName(), err)
			}
		} else {
			log.Info("Waiting for seed to be deleted")
			a.Recorder.Eventf(obj, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Waiting for seed %q to be deleted", obj.GetName())
		}

		return false, false, nil
	}

	// Delete gardenlet from the target cluster if it still exists and is not already deleting
	gardenletDeployment, err := a.getGardenletDeployment(ctx, targetClient)
	if err != nil {
		return false, false, fmt.Errorf("could not get gardenlet deployment in target cluster: %w", err)
	}

	if gardenletDeployment != nil {
		if gardenletDeployment.DeletionTimestamp == nil {
			log.Info("Deleting gardenlet from target cluster")
			a.Recorder.Eventf(obj, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting gardenlet from target cluster")
			if err := a.deleteGardenlet(ctx, log, obj, targetClient, gardenlet, seed, gardenletConfig); err != nil {
				return false, false, fmt.Errorf("could delete gardenlet from target cluster: %w", err)
			}
		} else {
			log.Info("Waiting for gardenlet to be deleted from target cluster")
			a.Recorder.Eventf(obj, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Waiting for gardenlet to be deleted from target cluster")
		}

		return true, false, nil
	}

	// Delete seed secrets if any of them still exists and is not already deleting
	secret, backupSecret, err := a.getSeedSecrets(ctx, obj, &seedTemplate.Spec)
	if err != nil {
		return false, false, fmt.Errorf("could not get seed %s secrets: %w", obj.GetName(), err)
	}

	if secret != nil || backupSecret != nil {
		if (secret != nil && secret.DeletionTimestamp == nil) || (backupSecret != nil && backupSecret.DeletionTimestamp == nil) {
			log.Info("Deleting seed secrets")
			a.Recorder.Event(obj, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting seed secrets")
			if err := a.deleteSeedSecrets(ctx, obj, &seedTemplate.Spec); err != nil {
				return false, false, fmt.Errorf("could not delete seed %s secrets: %w", obj.GetName(), err)
			}
		} else {
			log.Info("Waiting for seed secrets to be deleted")
			a.Recorder.Event(obj, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Waiting for seed secrets to be deleted")
		}

		return true, false, nil
	}

	// Delete garden namespace from the shoot if it still exists and is not already deleting
	gardenNamespace, err := a.getGardenNamespace(ctx, targetClient)
	if err != nil {
		return false, false, fmt.Errorf("could not check if garden namespace exists in target cluster: %w", err)
	}

	if gardenNamespace != nil {
		if gardenNamespace.DeletionTimestamp == nil {
			log.Info("Deleting garden namespace from target cluster")
			a.Recorder.Eventf(obj, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting garden namespace from target cluster")
			if err := a.deleteGardenNamespace(ctx, targetClient); err != nil {
				return false, false, fmt.Errorf("could not delete garden namespace from target cluster: %w", err)
			}
		} else {
			log.Info("Waiting for garden namespace to be deleted from target cluster")
			a.Recorder.Eventf(obj, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Waiting for garden namespace to be deleted from target cluster")
		}

		return true, false, nil
	}

	updateCondition(a.Clock, conditions, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleted,
		fmt.Sprintf("Seed %s has been unregistred", obj.GetName()))
	return false, true, nil
}

func (a *Actuator) ensureGardenNamespace(ctx context.Context, targetClient client.Client) error {
	gardenNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.GardenNamespaceTarget,
		},
	}
	if err := targetClient.Get(ctx, client.ObjectKeyFromObject(gardenNamespace), gardenNamespace); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return targetClient.Create(ctx, gardenNamespace)
	}
	return nil
}

func (a *Actuator) deleteGardenNamespace(ctx context.Context, shootClient kubernetes.Interface) error {
	gardenNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.GardenNamespaceTarget,
		},
	}
	return client.IgnoreNotFound(shootClient.Client().Delete(ctx, gardenNamespace))
}

func (a *Actuator) getGardenNamespace(ctx context.Context, shootClient kubernetes.Interface) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{}
	if err := shootClient.Client().Get(ctx, kubernetesutils.Key(a.GardenNamespaceTarget), ns); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return ns, nil
}

func (a *Actuator) deleteSeed(ctx context.Context, seedName string) error {
	seed := &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name: seedName,
		},
	}
	return client.IgnoreNotFound(a.GardenClient.Delete(ctx, seed))
}

func (a *Actuator) getSeed(ctx context.Context, seedName string) (*gardencorev1beta1.Seed, error) {
	seed := &gardencorev1beta1.Seed{}
	if err := a.GardenClient.Get(ctx, kubernetesutils.Key(seedName), seed); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return seed, nil
}

func (a *Actuator) deployGardenlet(
	ctx context.Context,
	log logr.Logger,
	obj client.Object,
	targetClient kubernetes.Interface,
	gardenlet *seedmanagementv1alpha1.Gardenlet,
	seed *gardencorev1beta1.Seed,
	gardenletConfig *gardenletv1alpha1.GardenletConfiguration,
) error {
	// Prepare gardenlet chart values
	values, err := a.prepareGardenletChartValues(
		ctx,
		log,
		obj,
		targetClient,
		gardenlet,
		seed,
		gardenletConfig,
		helper.GetBootstrap(gardenlet.Bootstrap),
		pointer.BoolDeref(gardenlet.MergeWithParent, false),
		a.GetTargetDomain(),
	)
	if err != nil {
		return err
	}

	// Apply gardenlet chart
	if err := targetClient.ChartApplier().ApplyFromEmbeddedFS(ctx, charts.ChartGardenlet, charts.ChartPathGardenlet, a.GardenNamespaceTarget, "gardenlet", kubernetes.Values(values)); err != nil {
		return err
	}

	// remove renew-kubeconfig annotation, if it exists
	if obj.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRenewKubeconfig {
		patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
		annotations := obj.GetAnnotations()
		delete(annotations, v1beta1constants.GardenerOperation)
		obj.SetAnnotations(annotations)
		if err := a.GardenClient.Patch(ctx, obj, patch); err != nil {
			return err
		}
	}

	return nil
}

func (a *Actuator) deleteGardenlet(
	ctx context.Context,
	log logr.Logger,
	obj client.Object,
	shootClient kubernetes.Interface,
	gardenlet *seedmanagementv1alpha1.Gardenlet,
	seed *gardencorev1beta1.Seed,
	gardenletConfig *gardenletv1alpha1.GardenletConfiguration,
) error {
	// Prepare gardenlet chart values
	values, err := a.prepareGardenletChartValues(
		ctx,
		log,
		obj,
		shootClient,
		gardenlet,
		seed,
		gardenletConfig,
		helper.GetBootstrap(gardenlet.Bootstrap),
		pointer.BoolDeref(gardenlet.MergeWithParent, false),
		a.GetTargetDomain(),
	)
	if err != nil {
		return err
	}

	// Delete gardenlet chart
	return shootClient.ChartApplier().DeleteFromEmbeddedFS(ctx, charts.ChartGardenlet, charts.ChartPathGardenlet, a.GardenNamespaceTarget, "gardenlet", kubernetes.Values(values))
}

func (a *Actuator) getGardenletDeployment(ctx context.Context, shootClient kubernetes.Interface) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	if err := shootClient.Client().Get(ctx, kubernetesutils.Key(a.GardenNamespaceTarget, v1beta1constants.DeploymentNameGardenlet), deployment); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return deployment, nil
}

func (a *Actuator) checkSeedSpec(ctx context.Context, spec *gardencorev1beta1.SeedSpec) error {
	// If VPA is enabled, check if the runtime namespace in the runtime cluster contains a vpa-admission-controller
	// deployment.
	if v1beta1helper.SeedSettingVerticalPodAutoscalerEnabled(spec.Settings) {
		runtimeVPAAdmissionControllerExists, err := a.CheckIfVPAAlreadyExists(ctx)
		if err != nil {
			return err
		}
		if runtimeVPAAdmissionControllerExists {
			return fmt.Errorf("runtime VPA is enabled but target cluster already has a VPA")
		}
	}

	return nil
}

func (a *Actuator) reconcileSeedSecrets(ctx context.Context, log logr.Logger, obj client.Object, spec *gardencorev1beta1.SeedSpec, gardenlet *seedmanagementv1alpha1.Gardenlet) error {
	// Get shoot secret
	infrastructureSecret, err := a.GetInfrastructureSecret(ctx)
	if err != nil {
		return err
	}

	// If backup is specified, create or update the backup secret if it doesn't exist or is owned by the managed seed
	if infrastructureSecret != nil && spec.Backup != nil {
		var checksum string

		// Get backup secret
		backupSecret, err := kubernetesutils.GetSecretByReference(ctx, a.GardenClient, &spec.Backup.SecretRef)
		if err == nil {
			checksum = utils.ComputeSecretChecksum(backupSecret.Data)[:8]
		} else if client.IgnoreNotFound(err) != nil {
			return err
		}

		// Create or update backup secret if it doesn't exist or is owned by the managed seed
		if apierrors.IsNotFound(err) || metav1.IsControlledBy(backupSecret, obj) {
			secret := &corev1.Secret{
				ObjectMeta: kubernetesutils.ObjectMeta(spec.Backup.SecretRef.Namespace, spec.Backup.SecretRef.Name),
			}
			if _, err := controllerutils.CreateOrGetAndStrategicMergePatch(ctx, a.GardenClient, secret, func() error {
				secret.OwnerReferences = []metav1.OwnerReference{
					*metav1.NewControllerRef(obj, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed")),
				}
				secret.Type = corev1.SecretTypeOpaque
				secret.Data = infrastructureSecret.Data
				return nil
			}); err != nil {
				return err
			}

			checksum = utils.ComputeSecretChecksum(secret.Data)[:8]
		}

		// Inject backup-secret hash into the pod annotations
		gardenlet.Deployment.PodAnnotations = utils.MergeStringMaps[string](gardenlet.Deployment.PodAnnotations, map[string]string{
			"checksum/seed-backup-secret": spec.Backup.SecretRef.Name + "-" + checksum,
		})
	}

	// If secret reference is specified and the static token kubeconfig is enabled,
	// create or update the corresponding secret with the kubeconfig from the static token kubeconfig secret.
	if spec.SecretRef != nil {
		// Get target kubeconfig secret
		targetKubeconfigSecret, err := a.GetKubeconfigSecret(ctx)
		if err != nil {
			return err
		}
		if targetKubeconfigSecret != nil {
			// Create or update seed secret
			secret := &corev1.Secret{
				ObjectMeta: kubernetesutils.ObjectMeta(spec.SecretRef.Namespace, spec.SecretRef.Name),
			}
			if _, err := controllerutils.CreateOrGetAndStrategicMergePatch(ctx, a.GardenClient, secret, func() error {
				secret.OwnerReferences = []metav1.OwnerReference{
					*metav1.NewControllerRef(obj, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed")),
				}
				secret.Type = corev1.SecretTypeOpaque
				secret.Data = map[string][]byte{
					kubernetes.KubeConfig: targetKubeconfigSecret.Data[kubernetes.KubeConfig],
				}
				return nil
			}); err != nil {
				return err
			}
		}
	}

	// When the secretRef is unset, cleanup the secret if the reference annotations are present in the managedseed.
	if spec.SecretRef == nil &&
		obj.GetAnnotations()[seedmanagementv1alpha1constants.AnnotationSeedSecretName] != "" &&
		obj.GetAnnotations()[seedmanagementv1alpha1constants.AnnotationSeedSecretNamespace] != "" {
		secret, err := kubernetesutils.GetSecretByReference(ctx,
			a.GardenClient,
			&corev1.SecretReference{
				Name:      obj.GetAnnotations()[seedmanagementv1alpha1constants.AnnotationSeedSecretName],
				Namespace: obj.GetAnnotations()[seedmanagementv1alpha1constants.AnnotationSeedSecretNamespace],
			},
		)
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed getting secret in the managedseed annotations: %w", err)
		}

		if err == nil && metav1.IsControlledBy(secret, obj) {
			// This finalizer is added by the seed controller, but it cannot remove it when the secretRef is unset.
			if controllerutil.ContainsFinalizer(secret, gardencorev1beta1.ExternalGardenerName) {
				if err := controllerutils.RemoveFinalizers(ctx, a.GardenClient, secret, gardencorev1beta1.ExternalGardenerName); err != nil {
					return fmt.Errorf("failed to remove finalizer from secret referred by managedseed: %w", err)
				}
			}

			if err := a.GardenClient.Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("error deleting seed secret referred by managedseed: %w", err)
			}
			log.Info("Deleted seed secret referred by managedseed", "secretName", secret.Name, "secretNamespace", secret.Namespace)
		}

		patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
		annotations := obj.GetAnnotations()
		delete(annotations, seedmanagementv1alpha1constants.AnnotationSeedSecretName)
		delete(annotations, seedmanagementv1alpha1constants.AnnotationSeedSecretNamespace)
		obj.SetAnnotations(annotations)
		if err := a.GardenClient.Patch(ctx, obj, patch); err != nil {
			return fmt.Errorf("error removing seed secret reference annotations: %w", err)
		}
	}

	return nil
}

func (a *Actuator) deleteSeedSecrets(ctx context.Context, obj client.Object, spec *gardencorev1beta1.SeedSpec) error {
	// If backup is specified, delete the backup secret if it exists and is owned by the managed seed
	if spec.Backup != nil {
		backupSecret, err := kubernetesutils.GetSecretByReference(ctx, a.GardenClient, &spec.Backup.SecretRef)
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		if err == nil && metav1.IsControlledBy(backupSecret, obj) {
			if err := kubernetesutils.DeleteSecretByReference(ctx, a.GardenClient, &spec.Backup.SecretRef); err != nil {
				return err
			}
		}
	}

	// If secret reference is specified, delete the corresponding secret
	if spec.SecretRef != nil {
		if err := kubernetesutils.DeleteSecretByReference(ctx, a.GardenClient, spec.SecretRef); err != nil {
			return err
		}
	}

	return nil
}

func (a *Actuator) getSeedSecrets(ctx context.Context, obj client.Object, spec *gardencorev1beta1.SeedSpec) (*corev1.Secret, *corev1.Secret, error) {
	var secret, backupSecret *corev1.Secret
	var err error

	// If backup is specified, get the backup secret if it exists and is owned by the managed seed
	if spec.Backup != nil {
		backupSecret, err = kubernetesutils.GetSecretByReference(ctx, a.GardenClient, &spec.Backup.SecretRef)
		if client.IgnoreNotFound(err) != nil {
			return nil, nil, err
		}
		if backupSecret != nil && !metav1.IsControlledBy(backupSecret, obj) {
			backupSecret = nil
		}
	}

	// If secret reference is specified, get the corresponding secret if it exists
	if spec.SecretRef != nil {
		secret, err = kubernetesutils.GetSecretByReference(ctx, a.GardenClient, spec.SecretRef)
		if client.IgnoreNotFound(err) != nil {
			return nil, nil, err
		}
	}

	return secret, backupSecret, nil
}

func (a *Actuator) getShootSecret(ctx context.Context, shoot *gardencorev1beta1.Shoot) (*corev1.Secret, error) {
	shootSecretBinding := &gardencorev1beta1.SecretBinding{}
	if shoot.Spec.SecretBindingName == nil {
		return nil, fmt.Errorf("secretbinding name is nil for the Shoot: %s/%s", shoot.Namespace, shoot.Name)
	}
	if err := a.GardenClient.Get(ctx, kubernetesutils.Key(shoot.Namespace, *shoot.Spec.SecretBindingName), shootSecretBinding); err != nil {
		return nil, err
	}
	return kubernetesutils.GetSecretByReference(ctx, a.GardenClient, &shootSecretBinding.SecretRef)
}

func (a *Actuator) getShootKubeconfigSecret(ctx context.Context, shoot *gardencorev1beta1.Shoot) (*corev1.Secret, error) {
	shootKubeconfigSecret := &corev1.Secret{}
	if err := a.GardenClient.Get(ctx, kubernetesutils.Key(shoot.Namespace, gardenerutils.ComputeShootProjectSecretName(shoot.Name, gardenerutils.ShootProjectSecretSuffixKubeconfig)), shootKubeconfigSecret); err != nil {
		return nil, err
	}
	return shootKubeconfigSecret, nil
}

func (a *Actuator) runtimeVPADeploymentExists(ctx context.Context, runtimeClient client.Client, runtimeNamespace string) (bool, error) {
	if err := runtimeClient.Get(ctx, kubernetesutils.Key(runtimeNamespace, "vpa-admission-controller"), &appsv1.Deployment{}); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *Actuator) prepareGardenletChartValues(
	ctx context.Context,
	log logr.Logger,
	obj client.Object,
	shootClient kubernetes.Interface,
	gardenlet *seedmanagementv1alpha1.Gardenlet,
	seed *gardencorev1beta1.Seed,
	gardenletConfig *gardenletv1alpha1.GardenletConfiguration,
	bootstrap seedmanagementv1alpha1.Bootstrap,
	mergeWithParent bool,
	targetDomain string,
) (map[string]interface{}, error) {
	// Merge gardenlet deployment with parent values
	deployment, err := a.ValuesHelper.MergeGardenletDeployment(gardenlet.Deployment)
	if err != nil {
		return nil, err
	}

	// Merge gardenlet configuration with parent if specified
	if mergeWithParent {
		gardenletConfig, err = a.ValuesHelper.MergeGardenletConfiguration(gardenletConfig)
		if err != nil {
			return nil, err
		}
	}

	// Ensure garden client connection is set
	if gardenletConfig.GardenClientConnection == nil {
		gardenletConfig.GardenClientConnection = &gardenletv1alpha1.GardenClientConnection{}
	}

	// Prepare garden client connection
	var bootstrapKubeconfig string
	if bootstrap == seedmanagementv1alpha1.BootstrapNone {
		a.removeBootstrapConfigFromGardenClientConnection(gardenletConfig.GardenClientConnection)
	} else {
		bootstrapKubeconfig, err = a.prepareGardenClientConnectionWithBootstrap(ctx, log, obj, shootClient, gardenletConfig.GardenClientConnection, seed, bootstrap)
		if err != nil {
			return nil, err
		}
	}

	// Ensure seed config is set
	if gardenletConfig.SeedConfig == nil {
		gardenletConfig.SeedConfig = &gardenletv1alpha1.SeedConfig{}
	}

	// Set the seed name
	gardenletConfig.SeedConfig.SeedTemplate.Name = obj.GetName()

	// Get gardenlet chart values
	return a.ValuesHelper.GetGardenletChartValues(
		ensureGardenletEnvironment(deployment, targetDomain),
		gardenletConfig,
		bootstrapKubeconfig,
	)
}

// ensureGardenletEnvironment sets the KUBERNETES_SERVICE_HOST to the API of the ManagedSeed cluster.
// This is needed so that the deployed gardenlet can properly set the network policies allowing
// access of control plane components of the hosted shoots to the API server of the (managed) seed.
func ensureGardenletEnvironment(deployment *seedmanagementv1alpha1.GardenletDeployment, domain string) *seedmanagementv1alpha1.GardenletDeployment {
	const kubernetesServiceHost = "KUBERNETES_SERVICE_HOST"

	if deployment.Env == nil {
		deployment.Env = []corev1.EnvVar{}
	}

	for _, env := range deployment.Env {
		if env.Name == kubernetesServiceHost {
			return deployment
		}
	}

	if len(domain) != 0 {
		deployment.Env = append(
			deployment.Env,
			corev1.EnvVar{
				Name:  kubernetesServiceHost,
				Value: gardenerutils.GetAPIServerDomain(domain),
			},
		)
	}

	return deployment
}

func (a *Actuator) prepareGardenClientConnectionWithBootstrap(
	ctx context.Context,
	log logr.Logger,
	obj client.Object,
	targetClient kubernetes.Interface,
	gcc *gardenletv1alpha1.GardenClientConnection,
	seed *gardencorev1beta1.Seed,
	bootstrap seedmanagementv1alpha1.Bootstrap,
) (
	string,
	error,
) {
	// Ensure kubeconfig secret is set
	if gcc.KubeconfigSecret == nil {
		gcc.KubeconfigSecret = &corev1.SecretReference{
			Name:      GardenletDefaultKubeconfigSecretName,
			Namespace: a.GardenNamespaceTarget,
		}
	}

	if seed != nil && seed.Status.ClientCertificateExpirationTimestamp != nil && seed.Status.ClientCertificateExpirationTimestamp.UTC().Before(time.Now().UTC()) {
		// Check if client certificate is expired. If yes then delete the existing kubeconfig secret to make sure that the
		// seed can be re-bootstrapped.
		if err := kubernetesutils.DeleteSecretByReference(ctx, targetClient.Client(), gcc.KubeconfigSecret); err != nil {
			return "", err
		}
	} else if obj.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRenewKubeconfig {
		// Also remove the kubeconfig if the renew-kubeconfig operation annotation is set on the ManagedSeed resource.
		log.Info("Renewing gardenlet kubeconfig secret due to operation annotation")
		a.Recorder.Event(obj, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Renewing gardenlet kubeconfig secret due to operation annotation")

		if err := kubernetesutils.DeleteSecretByReference(ctx, targetClient.Client(), gcc.KubeconfigSecret); err != nil {
			return "", err
		}
	} else {
		seedIsAlreadyBootstrapped, err := isAlreadyBootstrapped(ctx, targetClient.Client(), gcc.KubeconfigSecret)
		if err != nil {
			return "", err
		}

		if seedIsAlreadyBootstrapped {
			return "", nil
		}
	}

	// Ensure kubeconfig is not set
	gcc.Kubeconfig = ""

	// Ensure bootstrap kubeconfig secret is set
	if gcc.BootstrapKubeconfig == nil {
		gcc.BootstrapKubeconfig = &corev1.SecretReference{
			Name:      GardenletDefaultKubeconfigBootstrapSecretName,
			Namespace: a.GardenNamespaceTarget,
		}
	}

	return a.createBootstrapKubeconfig(ctx, metav1.ObjectMeta{Name: obj.GetName(), Namespace: obj.GetNamespace()}, bootstrap, gcc.GardenClusterAddress, gcc.GardenClusterCACert)
}

// isAlreadyBootstrapped checks if the gardenlet already has a valid Garden cluster certificate through TLS bootstrapping
// by checking if the specified secret reference already exists
func isAlreadyBootstrapped(ctx context.Context, c client.Client, s *corev1.SecretReference) (bool, error) {
	// If kubeconfig secret exists, return an empty result, since the bootstrap can be skipped
	secret, err := kubernetesutils.GetSecretByReference(ctx, c, s)
	if client.IgnoreNotFound(err) != nil {
		return false, err
	}
	return secret != nil, nil
}

func (a *Actuator) removeBootstrapConfigFromGardenClientConnection(gcc *gardenletv1alpha1.GardenClientConnection) {
	// Ensure kubeconfig secret and bootstrap kubeconfig secret are not set
	gcc.KubeconfigSecret = nil
	gcc.BootstrapKubeconfig = nil
}

// createBootstrapKubeconfig creates a kubeconfig for the Garden cluster
// containing either the token of a service account or a bootstrap token
// returns the kubeconfig as a string
func (a *Actuator) createBootstrapKubeconfig(ctx context.Context, objectMeta metav1.ObjectMeta, bootstrap seedmanagementv1alpha1.Bootstrap, address *string, caCert []byte) (string, error) {
	var (
		err                 error
		bootstrapKubeconfig []byte
	)

	gardenClientRestConfig := gardenerutils.PrepareGardenClientRestConfig(a.GardenConfig, address, caCert)

	switch bootstrap {
	case seedmanagementv1alpha1.BootstrapServiceAccount:
		// Create a kubeconfig containing the token of a temporary service account as client credentials
		var (
			serviceAccountName      = gardenletbootstraputil.ServiceAccountName(objectMeta.Name)
			serviceAccountNamespace = objectMeta.Namespace
		)

		if serviceAccountNamespace == "" {
			serviceAccountNamespace = v1beta1constants.GardenNamespace
		}

		// Create a kubeconfig containing a valid service account token as client credentials
		kubernetesClientSet, err := kubernetesclientset.NewForConfig(gardenClientRestConfig)
		if err != nil {
			return "", fmt.Errorf("failed creating Kubernetes client: %w", err)
		}

		bootstrapKubeconfig, err = gardenletbootstraputil.ComputeGardenletKubeconfigWithServiceAccountToken(ctx, a.GardenClient, kubernetesClientSet.CoreV1(), gardenClientRestConfig, serviceAccountName, serviceAccountNamespace)
		if err != nil {
			return "", err
		}

	case seedmanagementv1alpha1.BootstrapToken:
		var (
			tokenID          = gardenletbootstraputil.TokenID(objectMeta)
			tokenDescription = gardenletbootstraputil.Description(gardenletbootstraputil.KindManagedSeed, objectMeta.Namespace, objectMeta.Name)
			tokenValidity    = 24 * time.Hour
		)

		// Create a kubeconfig containing a valid bootstrap token as client credentials
		bootstrapKubeconfig, err = gardenletbootstraputil.ComputeGardenletKubeconfigWithBootstrapToken(ctx, a.GardenClient, gardenClientRestConfig, tokenID, tokenDescription, tokenValidity)
		if err != nil {
			return "", err
		}
	}

	return string(bootstrapKubeconfig), nil
}

func updateCondition(clock clock.Clock, conditions []gardencorev1beta1.Condition, ct gardencorev1beta1.ConditionType, cs gardencorev1beta1.ConditionStatus, reason, message string) {
	condition := v1beta1helper.GetOrInitConditionWithClock(clock, conditions, ct)
	condition = v1beta1helper.UpdatedConditionWithClock(clock, condition, cs, reason, message)
	conditions = v1beta1helper.MergeConditions(conditions, condition)
}
