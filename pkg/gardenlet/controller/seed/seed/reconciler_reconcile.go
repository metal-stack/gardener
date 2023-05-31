// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2"
	"github.com/go-logr/logr"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/clusterautoscaler"
	"github.com/gardener/gardener/pkg/component/clusteridentity"
	"github.com/gardener/gardener/pkg/component/coredns"
	"github.com/gardener/gardener/pkg/component/dependencywatchdog"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/extensions"
	"github.com/gardener/gardener/pkg/component/extensions/crds"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/docker"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/component/hvpa"
	"github.com/gardener/gardener/pkg/component/istio"
	"github.com/gardener/gardener/pkg/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/component/kubeapiserverexposure"
	"github.com/gardener/gardener/pkg/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/component/kubeproxy"
	"github.com/gardener/gardener/pkg/component/kubernetesdashboard"
	"github.com/gardener/gardener/pkg/component/kubescheduler"
	"github.com/gardener/gardener/pkg/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/component/logging/eventlogger"
	"github.com/gardener/gardener/pkg/component/logging/fluentoperator"
	"github.com/gardener/gardener/pkg/component/logging/vali"
	"github.com/gardener/gardener/pkg/component/metricsserver"
	"github.com/gardener/gardener/pkg/component/monitoring"
	"github.com/gardener/gardener/pkg/component/nginxingress"
	"github.com/gardener/gardener/pkg/component/nginxingressshoot"
	"github.com/gardener/gardener/pkg/component/nodeexporter"
	"github.com/gardener/gardener/pkg/component/nodeproblemdetector"
	"github.com/gardener/gardener/pkg/component/resourcemanager"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/component/vpa"
	"github.com/gardener/gardener/pkg/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/component/vpnshoot"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/operation/common"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/timewindow"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

func (r *Reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	seedObj *seedpkg.Seed,
	seedIsGarden bool,
) (
	reconcile.Result,
	error,
) {
	var (
		seed                      = seedObj.GetInfo()
		conditionSeedBootstrapped = v1beta1helper.GetOrInitConditionWithClock(r.Clock, seedObj.GetInfo().Status.Conditions, gardencorev1beta1.SeedBootstrapped)
	)

	// Initialize capacity and allocatable
	var capacity, allocatable corev1.ResourceList
	if r.Config.Resources != nil && len(r.Config.Resources.Capacity) > 0 {
		capacity = make(corev1.ResourceList, len(r.Config.Resources.Capacity))
		allocatable = make(corev1.ResourceList, len(r.Config.Resources.Capacity))

		for resourceName, quantity := range r.Config.Resources.Capacity {
			capacity[resourceName] = quantity
			allocatable[resourceName] = quantity

			if reservedQuantity, ok := r.Config.Resources.Reserved[resourceName]; ok {
				allocatableQuantity := quantity.DeepCopy()
				allocatableQuantity.Sub(reservedQuantity)
				allocatable[resourceName] = allocatableQuantity
			}
		}
	}

	if !controllerutil.ContainsFinalizer(seed, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.GardenClient, seed, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Add the Gardener finalizer to the referenced Seed secret to protect it from deletion as long as the Seed resource
	// does exist.
	if seed.Spec.SecretRef != nil {
		secret, err := kubernetesutils.GetSecretByReference(ctx, r.GardenClient, seed.Spec.SecretRef)
		if err != nil {
			return reconcile.Result{}, err
		}

		if !controllerutil.ContainsFinalizer(secret, gardencorev1beta1.ExternalGardenerName) {
			log.Info("Adding finalizer to referenced secret", "secret", client.ObjectKeyFromObject(secret))
			if err := controllerutils.AddFinalizers(ctx, r.GardenClient, secret, gardencorev1beta1.ExternalGardenerName); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	// Check whether the Kubernetes version of the Seed cluster fulfills the minimal requirements.
	seedKubernetesVersion, err := r.checkMinimumK8SVersion(r.SeedClientSet.Version())
	if err != nil {
		conditionSeedBootstrapped = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "K8SVersionTooOld", err.Error())
		if err := r.patchSeedStatus(ctx, r.GardenClient, seed, "<unknown>", capacity, allocatable, conditionSeedBootstrapped); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not patch seed status after check for minimum Kubernetes version failed: %w", err)
		}
		return reconcile.Result{}, err
	}

	gardenSecrets, err := gardenerutils.ReadGardenSecrets(ctx, log, r.GardenClient, gardenerutils.ComputeGardenNamespace(seed.Name), true)
	if err != nil {
		conditionSeedBootstrapped = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "GardenSecretsError", err.Error())
		if err := r.patchSeedStatus(ctx, r.GardenClient, seed, "<unknown>", capacity, allocatable, conditionSeedBootstrapped); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not patch seed status after reading garden secrets failed: %w", err)
		}
		return reconcile.Result{}, err
	}

	conditionSeedBootstrapped = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionSeedBootstrapped, gardencorev1beta1.ConditionProgressing, "BootstrapProgressing", "Seed cluster is currently being bootstrapped.")
	if err = r.patchSeedStatus(ctx, r.GardenClient, seed, seedKubernetesVersion, capacity, allocatable, conditionSeedBootstrapped); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status of %s condition to %s: %w", conditionSeedBootstrapped.Type, gardencorev1beta1.ConditionProgressing, err)
	}

	// Bootstrap the Seed cluster.
	if err := r.runReconcileSeedFlow(ctx, log, seedObj, seedIsGarden, gardenSecrets); err != nil {
		conditionSeedBootstrapped = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "BootstrappingFailed", err.Error())
		if err := r.patchSeedStatus(ctx, r.GardenClient, seed, "<unknown>", capacity, allocatable, conditionSeedBootstrapped); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not patch seed status after reconciliation flow failed: %w", err)
		}
		return reconcile.Result{}, err
	}

	// Set the status of SeedSystemComponentsHealthy condition to Progressing so that the Seed does not immediately become ready
	// after being successfully bootstrapped in case the system components got updated. The SeedSystemComponentsHealthy condition
	// will be set to either True, False or Progressing by the seed care reconciler depending on the health of the system components
	// after the necessary checks are completed.
	conditionSeedSystemComponentsHealthy := v1beta1helper.GetOrInitConditionWithClock(r.Clock, seed.Status.Conditions, gardencorev1beta1.SeedSystemComponentsHealthy)
	conditionSeedSystemComponentsHealthy = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionSeedSystemComponentsHealthy, gardencorev1beta1.ConditionProgressing, "SystemComponentsCheckProgressing", "Pending health check of system components after successful bootstrap of seed cluster.")
	conditionSeedBootstrapped = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionSeedBootstrapped, gardencorev1beta1.ConditionTrue, "BootstrappingSucceeded", "Seed cluster has been bootstrapped successfully.")
	if err = r.patchSeedStatus(ctx, r.GardenClient, seed, seedKubernetesVersion, capacity, allocatable, conditionSeedBootstrapped, conditionSeedSystemComponentsHealthy); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status of %s condition to %s and %s conditions to %s: %w", conditionSeedBootstrapped.Type, gardencorev1beta1.ConditionTrue, conditionSeedSystemComponentsHealthy.Type, gardencorev1beta1.ConditionProgressing, err)
	}

	if seed.Spec.Backup != nil {
		// This should be post updating the seed is available. Since, scheduler will then mostly use
		// same seed for deploying the backupBucket extension.
		if err := deployBackupBucketInGarden(ctx, r.GardenClient, seed); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{RequeueAfter: r.Config.Controllers.Seed.SyncPeriod.Duration}, nil
}

func (r *Reconciler) checkMinimumK8SVersion(version string) (string, error) {
	const minKubernetesVersion = "1.20"

	seedVersionOK, err := versionutils.CompareVersions(version, ">=", minKubernetesVersion)
	if err != nil {
		return "<unknown>", err
	}
	if !seedVersionOK {
		return "<unknown>", fmt.Errorf("the Kubernetes version of the Seed cluster must be at least %s", minKubernetesVersion)
	}

	return version, nil
}

const (
	seedBootstrapChartName        = "seed-bootstrap"
	kubeAPIServerPrefix           = "api-seed"
	plutonoPrefix                 = "g-seed"
	prometheusPrefix              = "p-seed"
	ingressTLSCertificateValidity = 730 * 24 * time.Hour // ~2 years, see https://support.apple.com/en-us/HT210176
)

func (r *Reconciler) runReconcileSeedFlow(
	ctx context.Context,
	log logr.Logger,
	seed *seedpkg.Seed,
	seedIsGarden bool,
	secrets map[string]*corev1.Secret,
) error {
	var (
		applier       = r.SeedClientSet.Applier()
		seedClient    = r.SeedClientSet.Client()
		chartApplier  = r.SeedClientSet.ChartApplier()
		chartRenderer = r.SeedClientSet.ChartRenderer()
	)

	secretsManager, err := secretsmanager.New(
		ctx,
		log.WithName("secretsmanager"),
		clock.RealClock{},
		seedClient,
		r.GardenNamespace,
		v1beta1constants.SecretManagerIdentityGardenlet,
		secretsmanager.Config{CASecretAutoRotation: true},
	)
	if err != nil {
		return err
	}

	// Deploy dedicated CA certificate for seed cluster, auto-rotate it roughly once a month and drop the old CA 24 hours
	// after rotation.
	if _, err := secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:       v1beta1constants.SecretNameCASeed,
		CommonName: "kubernetes",
		CertType:   secretsutils.CACert,
		Validity:   pointer.Duration(30 * 24 * time.Hour),
	}, secretsmanager.Rotate(secretsmanager.KeepOld), secretsmanager.IgnoreOldSecretsAfter(24*time.Hour)); err != nil {
		return err
	}

	kubernetesVersion, err := semver.NewVersion(r.SeedClientSet.Version())
	if err != nil {
		return err
	}

	var (
		vpaGK    = schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "VerticalPodAutoscaler"}
		hvpaGK   = schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "Hvpa"}
		issuerGK = schema.GroupKind{Group: "certmanager.k8s.io", Kind: "ClusterIssuer"}

		vpaEnabled     = seed.GetInfo().Spec.Settings == nil || seed.GetInfo().Spec.Settings.VerticalPodAutoscaler == nil || seed.GetInfo().Spec.Settings.VerticalPodAutoscaler.Enabled
		loggingEnabled = gardenlethelper.IsLoggingEnabled(&r.Config)
		hvpaEnabled    = features.DefaultFeatureGate.Enabled(features.HVPA)

		loggingConfig   = r.Config.Logging
		gardenNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: r.GardenNamespace,
			},
		}
	)

	if !vpaEnabled {
		// VPA is a prerequisite. If it's not enabled via the seed spec it must be provided through some other mechanism.
		if _, err := seedClient.RESTMapper().RESTMapping(vpaGK); err != nil {
			return fmt.Errorf("VPA is required for seed cluster: %s", err)
		}
	}

	// create + label garden namespace
	log.Info("Labeling and annotating namespace", "namespaceName", gardenNamespace.Name)
	if _, err := controllerutils.CreateOrGetAndMergePatch(ctx, seedClient, gardenNamespace, func() error {
		metav1.SetMetaDataLabel(&gardenNamespace.ObjectMeta, "role", v1beta1constants.GardenNamespace)

		// When the seed is the garden cluster then this information is managed by gardener-operator.
		if !seedIsGarden {
			metav1.SetMetaDataLabel(&gardenNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigConsider, "true")
			metav1.SetMetaDataAnnotation(&gardenNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(seed.GetInfo().Spec.Provider.Zones, ","))
		}
		return nil
	}); err != nil {
		return err
	}

	// label kube-system namespace
	namespaceKubeSystem := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: metav1.NamespaceSystem}}
	log.Info("Labeling namespace", "namespaceName", namespaceKubeSystem.Name)
	patch := client.MergeFrom(namespaceKubeSystem.DeepCopy())
	metav1.SetMetaDataLabel(&namespaceKubeSystem.ObjectMeta, "role", metav1.NamespaceSystem)
	if err := seedClient.Patch(ctx, namespaceKubeSystem, patch); err != nil {
		return err
	}

	// replicate global monitoring secret (read from garden cluster) to the seed cluster's garden namespace
	globalMonitoringSecretGarden, ok := secrets[v1beta1constants.GardenRoleGlobalMonitoring]
	if !ok {
		return errors.New("global monitoring secret not found in seed namespace")
	}

	globalMonitoringSecretSeed := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "seed-" + globalMonitoringSecretGarden.Name,
			Namespace: r.GardenNamespace,
		},
	}

	log.Info("Replicating global monitoring secret to garden namespace in seed", "secret", client.ObjectKeyFromObject(globalMonitoringSecretGarden))
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, seedClient, globalMonitoringSecretSeed, func() error {
		globalMonitoringSecretSeed.Type = globalMonitoringSecretGarden.Type
		globalMonitoringSecretSeed.Data = globalMonitoringSecretGarden.Data
		globalMonitoringSecretSeed.Immutable = globalMonitoringSecretGarden.Immutable

		if _, ok := globalMonitoringSecretSeed.Data[secretsutils.DataKeySHA1Auth]; !ok {
			globalMonitoringSecretSeed.Data[secretsutils.DataKeySHA1Auth] = utils.CreateSHA1Secret(globalMonitoringSecretGarden.Data[secretsutils.DataKeyUserName], globalMonitoringSecretGarden.Data[secretsutils.DataKeyPassword])
		}

		return nil
	}); err != nil {
		return err
	}

	seedImages, err := imagevector.FindImages(
		r.ImageVector,
		[]string{
			images.ImageNameAlertmanager,
			images.ImageNameAlpine,
			images.ImageNameConfigmapReloader,
			images.ImageNameVali,
			images.ImageNameValiCurator,
			images.ImageNameTune2fs,
			images.ImageNamePlutono,
			images.ImageNamePrometheus,
		},
		imagevector.RuntimeVersion(kubernetesVersion.String()),
		imagevector.TargetVersion(kubernetesVersion.String()),
	)
	if err != nil {
		return err
	}

	// Deploy the CRDs in the seed cluster.
	log.Info("Deploying custom resource definitions")

	if hvpaEnabled {
		if err := kubernetesutils.DeleteObjects(ctx, seedClient,
			&vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-vpa", Namespace: r.GardenNamespace}},
			&vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "aggregate-prometheus-vpa", Namespace: r.GardenNamespace}},
		); err != nil {
			return err
		}

		if err := hvpa.NewCRD(applier).Deploy(ctx); err != nil {
			return err
		}
	}

	istioCRDs := istio.NewCRD(chartApplier)
	if err := istioCRDs.Deploy(ctx); err != nil {
		return err
	}

	if !seedIsGarden && vpaEnabled {
		if err := vpa.NewCRD(applier, nil).Deploy(ctx); err != nil {
			return err
		}
	}

	if err := fluentoperator.NewCRDs(applier).Deploy(ctx); err != nil {
		return err
	}

	if err := crds.NewExtensionsCRD(applier).Deploy(ctx); err != nil {
		return err
	}

	// When the seed is the garden cluster then gardener-resource-manager is reconciled by the gardener-operator.
	if !seedIsGarden {
		var defaultNotReadyTolerationSeconds, defaultUnreachableTolerationSeconds *int64
		if nodeToleration := r.Config.NodeToleration; nodeToleration != nil {
			defaultNotReadyTolerationSeconds = nodeToleration.DefaultNotReadyTolerationSeconds
			defaultUnreachableTolerationSeconds = nodeToleration.DefaultUnreachableTolerationSeconds
		}

		var additionalNetworkPolicyNamespaceSelectors []metav1.LabelSelector
		if config := r.Config.Controllers.NetworkPolicy; config != nil {
			additionalNetworkPolicyNamespaceSelectors = config.AdditionalNamespaceSelectors
		}

		// Deploy gardener-resource-manager first since it serves central functionality (e.g., projected token mount
		// webhook) which is required for all other components to start-up.
		gardenerResourceManager, err := sharedcomponent.NewRuntimeGardenerResourceManager(
			seedClient,
			r.GardenNamespace,
			kubernetesVersion,
			r.ImageVector,
			secretsManager,
			r.Config.LogLevel, r.Config.LogFormat,
			v1beta1constants.SecretNameCASeed,
			v1beta1constants.PriorityClassNameSeedSystemCritical,
			defaultNotReadyTolerationSeconds,
			defaultUnreachableTolerationSeconds,
			features.DefaultFeatureGate.Enabled(features.DefaultSeccompProfile),
			v1beta1helper.SeedSettingTopologyAwareRoutingEnabled(seed.GetInfo().Spec.Settings),
			features.DefaultFeatureGate.Enabled(features.FullNetworkPoliciesInRuntimeCluster),
			additionalNetworkPolicyNamespaceSelectors,
			seed.GetInfo().Spec.Provider.Zones,
		)
		if err != nil {
			return err
		}

		log.Info("Deploying and waiting for gardener-resource-manager to be healthy")
		if err := component.OpWait(gardenerResourceManager).Deploy(ctx); err != nil {
			return err
		}
	}

	// Deploy System Resources
	systemResources, err := defaultSystem(seedClient, seed, r.ImageVector, seed.GetInfo().Spec.Settings.ExcessCapacityReservation.Enabled, r.GardenNamespace)
	if err != nil {
		return err
	}

	if err := systemResources.Deploy(ctx); err != nil {
		return err
	}

	// Wait until required extensions are ready because they might be needed by following deployments
	if err := WaitUntilRequiredExtensionsReady(ctx, r.GardenClient, seed.GetInfo(), 5*time.Second, 1*time.Minute); err != nil {
		return err
	}

	// Fetch component-specific aggregate and central monitoring configuration
	var (
		aggregateScrapeConfigs                = strings.Builder{}
		aggregateMonitoringComponentFunctions = []component.AggregateMonitoringConfiguration{
			istio.AggregateMonitoringConfiguration,
		}

		centralScrapeConfigs                            = strings.Builder{}
		centralCAdvisorScrapeConfigMetricRelabelConfigs = strings.Builder{}
		centralMonitoringComponentFunctions             = []component.CentralMonitoringConfiguration{
			hvpa.CentralMonitoringConfiguration,
			kubestatemetrics.CentralMonitoringConfiguration,
		}
	)

	for _, componentFn := range aggregateMonitoringComponentFunctions {
		aggregateMonitoringConfig, err := componentFn()
		if err != nil {
			return err
		}

		for _, config := range aggregateMonitoringConfig.ScrapeConfigs {
			aggregateScrapeConfigs.WriteString(fmt.Sprintf("- %s\n", utils.Indent(config, 2)))
		}
	}

	for _, componentFn := range centralMonitoringComponentFunctions {
		centralMonitoringConfig, err := componentFn()
		if err != nil {
			return err
		}

		for _, config := range centralMonitoringConfig.ScrapeConfigs {
			centralScrapeConfigs.WriteString(fmt.Sprintf("- %s\n", utils.Indent(config, 2)))
		}

		for _, config := range centralMonitoringConfig.CAdvisorScrapeConfigMetricRelabelConfigs {
			centralCAdvisorScrapeConfigMetricRelabelConfigs.WriteString(fmt.Sprintf("- %s\n", utils.Indent(config, 2)))
		}
	}

	// Logging feature gate
	var (
		valiValues = map[string]interface{}{}

		inputs  []*fluentbitv1alpha2.ClusterInput
		filters []*fluentbitv1alpha2.ClusterFilter
		parsers []*fluentbitv1alpha2.ClusterParser
	)
	valiValues["enabled"] = loggingEnabled

	if loggingEnabled {
		// check if vali is disabled in gardenlet config
		if !gardenlethelper.IsValiEnabled(&r.Config) {
			valiValues["enabled"] = false
			if err := common.DeleteVali(ctx, seedClient, gardenNamespace.Name); err != nil {
				return err
			}
		} else {
			valiValues["authEnabled"] = false
			valiValues["storage"] = loggingConfig.Vali.Garden.Storage
			if err := ResizeOrDeleteValiDataVolumeIfStorageNotTheSame(ctx, log, seedClient, *loggingConfig.Vali.Garden.Storage); err != nil {
				return err
			}

			if hvpaEnabled {
				shootInfo := &corev1.ConfigMap{}
				maintenanceBegin := "220000-0000"
				maintenanceEnd := "230000-0000"
				if err := seedClient.Get(ctx, kubernetesutils.Key(metav1.NamespaceSystem, v1beta1constants.ConfigMapNameShootInfo), shootInfo); err != nil {
					if !apierrors.IsNotFound(err) {
						return err
					}
				} else {
					shootMaintenanceBegin, err := timewindow.ParseMaintenanceTime(shootInfo.Data["maintenanceBegin"])
					if err != nil {
						return err
					}
					maintenanceBegin = shootMaintenanceBegin.Add(1, 0, 0).Formatted()

					shootMaintenanceEnd, err := timewindow.ParseMaintenanceTime(shootInfo.Data["maintenanceEnd"])
					if err != nil {
						return err
					}
					maintenanceEnd = shootMaintenanceEnd.Add(1, 0, 0).Formatted()
				}

				valiValues["hvpa"] = map[string]interface{}{
					"enabled": true,
					"maintenanceTimeWindow": map[string]interface{}{
						"begin": maintenanceBegin,
						"end":   maintenanceEnd,
					},
				}

				currentResources, err := kubernetesutils.GetContainerResourcesInStatefulSet(ctx, seedClient, kubernetesutils.Key(r.GardenNamespace, v1beta1constants.StatefulSetNameVali))
				if err != nil {
					return err
				}
				if len(currentResources) != 0 && currentResources[v1beta1constants.StatefulSetNameVali] != nil {
					valiValues["resources"] = map[string]interface{}{
						// Copy requests only, effectively removing limits
						v1beta1constants.StatefulSetNameVali: &corev1.ResourceRequirements{
							Requests: currentResources[v1beta1constants.StatefulSetNameVali].Requests,
						},
					}
				}
			}

			valiValues["priorityClassName"] = v1beta1constants.PriorityClassNameSeedSystem600
		}

		componentsFunctions := []component.CentralLoggingConfiguration{
			// journald components
			kubelet.CentralLoggingConfiguration,
			docker.CentralLoggingConfiguration,
			containerd.CentralLoggingConfiguration,
			downloader.CentralLoggingConfiguration,
			// seed system components
			extensions.CentralLoggingConfiguration,
			dependencywatchdog.CentralLoggingConfiguration,
			resourcemanager.CentralLoggingConfiguration,
			monitoring.CentralLoggingConfiguration,
			vali.CentralLoggingConfiguration,
			// shoot control plane components
			etcd.CentralLoggingConfiguration,
			clusterautoscaler.CentralLoggingConfiguration,
			kubeapiserver.CentralLoggingConfiguration,
			kubescheduler.CentralLoggingConfiguration,
			kubecontrollermanager.CentralLoggingConfiguration,
			kubestatemetrics.CentralLoggingConfiguration,
			hvpa.CentralLoggingConfiguration,
			vpa.CentralLoggingConfiguration,
			vpnseedserver.CentralLoggingConfiguration,
			// shoot system components
			coredns.CentralLoggingConfiguration,
			kubeproxy.CentralLoggingConfiguration,
			metricsserver.CentralLoggingConfiguration,
			nodeexporter.CentralLoggingConfiguration,
			nodeproblemdetector.CentralLoggingConfiguration,
			vpnshoot.CentralLoggingConfiguration,
			// shoot addon components
			kubernetesdashboard.CentralLoggingConfiguration,
			nginxingressshoot.CentralLoggingConfiguration,
		}

		if gardenlethelper.IsEventLoggingEnabled(&r.Config) {
			componentsFunctions = append(componentsFunctions, eventlogger.CentralLoggingConfiguration)
		}

		// Fetch component specific logging configurations
		for _, componentFn := range componentsFunctions {
			loggingConfig, err := componentFn()
			if err != nil {
				return err
			}

			if len(loggingConfig.Inputs) > 0 {
				inputs = append(inputs, loggingConfig.Inputs...)
			}

			if len(loggingConfig.Filters) > 0 {
				filters = append(filters, loggingConfig.Filters...)
			}

			if len(loggingConfig.Parsers) > 0 {
				parsers = append(parsers, loggingConfig.Parsers...)
			}
		}
	} else {
		if err := common.DeleteVali(ctx, seedClient, v1beta1constants.GardenNamespace); err != nil {
			return err
		}
	}

	// Monitoring resource values
	monitoringResources := map[string]interface{}{
		"prometheus":           map[string]interface{}{},
		"aggregate-prometheus": map[string]interface{}{},
	}

	if hvpaEnabled {
		for resource := range monitoringResources {
			currentResources, err := kubernetesutils.GetContainerResourcesInStatefulSet(ctx, seedClient, kubernetesutils.Key(r.GardenNamespace, resource))
			if err != nil {
				return err
			}
			if len(currentResources) != 0 && currentResources["prometheus"] != nil {
				monitoringResources[resource] = map[string]interface{}{
					"prometheus": currentResources["prometheus"],
				}
			}
		}
	}

	// AlertManager configuration
	alertManagerConfig := map[string]interface{}{
		"storage": seed.GetValidVolumeSize("1Gi"),
	}

	if alertingSMTPSecret, ok := secrets[v1beta1constants.GardenRoleAlerting]; ok && string(alertingSMTPSecret.Data["auth_type"]) == "smtp" {
		emailConfig := map[string]interface{}{
			"to":            string(alertingSMTPSecret.Data["to"]),
			"from":          string(alertingSMTPSecret.Data["from"]),
			"smarthost":     string(alertingSMTPSecret.Data["smarthost"]),
			"auth_username": string(alertingSMTPSecret.Data["auth_username"]),
			"auth_identity": string(alertingSMTPSecret.Data["auth_identity"]),
			"auth_password": string(alertingSMTPSecret.Data["auth_password"]),
		}
		alertManagerConfig["enabled"] = true
		alertManagerConfig["emailConfigs"] = []map[string]interface{}{emailConfig}
	} else {
		alertManagerConfig["enabled"] = false
		if err := common.DeleteAlertmanager(ctx, seedClient, r.GardenNamespace); err != nil {
			return err
		}
	}

	var (
		applierOptions          = kubernetes.CopyApplierOptions(kubernetes.DefaultMergeFuncs)
		retainStatusInformation = func(new, old *unstructured.Unstructured) {
			// Apply status from old Object to retain status information
			new.Object["status"] = old.Object["status"]
		}
		plutonoHost    = seed.GetIngressFQDN(plutonoPrefix)
		prometheusHost = seed.GetIngressFQDN(prometheusPrefix)
	)

	applierOptions[vpaGK] = retainStatusInformation
	applierOptions[hvpaGK] = retainStatusInformation
	applierOptions[issuerGK] = retainStatusInformation

	wildcardCert, err := gardenerutils.GetWildcardCertificate(ctx, seedClient)
	if err != nil {
		return err
	}

	var (
		plutonoIngressTLSSecretName    string
		prometheusIngressTLSSecretName string
	)

	if wildcardCert != nil {
		plutonoIngressTLSSecretName = wildcardCert.GetName()
		prometheusIngressTLSSecretName = wildcardCert.GetName()
	} else {
		plutonoIngressTLSSecret, err := secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
			Name:                        "plutono-tls",
			CommonName:                  "plutono",
			Organization:                []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:                    []string{seed.GetIngressFQDN(plutonoPrefix)},
			CertType:                    secretsutils.ServerCert,
			Validity:                    pointer.Duration(ingressTLSCertificateValidity),
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCASeed))
		if err != nil {
			return err
		}

		prometheusIngressTLSSecret, err := secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
			Name:                        "aggregate-prometheus-tls",
			CommonName:                  "prometheus",
			Organization:                []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:                    []string{seed.GetIngressFQDN(prometheusPrefix)},
			CertType:                    secretsutils.ServerCert,
			Validity:                    pointer.Duration(ingressTLSCertificateValidity),
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCASeed))
		if err != nil {
			return err
		}

		plutonoIngressTLSSecretName = plutonoIngressTLSSecret.Name
		prometheusIngressTLSSecretName = prometheusIngressTLSSecret.Name
	}

	imageVectorOverwrites := make(map[string]string, len(r.ComponentImageVectors))
	for name, data := range r.ComponentImageVectors {
		imageVectorOverwrites[name] = data
	}

	anySNIInUse, err := kubeapiserverexposure.AnyDeployedSNI(ctx, seedClient)
	if err != nil {
		return err
	}
	sniEnabledOrInUse := anySNIInUse || features.DefaultFeatureGate.Enabled(features.APIServerSNI)

	seedIsOriginOfClusterIdentity, err := clusteridentity.IsClusterIdentityEmptyOrFromOrigin(ctx, seedClient, v1beta1constants.ClusterIdentityOriginSeed)
	if err != nil {
		return err
	}

	if err := cleanupOrphanExposureClassHandlerResources(ctx, log, seedClient, r.Config.ExposureClassHandlers, seed.GetInfo().Spec.Provider.Zones); err != nil {
		return err
	}

	ingressClass, err := gardenerutils.ComputeNginxIngressClassForSeed(seed.GetInfo(), seed.GetInfo().Status.KubernetesVersion)
	if err != nil {
		return err
	}

	values := kubernetes.Values(map[string]interface{}{
		"global": map[string]interface{}{
			"ingressClass": ingressClass,
			"images":       imagevector.ImageMapToValues(seedImages),
		},
		"prometheus": map[string]interface{}{
			"deployAllowAllAccessNetworkPolicy": !features.DefaultFeatureGate.Enabled(features.FullNetworkPoliciesInRuntimeCluster),
			"resources":                         monitoringResources["prometheus"],
			"storage":                           seed.GetValidVolumeSize("10Gi"),
			"additionalScrapeConfigs":           centralScrapeConfigs.String(),
			"additionalCAdvisorScrapeConfigMetricRelabelConfigs": centralCAdvisorScrapeConfigMetricRelabelConfigs.String(),
		},
		"aggregatePrometheus": map[string]interface{}{
			"resources":               monitoringResources["aggregate-prometheus"],
			"storage":                 seed.GetValidVolumeSize("20Gi"),
			"seed":                    seed.GetInfo().Name,
			"hostName":                prometheusHost,
			"secretName":              prometheusIngressTLSSecretName,
			"additionalScrapeConfigs": aggregateScrapeConfigs.String(),
		},
		"plutono": map[string]interface{}{
			"hostName":   plutonoHost,
			"secretName": plutonoIngressTLSSecretName,
		},
		"vali":         valiValues,
		"alertmanager": alertManagerConfig,
		"hvpa": map[string]interface{}{
			"enabled": hvpaEnabled,
		},
		"istio": map[string]interface{}{
			"enabled": true,
		},
		"ingress": map[string]interface{}{
			"authSecretName": globalMonitoringSecretSeed.Name,
		},
	})

	// Delete Grafana artifacts.
	if err := common.DeleteGrafana(ctx, r.SeedClientSet, r.GardenNamespace); err != nil {
		return err
	}

	// Delete Grafana ingress which doesn't have the component label in the garden namespace.
	if err := seedClient.Delete(
		ctx,
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "grafana",
				Namespace: r.GardenNamespace,
			}},
	); client.IgnoreNotFound(err) != nil {
		return err
	}

	// TODO(rickardsjp, istvanballok): Remove in release v1.77 once the Loki to Vali migration is complete.
	if exists, err := common.LokiPvcExists(ctx, seedClient, r.GardenNamespace, log); err != nil {
		return err
	} else if exists {
		if err := common.DeleteLokiRetainPvc(ctx, seedClient, r.GardenNamespace, log); err != nil {
			return err
		}
		if err := common.RenameLokiPvcToValiPvc(ctx, seedClient, r.GardenNamespace, log); err != nil {
			return err
		}
	}

	if err := chartApplier.Apply(ctx, filepath.Join(r.ChartsPath, seedBootstrapChartName), r.GardenNamespace, seedBootstrapChartName, values, applierOptions); err != nil {
		return err
	}

	if !v1beta1helper.SeedUsesNginxIngressController(seed.GetInfo()) {
		nginxIngress := nginxingress.New(seedClient, r.GardenNamespace, nginxingress.Values{})

		if err := component.OpDestroyAndWait(nginxIngress).Destroy(ctx); err != nil {
			return err
		}
	}

	if err := migrateIngressClassForShootIngresses(ctx, r.GardenClient, seedClient, seed, ingressClass, kubernetesVersion); err != nil {
		return err
	}

	// setup for flow graph
	var dnsRecord component.DeployMigrateWaiter

	istio, err := defaultIstio(seedClient, r.ImageVector, chartRenderer, seed, &r.Config, sniEnabledOrInUse, seedIsGarden)
	if err != nil {
		return err
	}
	dwdWeeder, dwdProber, err := defaultDependencyWatchdogs(seedClient, kubernetesVersion, r.ImageVector, seed.GetInfo().Spec.Settings, r.GardenNamespace)
	if err != nil {
		return err
	}
	vpnAuthzServer, err := defaultVPNAuthzServer(seedClient, kubernetesVersion, r.ImageVector, r.GardenNamespace)
	if err != nil {
		return err
	}

	if features.DefaultFeatureGate.Enabled(features.FullNetworkPoliciesInRuntimeCluster) {
		if err := kubernetesutils.DeleteObject(ctx, seedClient, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-seed-prometheus", Namespace: r.GardenNamespace}}); err != nil {
			return err
		}
	}

	var (
		g = flow.NewGraph("Seed cluster creation")
		_ = g.Add(flow.Task{
			Name: "Deploying Istio",
			Fn:   istio.Deploy,
		})
		nginxLBReady = g.Add(flow.Task{
			Name: "Waiting until nginx ingress LoadBalancer is ready",
			Fn: func(ctx context.Context) error {
				dnsRecord, err = waitForNginxIngressServiceAndGetDNSComponent(ctx, log, seed, r.GardenClient, seedClient, r.ImageVector, kubernetesVersion, ingressClass, r.GardenNamespace)
				return err
			},
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying managed ingress DNS record",
			Fn:           flow.TaskFn(func(ctx context.Context) error { return deployDNSResources(ctx, dnsRecord) }).DoIf(v1beta1helper.SeedWantsManagedIngress(seed.GetInfo())),
			Dependencies: flow.NewTaskIDs(nginxLBReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying managed ingress DNS record (if existing)",
			Fn:           flow.TaskFn(func(ctx context.Context) error { return destroyDNSResources(ctx, dnsRecord) }).DoIf(!v1beta1helper.SeedWantsManagedIngress(seed.GetInfo())),
			Dependencies: flow.NewTaskIDs(nginxLBReady),
		})
		_ = g.Add(flow.Task{
			Name: "Deploying cluster-autoscaler",
			Fn:   clusterautoscaler.NewBootstrapper(seedClient, r.GardenNamespace).Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying dependency-watchdog-weeder",
			Fn:   dwdWeeder.Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying dependency-watchdog-prober",
			Fn:   dwdProber.Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying VPN authorization server",
			Fn:   vpnAuthzServer.Deploy,
		})
	)

	// Use the managed resource for cluster-identity only if there is no cluster-identity config map in kube-system namespace from a different origin than seed.
	// This prevents gardenlet from deleting the config map accidentally on seed deletion when it was created by a different party (gardener-apiserver or shoot).
	if seedIsOriginOfClusterIdentity {
		_ = g.Add(flow.Task{
			Name: "Deploying cluster-identity",
			Fn:   clusteridentity.NewForSeed(seedClient, r.GardenNamespace, *seed.GetInfo().Status.ClusterIdentity).Deploy,
		})
	}

	// When the seed is the garden cluster then the following components are reconciled by the gardener-operator.
	if !seedIsGarden {
		vpa, err := sharedcomponent.NewVerticalPodAutoscaler(
			seedClient,
			r.GardenNamespace,
			kubernetesVersion,
			r.ImageVector,
			secretsManager,
			vpaEnabled,
			v1beta1constants.SecretNameCASeed,
			v1beta1constants.PriorityClassNameSeedSystem800,
			v1beta1constants.PriorityClassNameSeedSystem700,
			v1beta1constants.PriorityClassNameSeedSystem700,
		)
		if err != nil {
			return err
		}

		hvpa, err := sharedcomponent.NewHVPA(
			seedClient,
			r.GardenNamespace,
			kubernetesVersion,
			r.ImageVector,
			hvpaEnabled,
			v1beta1constants.PriorityClassNameSeedSystem700,
		)
		if err != nil {
			return err
		}

		etcdDruid, err := sharedcomponent.NewEtcdDruid(
			seedClient,
			r.GardenNamespace,
			kubernetesVersion,
			r.ImageVector,
			r.ComponentImageVectors,
			r.Config.ETCDConfig,
			v1beta1constants.PriorityClassNameSeedSystem800,
		)
		if err != nil {
			return err
		}

		// TODO(Kristian-ZH): Remove this in the next releases
		if err := CleanupOldFluentBit(ctx, seedClient); err != nil {
			return err
		}

		fluentOperatorCustomResources, err := sharedcomponent.NewFluentOperatorCustomResources(
			seedClient,
			r.GardenNamespace,
			kubernetesVersion,
			r.ImageVector,
			loggingEnabled,
			v1beta1constants.PriorityClassNameSeedSystem600,
			inputs,
			filters,
			parsers,
		)
		if err != nil {
			return err
		}

		fluentOperator, err := sharedcomponent.NewFluentOperator(
			seedClient,
			r.GardenNamespace,
			kubernetesVersion,
			r.ImageVector,
			loggingEnabled,
			v1beta1constants.PriorityClassNameSeedSystem600,
		)
		if err != nil {
			return err
		}

		kubeStateMetrics, err := sharedcomponent.NewKubeStateMetrics(
			seedClient,
			r.GardenNamespace,
			kubernetesVersion,
			r.ImageVector,
			v1beta1constants.PriorityClassNameSeedSystem600,
		)
		if err != nil {
			return err
		}

		var (
			_ = g.Add(flow.Task{
				Name: "Deploying Kubernetes vertical pod autoscaler",
				Fn:   vpa.Deploy,
			})
			_ = g.Add(flow.Task{
				Name: "Deploying HVPA controller",
				Fn:   hvpa.Deploy,
			})
			_ = g.Add(flow.Task{
				Name: "Deploying ETCD Druid",
				Fn:   etcdDruid.Deploy,
			})
			_ = g.Add(flow.Task{
				Name: "Deploying kube-state-metrics",
				Fn:   kubeStateMetrics.Deploy,
			})
			reconcileFluentOperatorResources = g.Add(flow.Task{
				Name: "Deploying Fluent Operator resources",
				Fn:   component.OpWait(fluentOperatorCustomResources).Deploy,
			})
			_ = g.Add(flow.Task{
				Name:         "Deploying Fluent Operator",
				Fn:           component.OpWait(fluentOperator).Deploy,
				Dependencies: flow.NewTaskIDs(reconcileFluentOperatorResources),
			})
		)
	}

	kubeAPIServerService := kubeapiserverexposure.NewInternalNameService(seedClient, r.GardenNamespace)
	if wildcardCert != nil {
		kubeAPIServerIngress := kubeapiserverexposure.NewIngress(seedClient, r.GardenNamespace, kubeapiserverexposure.IngressValues{
			Host:             seed.GetIngressFQDN(kubeAPIServerPrefix),
			IngressClassName: &ingressClass,
			ServiceName:      v1beta1constants.DeploymentNameKubeAPIServer,
			TLSSecretName:    &wildcardCert.Name,
		})
		var (
			_ = g.Add(flow.Task{
				Name: "Deploying kube-apiserver service",
				Fn:   kubeAPIServerService.Deploy,
			})
			_ = g.Add(flow.Task{
				Name: "Deploying kube-apiserver ingress",
				Fn:   kubeAPIServerIngress.Deploy,
			})
		)
	} else {
		kubeAPIServerIngress := kubeapiserverexposure.NewIngress(seedClient, r.GardenNamespace, kubeapiserverexposure.IngressValues{})
		var (
			_ = g.Add(flow.Task{
				Name: "Destroying kube-apiserver service",
				Fn:   kubeAPIServerService.Destroy,
			})
			_ = g.Add(flow.Task{
				Name: "Destroying kube-apiserver ingress",
				Fn:   kubeAPIServerIngress.Destroy,
			})
		)
	}

	if err := g.Compile().Run(ctx, flow.Opts{Log: log}); err != nil {
		return flow.Errors(err)
	}

	return secretsManager.Cleanup(ctx)
}

func deployBackupBucketInGarden(ctx context.Context, k8sGardenClient client.Client, seed *gardencorev1beta1.Seed) error {
	// By default, we assume the seed.Spec.Backup.Provider matches the seed.Spec.Provider.Type as per the validation logic.
	// However, if the backup region is specified we take it.
	region := seed.Spec.Provider.Region
	if seed.Spec.Backup.Region != nil {
		region = *seed.Spec.Backup.Region
	}

	backupBucket := &gardencorev1beta1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(seed.UID),
		},
	}

	ownerRef := metav1.NewControllerRef(seed, gardencorev1beta1.SchemeGroupVersion.WithKind("Seed"))

	_, err := controllerutils.CreateOrGetAndStrategicMergePatch(ctx, k8sGardenClient, backupBucket, func() error {
		backupBucket.OwnerReferences = []metav1.OwnerReference{*ownerRef}
		backupBucket.Spec = gardencorev1beta1.BackupBucketSpec{
			Provider: gardencorev1beta1.BackupBucketProvider{
				Type:   seed.Spec.Backup.Provider,
				Region: region,
			},
			ProviderConfig: seed.Spec.Backup.ProviderConfig,
			SecretRef: corev1.SecretReference{
				Name:      seed.Spec.Backup.SecretRef.Name,
				Namespace: seed.Spec.Backup.SecretRef.Namespace,
			},
			SeedName: &seed.Name, // In future this will be moved to gardener-scheduler.
		}
		return nil
	})
	return err
}

// ResizeOrDeleteValiDataVolumeIfStorageNotTheSame updates the garden Vali PVC if passed storage value is not the same as the current one.
// Caution: If the passed storage capacity is less than the current one the existing PVC and its PV will be deleted.
func ResizeOrDeleteValiDataVolumeIfStorageNotTheSame(ctx context.Context, log logr.Logger, k8sClient client.Client, newStorageQuantity resource.Quantity) error {
	// Check if we need resizing
	pvc := &corev1.PersistentVolumeClaim{}
	if err := k8sClient.Get(ctx, kubernetesutils.Key(v1beta1constants.GardenNamespace, "vali-vali-0"), pvc); err != nil {
		return client.IgnoreNotFound(err)
	}

	log = log.WithValues("persistentVolumeClaim", client.ObjectKeyFromObject(pvc))

	storageCmpResult := newStorageQuantity.Cmp(*pvc.Spec.Resources.Requests.Storage())
	if storageCmpResult == 0 {
		return nil
	}

	statefulSetKey := client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: v1beta1constants.StatefulSetNameVali}
	log.Info("Scaling StatefulSet to zero in order to detach PVC", "statefulSet", statefulSetKey)
	if err := kubernetes.ScaleStatefulSetAndWaitUntilScaled(ctx, k8sClient, statefulSetKey, 0); client.IgnoreNotFound(err) != nil {
		return err
	}

	switch {
	case storageCmpResult > 0:
		patch := client.MergeFrom(pvc.DeepCopy())
		pvc.Spec.Resources.Requests = corev1.ResourceList{
			corev1.ResourceStorage: newStorageQuantity,
		}
		log.Info("Patching storage of PVC", "storage", newStorageQuantity.String())
		if err := k8sClient.Patch(ctx, pvc, patch); client.IgnoreNotFound(err) != nil {
			return err
		}
	case storageCmpResult < 0:
		log.Info("Deleting PVC because size needs to be reduced")
		if err := client.IgnoreNotFound(k8sClient.Delete(ctx, pvc)); err != nil {
			return err
		}
	}

	valiSts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.StatefulSetNameVali, Namespace: v1beta1constants.GardenNamespace}}
	return client.IgnoreNotFound(k8sClient.Delete(ctx, valiSts))
}

func cleanupOrphanExposureClassHandlerResources(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	exposureClassHandlers []config.ExposureClassHandler,
	zones []string,
) error {
	// Remove ordinary, orphaned istio exposure class namespaces
	exposureClassHandlerNamespaces := &corev1.NamespaceList{}
	if err := c.List(ctx, exposureClassHandlerNamespaces, client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExposureClassHandler}); err != nil {
		return err
	}

	for _, namespace := range exposureClassHandlerNamespaces.Items {
		if err := cleanupOrphanIstioNamespace(ctx, log, c, namespace, true, func() bool {
			for _, handler := range exposureClassHandlers {
				if *handler.SNI.Ingress.Namespace == namespace.Name {
					return true
				}
			}
			return false
		}); err != nil {
			return err
		}
	}

	// Remove zonal, orphaned istio exposure class namespaces
	zonalExposureClassHandlerNamespaces := &corev1.NamespaceList{}
	if err := c.List(ctx, zonalExposureClassHandlerNamespaces, client.MatchingLabelsSelector{
		Selector: labels.NewSelector().Add(utils.MustNewRequirement(v1beta1constants.GardenRole, selection.Exists)).Add(utils.MustNewRequirement(v1beta1constants.LabelExposureClassHandlerName, selection.Exists)),
	}); err != nil {
		return err
	}

	zoneSet := sets.New(zones...)
	for _, namespace := range zonalExposureClassHandlerNamespaces.Items {
		if ok, zone := sharedcomponent.IsZonalIstioExtension(namespace.Labels); ok {
			if err := cleanupOrphanIstioNamespace(ctx, log, c, namespace, true, func() bool {
				if !zoneSet.Has(zone) {
					return false
				}
				for _, handler := range exposureClassHandlers {
					if handler.Name == namespace.Labels[v1beta1constants.LabelExposureClassHandlerName] {
						return true
					}
				}
				return false
			}); err != nil {
				return err
			}
		}
	}

	// Remove zonal, orphaned istio default namespaces
	zonalIstioNamespaces := &corev1.NamespaceList{}
	if err := c.List(ctx, zonalIstioNamespaces, client.MatchingLabelsSelector{
		Selector: labels.NewSelector().Add(utils.MustNewRequirement(istio.DefaultZoneKey, selection.Exists)),
	}); err != nil {
		return err
	}

	for _, namespace := range zonalIstioNamespaces.Items {
		if ok, zone := sharedcomponent.IsZonalIstioExtension(namespace.Labels); ok {
			if err := cleanupOrphanIstioNamespace(ctx, log, c, namespace, false, func() bool {
				return zoneSet.Has(zone)
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func cleanupOrphanIstioNamespace(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	namespace corev1.Namespace,
	needsHandler bool,
	isAliveFunc func() bool,
) error {
	log = log.WithValues("namespace", client.ObjectKeyFromObject(&namespace))

	if isAlive := isAliveFunc(); isAlive {
		return nil
	}
	log.Info("Namespace is orphan as there is no ExposureClass handler in the gardenlet configuration anymore or the zone was removed")

	// Determine the corresponding handler name to the ExposureClass handler resources.
	handlerName, ok := namespace.Labels[v1beta1constants.LabelExposureClassHandlerName]
	if !ok && needsHandler {
		log.Info("Cannot delete ExposureClass handler resources as the corresponding handler is unknown and it is not save to remove them")
		return nil
	}

	gatewayList := &istiov1beta1.GatewayList{}
	if err := c.List(ctx, gatewayList); err != nil {
		return err
	}

	for _, gateway := range gatewayList.Items {
		if gateway.Name != v1beta1constants.DeploymentNameKubeAPIServer && gateway.Name != v1beta1constants.DeploymentNameVPNSeedServer {
			continue
		}
		if needsHandler {
			// Check if the gateway still selects the ExposureClass handler ingress gateway.
			if value, ok := gateway.Spec.Selector[v1beta1constants.LabelExposureClassHandlerName]; ok && value == handlerName {
				log.Info("Resources of ExposureClass handler cannot be deleted as they are still in use", "exposureClassHandler", handlerName)
				return nil
			}
		} else {
			_, zone := sharedcomponent.IsZonalIstioExtension(namespace.Labels)
			if value, ok := gateway.Spec.Selector[istio.DefaultZoneKey]; ok && strings.HasSuffix(value, zone) {
				log.Info("Resources of default zonal istio handler cannot be deleted as they are still in use", "zone", zone)
				return nil
			}
		}
	}

	// ExposureClass handler is orphan and not used by any Shoots anymore
	// therefore it is save to clean it up.
	log.Info("Delete orphan ExposureClass handler namespace")
	if err := c.Delete(ctx, &namespace); client.IgnoreNotFound(err) != nil {
		return err
	}

	return nil
}

// WaitUntilLoadBalancerIsReady is an alias for kubernetesutils.WaitUntilLoadBalancerIsReady. Exposed for tests.
var WaitUntilLoadBalancerIsReady = kubernetesutils.WaitUntilLoadBalancerIsReady

func waitForNginxIngressServiceAndGetDNSComponent(
	ctx context.Context,
	log logr.Logger,
	seed *seedpkg.Seed,
	gardenClient, seedClient client.Client,
	imageVector imagevector.ImageVector,
	kubernetesVersion *semver.Version,
	ingressClass string,
	gardenNamespaceName string,
) (
	component.DeployMigrateWaiter,
	error,
) {
	secretData, err := getDNSProviderSecretData(ctx, gardenClient, seed.GetInfo())
	if err != nil {
		return nil, err
	}

	var ingressLoadBalancerAddress string
	if v1beta1helper.SeedUsesNginxIngressController(seed.GetInfo()) {
		providerConfig, err := getConfig(seed.GetInfo())
		if err != nil {
			return nil, err
		}

		nginxIngress, err := defaultNginxIngress(seedClient, imageVector, kubernetesVersion, ingressClass, providerConfig, seed.GetLoadBalancerServiceAnnotations(), gardenNamespaceName)
		if err != nil {
			return nil, err
		}

		if err = component.OpWait(nginxIngress).Deploy(ctx); err != nil {
			return nil, err
		}

		ingressLoadBalancerAddress, err = WaitUntilLoadBalancerIsReady(
			ctx,
			log,
			seedClient,
			gardenNamespaceName,
			"nginx-ingress-controller",
			time.Minute,
		)
		if err != nil {
			return nil, err
		}
	}

	return getManagedIngressDNSRecord(log, seedClient, gardenNamespaceName, seed.GetInfo().Spec.DNS, secretData, seed.GetIngressFQDN("*"), ingressLoadBalancerAddress), nil
}

// CleanupOldFluentBit deletes all old fluent-bit resources which are not installed by the fluent-operator.
func CleanupOldFluentBit(ctx context.Context, seedClient client.Client) error {
	// Resources which does not duplicate these from the operators
	uniqueResource := []client.Object{
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit-read"}},
		&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit-read"}},
	}

	// Resources whose names duplicate these from the operators
	fluentBitDaemonSet := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit", Namespace: v1beta1constants.GardenNamespace}}
	fluentBitService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit", Namespace: v1beta1constants.GardenNamespace}}
	fluentBitServiceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit", Namespace: v1beta1constants.GardenNamespace}}

	if err := seedClient.Get(ctx, client.ObjectKeyFromObject(fluentBitDaemonSet), fluentBitDaemonSet); client.IgnoreNotFound(err) != nil {
		return err
	}
	if err := seedClient.Get(ctx, client.ObjectKeyFromObject(fluentBitService), fluentBitService); client.IgnoreNotFound(err) != nil {
		return err
	}
	if err := seedClient.Get(ctx, client.ObjectKeyFromObject(fluentBitServiceAccount), fluentBitServiceAccount); client.IgnoreNotFound(err) != nil {
		return err
	}

	if !isOwnedByFluentOperator(fluentBitDaemonSet) {
		if err := client.IgnoreNotFound(seedClient.Delete(ctx, fluentBitDaemonSet)); err != nil {
			return err
		}
	}

	if !isOwnedByFluentOperator(fluentBitService) {
		if err := client.IgnoreNotFound(seedClient.Delete(ctx, fluentBitService)); err != nil {
			return err
		}
	}

	if !isOwnedByFluentOperator(fluentBitServiceAccount) {
		if err := client.IgnoreNotFound(seedClient.Delete(ctx, fluentBitServiceAccount)); err != nil {
			return err
		}
	}

	return kubernetesutils.DeleteObjects(ctx, seedClient, uniqueResource...)
}

func isOwnedByFluentOperator(obj client.Object) bool {
	for _, ownerReference := range obj.GetOwnerReferences() {
		if ownerReference.Kind == "FluentBit" && ownerReference.APIVersion == "fluentbit.fluent.io/v1alpha2" {
			return true
		}
	}
	return false
}

// WaitUntilRequiredExtensionsReady checks and waits until all required extensions for a seed exist and are ready.
func WaitUntilRequiredExtensionsReady(ctx context.Context, gardenClient client.Client, seed *gardencorev1beta1.Seed, interval, timeout time.Duration) error {
	return retry.UntilTimeout(ctx, interval, timeout, func(ctx context.Context) (done bool, err error) {
		if err := gardenerutils.RequiredExtensionsReady(ctx, gardenClient, seed.Name, gardenerutils.ComputeRequiredExtensionsForSeed(seed)); err != nil {
			return retry.MinorError(err)
		}

		return retry.Ok()
	})
}
