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

package botanist

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/executor"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	nodelocaldnsconstants "github.com/gardener/gardener/pkg/component/nodelocaldns/constants"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// SecretLabelKeyManagedResource is a key for a label on a secret with the value 'managed-resource'.
const SecretLabelKeyManagedResource = "managed-resource"

// DefaultOperatingSystemConfig creates the default deployer for the OperatingSystemConfig custom resource.
func (b *Botanist) DefaultOperatingSystemConfig() (operatingsystemconfig.Interface, error) {
	oscImages, err := imagevectorutils.FindImages(imagevector.ImageVector(), []string{imagevector.ImageNameHyperkube, imagevector.ImageNamePauseContainer, imagevector.ImageNameValitail, imagevector.ImageNameGardenerNodeAgent}, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	clusterDNSAddress := b.Shoot.Networks.CoreDNS.String()
	if b.Shoot.NodeLocalDNSEnabled && b.Shoot.IPVSEnabled() {
		// If IPVS is enabled then instruct the kubelet to create pods resolving DNS to the `nodelocaldns` network
		// interface link-local ip address. For more information checkout the usage documentation under
		// https://kubernetes.io/docs/tasks/administer-cluster/nodelocaldns/.
		clusterDNSAddress = nodelocaldnsconstants.IPVSAddress
	}

	valitailEnabled, valiIngressHost := false, ""
	if b.isShootNodeLoggingEnabled() {
		valitailEnabled, valiIngressHost = true, b.ComputeValiHost()
	}

	return operatingsystemconfig.New(
		b.Logger,
		b.SeedClientSet.Client(),
		b.SecretsManager,
		&operatingsystemconfig.Values{
			Namespace:         b.Shoot.SeedNamespace,
			KubernetesVersion: b.Shoot.KubernetesVersion,
			Workers:           b.Shoot.GetInfo().Spec.Provider.Workers,
			OriginalValues: operatingsystemconfig.OriginalValues{
				ClusterDNSAddress:   clusterDNSAddress,
				ClusterDomain:       gardencorev1beta1.DefaultDomain,
				Images:              oscImages,
				KubeletConfig:       b.Shoot.GetInfo().Spec.Kubernetes.Kubelet,
				MachineTypes:        b.Shoot.CloudProfile.Spec.MachineTypes,
				SSHAccessEnabled:    v1beta1helper.ShootEnablesSSHAccess(b.Shoot.GetInfo()),
				ValitailEnabled:     valitailEnabled,
				ValiIngressHostName: valiIngressHost,
				NodeLocalDNSEnabled: v1beta1helper.IsNodeLocalDNSEnabled(b.Shoot.GetInfo().Spec.SystemComponents),
				SyncJitterPeriod:    b.Shoot.OSCSyncJitterPeriod,
			},
		},
		operatingsystemconfig.DefaultInterval,
		operatingsystemconfig.DefaultSevereThreshold,
		operatingsystemconfig.DefaultTimeout,
	), nil
}

// DeployOperatingSystemConfig deploys the OperatingSystemConfig custom resource and triggers the restore operation in
// case the Shoot is in the restore phase of the control plane migration.
func (b *Botanist) DeployOperatingSystemConfig(ctx context.Context) error {
	clusterCASecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	b.Shoot.Components.Extensions.OperatingSystemConfig.SetAPIServerURL(fmt.Sprintf("https://%s", b.Shoot.ComputeOutOfClusterAPIServerAddress(true)))
	b.Shoot.Components.Extensions.OperatingSystemConfig.SetCABundle(b.getOperatingSystemConfigCABundle(clusterCASecret.Data[secretsutils.DataKeyCertificateBundle]))

	if v1beta1helper.ShootEnablesSSHAccess(b.Shoot.GetInfo()) {
		sshKeypairSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameSSHKeyPair)
		if !found {
			return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameSSHKeyPair)
		}
		publicKeys := []string{string(sshKeypairSecret.Data[secretsutils.DataKeySSHAuthorizedKeys])}

		if sshKeypairSecretOld, found := b.SecretsManager.Get(v1beta1constants.SecretNameSSHKeyPair, secretsmanager.Old); found {
			publicKeys = append(publicKeys, string(sshKeypairSecretOld.Data[secretsutils.DataKeySSHAuthorizedKeys]))
		}

		b.Shoot.Components.Extensions.OperatingSystemConfig.SetSSHPublicKeys(publicKeys)
	}

	if b.IsRestorePhase() {
		return b.Shoot.Components.Extensions.OperatingSystemConfig.Restore(ctx, b.Shoot.GetShootState())
	}

	return b.Shoot.Components.Extensions.OperatingSystemConfig.Deploy(ctx)
}

func (b *Botanist) getOperatingSystemConfigCABundle(clusterCABundle []byte) *string {
	var caBundle string

	if cloudProfileCaBundle := b.Shoot.CloudProfile.Spec.CABundle; cloudProfileCaBundle != nil {
		caBundle = *cloudProfileCaBundle
	}

	if len(clusterCABundle) != 0 {
		caBundle = fmt.Sprintf("%s\n%s", caBundle, clusterCABundle)
	}

	if caBundle == "" {
		return nil
	}

	return &caBundle
}

// CloudConfigExecutionManagedResourceName is a constant for the name of a ManagedResource in the seed cluster in the
// shoot namespace which contains the cloud config user data execution script.
const CloudConfigExecutionManagedResourceName = "shoot-cloud-config-execution"

// exposed for testing
var (
	// ExecutorScriptFn is a function for computing the cloud config user data executor script.
	ExecutorScriptFn = executor.Script
	// DownloaderGenerateRBACResourcesDataFn is a function for generating the RBAC resources data map for the cloud
	// config user data executor scripts downloader.
	DownloaderGenerateRBACResourcesDataFn = downloader.GenerateRBACResourcesData
)

// DeployManagedResourceForCloudConfigExecutor creates the cloud config managed resource that contains:
// 1. A secret containing the dedicated cloud config execution script for each worker group
// 2. A secret containing some shared RBAC policies for downloading the cloud config execution script
func (b *Botanist) DeployManagedResourceForCloudConfigExecutor(ctx context.Context) error {
	var (
		managedResource                  = managedresources.NewForShoot(b.SeedClientSet.Client(), b.Shoot.SeedNamespace, CloudConfigExecutionManagedResourceName, managedresources.LabelValueGardener, false)
		managedResourceSecretsCount      = len(b.Shoot.GetInfo().Spec.Provider.Workers) + 1
		managedResourceSecretLabels      = map[string]string{SecretLabelKeyManagedResource: CloudConfigExecutionManagedResourceName}
		managedResourceSecretNamesWanted = sets.New[string]()
		managedResourceSecretNameToData  = make(map[string]map[string][]byte, managedResourceSecretsCount)

		cloudConfigExecutorSecretNames        []string
		workerNameToOperatingSystemConfigMaps = b.Shoot.Components.Extensions.OperatingSystemConfig.WorkerNameToOperatingSystemConfigsMap()

		fns = make([]flow.TaskFn, 0, managedResourceSecretsCount)
	)

	// Generate cloud-config user-data executor scripts for all worker pools.
	for _, worker := range b.Shoot.GetInfo().Spec.Provider.Workers {
		oscData, ok := workerNameToOperatingSystemConfigMaps[worker.Name]
		if !ok {
			return fmt.Errorf("did not find osc data for worker pool %q", worker.Name)
		}

		kubernetesVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(b.Shoot.KubernetesVersion, worker.Kubernetes)
		if err != nil {
			return err
		}

		hyperkubeImage, err := imagevector.ImageVector().FindImage(imagevector.ImageNameHyperkube, imagevectorutils.RuntimeVersion(kubernetesVersion.String()), imagevectorutils.TargetVersion(kubernetesVersion.String()))
		if err != nil {
			return err
		}

		secretName, data, err := b.generateCloudConfigExecutorResourcesForWorker(worker, oscData.Original, hyperkubeImage)
		if err != nil {
			return err
		}

		cloudConfigExecutorSecretNames = append(cloudConfigExecutorSecretNames, secretName)
		managedResourceSecretNameToData[fmt.Sprintf("shoot-cloud-config-execution-%s", worker.Name)] = data
	}

	// Allow the cloud-config-downloader to download the generated cloud-config user-data scripts.
	downloaderRBACResourcesData, err := DownloaderGenerateRBACResourcesDataFn(cloudConfigExecutorSecretNames)
	if err != nil {
		return err
	}
	managedResourceSecretNameToData["shoot-cloud-config-rbac"] = downloaderRBACResourcesData

	// Create Secrets for the ManagedResource containing all the executor scripts as well as the RBAC resources.
	for secretName, data := range managedResourceSecretNameToData {
		var (
			keyValues                                        = data
			managedResourceSecretName, managedResourceSecret = managedresources.NewSecret(
				b.SeedClientSet.Client(),
				b.Shoot.SeedNamespace,
				secretName,
				keyValues,
				true,
			)
		)

		managedResource.WithSecretRef(managedResourceSecretName)
		managedResourceSecretNamesWanted.Insert(managedResourceSecretName)

		fns = append(fns, func(ctx context.Context) error {
			return managedResourceSecret.
				AddLabels(managedResourceSecretLabels).
				Reconcile(ctx)
		})
	}

	if err := flow.Parallel(fns...)(ctx); err != nil {
		return err
	}

	if err := managedResource.Reconcile(ctx); err != nil {
		return err
	}

	// Cleanup no longer required Secrets for the ManagedResource (e.g., those for removed worker pools).
	secretList := &corev1.SecretList{}
	if err := b.SeedClientSet.Client().List(ctx, secretList, client.InNamespace(b.Shoot.SeedNamespace), client.MatchingLabels(managedResourceSecretLabels)); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjectsFromListConditionally(ctx, b.SeedClientSet.Client(), secretList, func(obj runtime.Object) bool {
		acc, err := meta.Accessor(obj)
		if err != nil {
			return false
		}
		return !managedResourceSecretNamesWanted.Has(acc.GetName())
	})
}

func (b *Botanist) generateCloudConfigExecutorResourcesForWorker(
	worker gardencorev1beta1.Worker,
	oscDataOriginal operatingsystemconfig.Data,
	hyperkubeImage *imagevectorutils.Image,
) (
	string,
	map[string][]byte,
	error,
) {
	kubernetesVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(b.Shoot.KubernetesVersion, worker.Kubernetes)
	if err != nil {
		return "", nil, err
	}

	var (
		registry   = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
		secretName = operatingsystemconfig.Key(worker.Name, kubernetesVersion, worker.CRI)
	)

	var kubeletDataVolume *gardencorev1beta1.DataVolume
	if worker.KubeletDataVolumeName != nil && worker.DataVolumes != nil {
		kubeletDataVolName := worker.KubeletDataVolumeName
		for _, dv := range worker.DataVolumes {
			dataVolume := dv
			if dataVolume.Name == *kubeletDataVolName {
				kubeletDataVolume = &dataVolume
				break
			}
		}
	}

	executorScript, err := ExecutorScriptFn(
		[]byte(oscDataOriginal.Content),
		b.Shoot.CloudConfigExecutionMaxDelaySeconds,
		hyperkubeImage,
		kubernetesVersion.String(),
		kubeletDataVolume,
		*oscDataOriginal.Command,
		oscDataOriginal.Units,
		oscDataOriginal.Files,
	)
	if err != nil {
		return "", nil, err
	}

	resources, err := registry.AddAllAndSerialize(executor.Secret(secretName, metav1.NamespaceSystem, worker.Name, executorScript))
	if err != nil {
		return "", nil, err
	}

	return secretName, resources, nil
}

// GardenerNodeAgentManagedResourceName is a constant for the name of a ManagedResource in the seed cluster in the shoot
// namespace which contains resources for gardener-node-agent.
const GardenerNodeAgentManagedResourceName = "shoot-gardener-node-agent"

// DeployManagedResourceForGardenerNodeAgent creates the ManagedResource that contains:
// - A secret containing the raw original OperatingSystemConfig for each worker pool.
// - A secret containing some shared RBAC resources for downloading the OSC secrets + bootstrapping the node.
func (b *Botanist) DeployManagedResourceForGardenerNodeAgent(ctx context.Context) error {
	var (
		managedResource                  = managedresources.NewForShoot(b.SeedClientSet.Client(), b.Shoot.SeedNamespace, GardenerNodeAgentManagedResourceName, managedresources.LabelValueGardener, false)
		managedResourceSecretsCount      = len(b.Shoot.GetInfo().Spec.Provider.Workers) + 1
		managedResourceSecretLabels      = map[string]string{SecretLabelKeyManagedResource: GardenerNodeAgentManagedResourceName}
		managedResourceSecretNamesWanted = sets.New[string]()
		managedResourceSecretNameToData  = make(map[string]map[string][]byte, managedResourceSecretsCount)

		operatingSystemConfigSecretNames      []string
		workerNameToOperatingSystemConfigMaps = b.Shoot.Components.Extensions.OperatingSystemConfig.WorkerNameToOperatingSystemConfigsMap()

		fns = make([]flow.TaskFn, 0, managedResourceSecretsCount)
	)

	// Generate one OperatingSystemConfig secret for each worker pools. This secret will later be referenced in a
	// ManagedResource.
	for _, worker := range b.Shoot.GetInfo().Spec.Provider.Workers {
		oscData, ok := workerNameToOperatingSystemConfigMaps[worker.Name]
		if !ok {
			return fmt.Errorf("did not find osc data for worker pool %q", worker.Name)
		}

		secretName, data, err := b.generateOperatingSystemConfigSecretForWorker(ctx, worker, oscData.Original.Object)
		if err != nil {
			return err
		}

		operatingSystemConfigSecretNames = append(operatingSystemConfigSecretNames, secretName)
		managedResourceSecretNameToData[fmt.Sprintf("shoot-gardener-node-agent-%s", worker.Name)] = data
	}

	rbacResourcesData, err := nodeagent.GenerateRBACResourcesData(operatingSystemConfigSecretNames)
	if err != nil {
		return err
	}
	managedResourceSecretNameToData["shoot-gardener-node-agent-rbac"] = rbacResourcesData

	// Create Secrets for the ManagedResource containing all the executor scripts as well as the RBAC resources.
	for secretName, data := range managedResourceSecretNameToData {
		var (
			keyValues                                        = data
			managedResourceSecretName, managedResourceSecret = managedresources.NewSecret(
				b.SeedClientSet.Client(),
				b.Shoot.SeedNamespace,
				secretName,
				keyValues,
				true,
			)
		)

		managedResource.WithSecretRef(managedResourceSecretName)
		managedResourceSecretNamesWanted.Insert(managedResourceSecretName)

		fns = append(fns, func(ctx context.Context) error {
			return managedResourceSecret.
				AddLabels(managedResourceSecretLabels).
				Reconcile(ctx)
		})
	}

	if err := flow.Parallel(fns...)(ctx); err != nil {
		return err
	}

	if err := managedResource.Reconcile(ctx); err != nil {
		return err
	}

	// Cleanup no longer required Secrets for the ManagedResource (e.g., those for removed worker pools).
	secretList := &corev1.SecretList{}
	if err := b.SeedClientSet.Client().List(ctx, secretList, client.InNamespace(b.Shoot.SeedNamespace), client.MatchingLabels(managedResourceSecretLabels)); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjectsFromListConditionally(ctx, b.SeedClientSet.Client(), secretList, func(obj runtime.Object) bool {
		acc, err := meta.Accessor(obj)
		if err != nil {
			return false
		}
		return !managedResourceSecretNamesWanted.Has(acc.GetName())
	})
}

var codec runtime.Codec

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))
	yamlSerializer := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
	versions := schema.GroupVersions([]schema.GroupVersion{extensionsv1alpha1.SchemeGroupVersion})
	codec = serializer.NewCodecFactory(scheme).CodecForVersions(yamlSerializer, yamlSerializer, versions, versions)
}

func (b *Botanist) generateOperatingSystemConfigSecretForWorker(
	ctx context.Context,
	worker gardencorev1beta1.Worker,
	osc *extensionsv1alpha1.OperatingSystemConfig,
) (
	string,
	map[string][]byte,
	error,
) {
	// Eliminate unwanted data/fields
	operatingSystemConfig := &extensionsv1alpha1.OperatingSystemConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:        osc.Name,
			Labels:      osc.Labels,
			Annotations: osc.Annotations,
		},
		Spec:   osc.Spec,
		Status: osc.Status,
	}

	// The OperatingSystemConfig will be deployed to the shoot to get processed by gardener-node-agent. It doesn't
	// have access to the referenced secrets (stored in the shoot namespace in the seed), hence we have to translate
	// all references into inline content.
	for i, file := range operatingSystemConfig.Spec.Files {
		if file.Content.SecretRef == nil {
			continue
		}

		secret := &corev1.Secret{}
		if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKey{Name: file.Content.SecretRef.Name, Namespace: b.Shoot.SeedNamespace}, secret); err != nil {
			return "", nil, fmt.Errorf("cannot resolve secret ref from osc: %w", err)
		}

		operatingSystemConfig.Spec.Files[i].Content.SecretRef = nil
		operatingSystemConfig.Spec.Files[i].Content.Inline = &extensionsv1alpha1.FileContentInline{
			Encoding: "b64",
			Data:     utils.EncodeBase64(secret.Data[file.Content.SecretRef.DataKey]),
		}
	}

	// TODO WIP: This block can be removed after Olivers PR.
	for _, unit := range operatingSystemConfig.Spec.Units {
		if unit.Name == kubelet.UnitName || unit.Name == "kubelet-monitor.service" {
			var content []string
			for _, line := range strings.Split(*unit.Content, "\n") {
				if !strings.Contains(line, kubelet.PathScriptCopyKubernetesBinary) {
					content = append(content, line)
				}
			}
			unit.Content = pointer.String(strings.Join(content, "\n"))
		}
	}

	oscRaw, err := runtime.Encode(codec, operatingSystemConfig)
	if err != nil {
		return "", nil, fmt.Errorf("failed encoding OperatingSystemConfig: %w", err)
	}

	kubernetesVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(b.Shoot.KubernetesVersion, worker.Kubernetes)
	if err != nil {
		return "", nil, err
	}

	nodeAgentSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatingsystemconfig.GardenerNodeAgentKey(worker.Name, kubernetesVersion, worker.CRI),
			Namespace: metav1.NamespaceSystem,
			Annotations: map[string]string{
				downloader.AnnotationKeyChecksum: utils.ComputeSHA256Hex(oscRaw),
			},
			Labels: map[string]string{
				v1beta1constants.GardenRole:      v1beta1constants.GardenRoleOperatingSystemConfig,
				v1beta1constants.LabelWorkerPool: worker.Name,
			},
		},
		Data: map[string][]byte{nodeagentv1alpha1.DataKeyOperatingSystemConfig: oscRaw},
	}

	resources, err := managedresources.
		NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer).
		AddAllAndSerialize(nodeAgentSecret)
	if err != nil {
		return "", nil, fmt.Errorf("failed adding gardener-node-agent secret to the registry and serializing it: %w", err)
	}

	return nodeAgentSecret.Name, resources, nil
}
