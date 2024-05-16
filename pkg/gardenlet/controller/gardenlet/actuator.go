// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/gardenlet/controller/managedseed"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/oci"
)

// Actuator acts upon Gardenlet resources.
type Actuator interface {
	// Reconcile reconciles Gardenlet creation or update.
	Reconcile(context.Context, logr.Logger, *seedmanagementv1alpha1.Gardenlet) (*seedmanagementv1alpha1.GardenletStatus, bool, error)
}

// actuator is a concrete implementation of Actuator.
type actuator struct {
	gardenConfig *rest.Config
	gardenClient client.Client
	seedClient   kubernetes.Interface
	clock        clock.Clock
	vp           managedseed.ValuesHelper
	recorder     record.EventRecorder
	helmRegistry oci.Interface
}

// newActuator creates a new Actuator with the given clients, ValuesHelper, and logger.
func newActuator(
	gardenConfig *rest.Config,
	gardenClient client.Client,
	seedClient kubernetes.Interface,
	clock clock.Clock,
	vp managedseed.ValuesHelper,
	recorder record.EventRecorder,
	helmRegistry oci.Interface,
) Actuator {
	return &actuator{
		gardenConfig: gardenConfig,
		gardenClient: gardenClient,
		seedClient:   seedClient,
		clock:        clock,
		vp:           vp,
		recorder:     recorder,
		helmRegistry: helmRegistry,
	}
}

// Reconcile reconciles Gardenlet creation or update.
func (a *actuator) Reconcile(
	ctx context.Context,
	log logr.Logger,
	gardenlet *seedmanagementv1alpha1.Gardenlet,
) (
	status *seedmanagementv1alpha1.GardenletStatus,
	wait bool,
	err error,
) {
	// Initialize status
	status = gardenlet.Status.DeepCopy()
	status.ObservedGeneration = gardenlet.Generation

	defer func() {
		if err != nil {
			log.Error(err, "Error during reconciliation")
			a.recorder.Eventf(gardenlet, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, err.Error())
			updateCondition(a.clock, status, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconcileError, err.Error())
		}
	}()

	// Extract seed template and gardenlet config
	_, gardenletConfig, err := helper.ExtractSeedTemplateAndGardenletConfig(gardenlet.Name, &gardenlet.Spec.Config)
	if err != nil {
		return status, false, err
	}

	seed, err := a.getSeed(ctx, gardenlet)
	if err != nil {
		return status, false, fmt.Errorf("could not read seed %s: %w", gardenlet.Name, err)
	}

	log.Info("Deploying gardenlet")
	a.recorder.Eventf(gardenlet, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Deploying gardenlet")
	if err := a.deployGardenlet(ctx, log, gardenlet, seed, gardenletConfig); err != nil {
		return status, false, fmt.Errorf("could not deploy gardenlet %w", err)
	}

	updateCondition(a.clock, status, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionTrue, gardencorev1beta1.EventReconciled,
		fmt.Sprintf("Gardenlet %s has been deployed", gardenlet.Name))
	return status, false, nil
}

func (a *actuator) getSeed(ctx context.Context, gardenlet *seedmanagementv1alpha1.Gardenlet) (*gardencorev1beta1.Seed, error) {
	seed := &gardencorev1beta1.Seed{}
	if err := a.gardenClient.Get(ctx, kubernetesutils.Key(gardenlet.Name), seed); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return seed, nil
}

func (a *actuator) deployGardenlet(
	ctx context.Context,
	log logr.Logger,
	gardenlet *seedmanagementv1alpha1.Gardenlet,
	seed *gardencorev1beta1.Seed,
	gardenletConfig *gardenletv1alpha1.GardenletConfiguration,
) error {
	// Prepare gardenlet chart values
	values, err := a.prepareGardenletChartValues(
		ctx,
		log,
		gardenlet,
		seed,
		gardenletConfig,
	)
	if err != nil {
		return err
	}

	// Apply gardenlet chart
	archive := gardenlet.Spec.Deployment.Helm.RawChart
	if len(archive) == 0 {
		var err error
		archive, err = a.helmRegistry.Pull(ctx, gardenlet.Spec.Deployment.Helm.OCIRepository)
		if err != nil {
			return fmt.Errorf("failed pulling Helm chart from OCI repository: %w", err)
		}
	}

	if err := a.seedClient.ChartApplier().ApplyFromArchive(ctx, archive, v1beta1constants.GardenNamespace, "gardenlet", kubernetes.Values(values)); err != nil {
		return err
	}

	// remove renew-kubeconfig annotation, if it exists
	if gardenlet.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRenewKubeconfig {
		patch := client.MergeFrom(gardenlet.DeepCopy())
		delete(gardenlet.Annotations, v1beta1constants.GardenerOperation)
		if err := a.gardenClient.Patch(ctx, gardenlet, patch); err != nil {
			return err
		}
	}

	return nil
}

func (a *actuator) getGardenletDeployment(ctx context.Context, shootClient kubernetes.Interface) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	if err := shootClient.Client().Get(ctx, kubernetesutils.Key(v1beta1constants.GardenNamespace, v1beta1constants.DeploymentNameGardenlet), deployment); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return deployment, nil
}

func (a *actuator) prepareGardenletChartValues(
	ctx context.Context,
	log logr.Logger,
	gardenlet *seedmanagementv1alpha1.Gardenlet,
	seed *gardencorev1beta1.Seed,
	gardenletConfig *gardenletv1alpha1.GardenletConfiguration,
) (map[string]interface{}, error) {
	// Ensure garden client connection is set
	if gardenletConfig.GardenClientConnection == nil {
		gardenletConfig.GardenClientConnection = &gardenletv1alpha1.GardenClientConnection{}
	}

	// Prepare garden client connection
	bootstrapKubeconfig, err := a.prepareGardenClientConnectionWithBootstrap(ctx, log, gardenletConfig.GardenClientConnection, gardenlet, seed)
	if err != nil {
		return nil, err
	}

	// Ensure seed config is set
	if gardenletConfig.SeedConfig == nil {
		gardenletConfig.SeedConfig = &gardenletv1alpha1.SeedConfig{}
	}

	// Set the seed name
	gardenletConfig.SeedConfig.SeedTemplate.Name = gardenlet.Name

	// Get gardenlet chart values
	values, err := a.vp.GetGardenletChartValues(
		&gardenlet.Spec.Deployment.GardenletDeployment,
		gardenletConfig,
		bootstrapKubeconfig,
	)
	if err != nil {
		return nil, err
	}

	if imageVector := gardenlet.Spec.Deployment.ImageVectorOverwrite; imageVector != nil {
		values["imageVectorOverwrite"] = *imageVector
	}

	if imageVector := gardenlet.Spec.Deployment.ComponentImageVectorOverwrite; imageVector != nil {
		values["componentImageVectorOverwrites"] = *imageVector
	}

	return values, nil
}

func (a *actuator) prepareGardenClientConnectionWithBootstrap(
	ctx context.Context,
	log logr.Logger,
	gcc *gardenletv1alpha1.GardenClientConnection,
	gardenlet *seedmanagementv1alpha1.Gardenlet,
	seed *gardencorev1beta1.Seed,
) (
	string,
	error,
) {
	// Ensure kubeconfig secret is set
	if gcc.KubeconfigSecret == nil {
		gcc.KubeconfigSecret = &corev1.SecretReference{
			Name:      GardenletDefaultKubeconfigSecretName,
			Namespace: v1beta1constants.GardenNamespace,
		}
	}

	if seed != nil && seed.Status.ClientCertificateExpirationTimestamp != nil && seed.Status.ClientCertificateExpirationTimestamp.UTC().Before(time.Now().UTC()) {
		// Check if client certificate is expired. If yes then delete the existing kubeconfig secret to make sure that the
		// seed can be re-bootstrapped.
		if err := kubernetesutils.DeleteSecretByReference(ctx, a.seedClient.Client(), gcc.KubeconfigSecret); err != nil {
			return "", err
		}
	} else if gardenlet.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRenewKubeconfig {
		// Also remove the kubeconfig if the renew-kubeconfig operation annotation is set on the Gardenlet resource.
		log.Info("Renewing gardenlet kubeconfig secret due to operation annotation")
		a.recorder.Event(gardenlet, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Renewing gardenlet kubeconfig secret due to operation annotation")

		if err := kubernetesutils.DeleteSecretByReference(ctx, a.seedClient.Client(), gcc.KubeconfigSecret); err != nil {
			return "", err
		}
	} else {
		seedIsAlreadyBootstrapped, err := isAlreadyBootstrapped(ctx, a.seedClient.Client(), gcc.KubeconfigSecret)
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
			Namespace: v1beta1constants.GardenNamespace,
		}
	}

	return a.createBootstrapKubeconfig(ctx, gardenlet.ObjectMeta, gcc.GardenClusterAddress, gcc.GardenClusterCACert)
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

// createBootstrapKubeconfig creates a kubeconfig for the Garden cluster
// containing either the token of a service account or a bootstrap token
// returns the kubeconfig as a string
func (a *actuator) createBootstrapKubeconfig(ctx context.Context, objectMeta metav1.ObjectMeta, address *string, caCert []byte) (string, error) {
	var (
		err                 error
		bootstrapKubeconfig []byte
	)

	gardenClientRestConfig := gardenerutils.PrepareGardenClientRestConfig(a.gardenConfig, address, caCert)

	var (
		tokenID          = gardenletbootstraputil.TokenID(objectMeta)
		tokenDescription = gardenletbootstraputil.Description(gardenletbootstraputil.KindGardenlet, objectMeta.Namespace, objectMeta.Name)
		tokenValidity    = 24 * time.Hour
	)

	// Create a kubeconfig containing a valid bootstrap token as client credentials
	bootstrapKubeconfig, err = gardenletbootstraputil.ComputeGardenletKubeconfigWithBootstrapToken(ctx, a.gardenClient, gardenClientRestConfig, tokenID, tokenDescription, tokenValidity)
	if err != nil {
		return "", err
	}

	return string(bootstrapKubeconfig), nil
}

func updateCondition(clock clock.Clock, status *seedmanagementv1alpha1.GardenletStatus, ct gardencorev1beta1.ConditionType, cs gardencorev1beta1.ConditionStatus, reason, message string) {
	condition := v1beta1helper.GetOrInitConditionWithClock(clock, status.Conditions, ct)
	condition = v1beta1helper.UpdatedConditionWithClock(clock, condition, cs, reason, message)
	status.Conditions = v1beta1helper.MergeConditions(status.Conditions, condition)
}
