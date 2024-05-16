// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package runtimeconfig

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-base/featuregate"
	podsecurityadmissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operator"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/oci"
)

const (
	requeueUnhealthyVirtualKubeApiserver = 10 * time.Second
	ConditionReconcileFailed             = "ReconcileFailed"
	ConditionDeleteFailed                = "DeleteFailed"
	ConditionReconcileSuccess            = "ReconcileSuccessful"
)

var (
	managedResourceLabels = map[string]string{v1beta1constants.LabelCareConditionType: string(operatorv1alpha1.RuntimeComponentsHealthy)}
)

// Reconciler reconciles Gardens.
type Reconciler struct {
	RuntimeClientSet kubernetes.Interface
	RuntimeVersion   *semver.Version
	HelmRegistry     oci.Interface
	Config           config.OperatorConfiguration
	Clock            clock.Clock
	Recorder         record.EventRecorder
	GardenClientMap  clientmap.ClientMap
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	garden := &operatorv1alpha1.Garden{}
	if err := r.RuntimeClientSet.Client().Get(ctx, request.NamespacedName, garden); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}
	if garden.DeletionTimestamp != nil {
		return reconcile.Result{}, r.cleanUp(ctx, log, garden)
	}

	return reconcile.Result{}, r.reconcile(ctx, log, garden)
}

func (r *Reconciler) reconcile(ctx context.Context, log logr.Logger, garden *operatorv1alpha1.Garden) error {
	extensionList := &operatorv1alpha1.ExtensionList{}
	err := r.RuntimeClientSet.Client().List(ctx, extensionList)
	if err != nil {
		return fmt.Errorf("error retrieving extensions: %w", err)
	}

	// merge extensions with defaults
	for i, extension := range extensionList.Items {
		extensionList.Items[i].Spec, err = operator.MergeExtensionSpecs(extension.Name, extension.Spec)
		if err != nil {
			return fmt.Errorf("error merging extension spec for name %s: %w", extension.Name, err)
		}
	}

	usedExts, err := r.computeUsedExtensions(garden, extensionList)
	if err != nil {
		return err
	}

	var errs []error

	log.Info("Deploying used extensions")
	for _, ext := range usedExts {
		errs = append(errs, r.deployExtension(ctx, log, ext))
	}

	log.Info("Cleanup managed resources for unused extensions")
	existingManagedResourcesList := &resourcesv1alpha1.ManagedResourceList{}
	err = r.RuntimeClientSet.Client().List(ctx, existingManagedResourcesList, client.MatchingLabels{
		ManagedByLabel: ControllerName,
	})
	errs = append(errs, err)

	for _, mr := range existingManagedResourcesList.Items {
		mr := mr
		errs = append(errs, r.deleteManagedResourceIfNotNeeded(ctx, log, &mr, usedExts))
	}

	return errors.Join(errs...)
}

func (r *Reconciler) cleanUp(ctx context.Context, log logr.Logger, _ *operatorv1alpha1.Garden) error {
	extensionList := &operatorv1alpha1.ExtensionList{}
	err := r.RuntimeClientSet.Client().List(ctx, extensionList)
	if err != nil {
		return fmt.Errorf("error retrieving extensions: %w", err)
	}

	for _, ext := range extensionList.Items {
		ext := ext
		requiredCondition := v1beta1helper.GetCondition(ext.Status.Conditions, operatorv1alpha1.ExtensionRequired)
		if requiredCondition.Status == gardencorev1beta1.ConditionTrue {
			continue
		}
		log.Info("Deleting managed resource", "Extension", ext.Name)
		if err := r.RuntimeClientSet.Client().DeleteAllOf(ctx, &resourcesv1alpha1.ManagedResource{}, client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{
			ManagedByLabel:     ControllerName,
			ExtensionNameLabel: ext.Name,
		}); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) computeUsedExtensions(garden *operatorv1alpha1.Garden, extensionList *operatorv1alpha1.ExtensionList) ([]operatorv1alpha1.Extension, error) {
	providedResources := make(map[string]map[string]operatorv1alpha1.Extension)

	for _, ext := range extensionList.Items {
		for _, res := range ext.Spec.Resources {
			extensionsWithKind := providedResources[res.Kind]
			if extensionsWithKind == nil {
				extensionsWithKind = make(map[string]operatorv1alpha1.Extension)
				providedResources[res.Kind] = extensionsWithKind
			}
			extensionsWithKind[res.Type] = ext
		}
	}

	extensions := make(map[string]operatorv1alpha1.Extension)
	if etcd := garden.Spec.VirtualCluster.ETCD; etcd != nil && etcd.Main != nil && etcd.Main.Backup != nil {
		extensionsWithKind, ok := providedResources[extensionsv1alpha1.BackupBucketResource]
		if !ok {
			return nil, fmt.Errorf("no extension provides a %q resource of type %q", extensionsv1alpha1.BackupBucketResource, etcd.Main.Backup.Provider)
		}
		ext, ok := extensionsWithKind[etcd.Main.Backup.Provider]
		if !ok {
			return nil, fmt.Errorf("no extension provides a %q resource of type %q", extensionsv1alpha1.BackupBucketResource, etcd.Main.Backup.Provider)
		}
		extensions[ext.Name] = ext
	}

	dns := garden.Spec.VirtualCluster.DNS
	if dns.Provider != nil {
		provider := *dns.Provider
		extensionsWithKind, ok := providedResources[extensionsv1alpha1.DNSRecordResource]
		if !ok {
			return nil, fmt.Errorf("no extension provides a %q resource of type %q", extensionsv1alpha1.DNSRecordResource, provider)
		}
		ext, ok := extensionsWithKind[provider]
		if !ok {
			return nil, fmt.Errorf("no extension provides a %q resource of type %q", extensionsv1alpha1.DNSRecordResource, provider)
		}
		extensions[ext.Name] = ext
	}

	exts := make([]operatorv1alpha1.Extension, 0, len(extensions))
	for _, ext := range extensions {
		exts = append(exts, ext)
	}
	return exts, nil
}

func (r *Reconciler) deployExtension(ctx context.Context, log logr.Logger, extension operatorv1alpha1.Extension) error {
	deployment := extension.Spec.Deployment
	if deployment == nil || deployment.Extension == nil {
		return fmt.Errorf("no deployment found for extension %q", extension.Name)
	}

	helmDeployment := deployment.Extension.Helm
	var helmValues map[string]interface{}
	if helmDeployment.Values != nil {
		if err := yaml.Unmarshal(helmDeployment.Values.Raw, &helmValues); err != nil {
			log.Error(err, "Failed to unmarshal helm deployment for extension", "extension", extension.Name)
			return err
		}
	}

	namespace := getNamespaceForExtension(&extension)
	log.Info("Creating namespace for extension", "extension", extension.Name, "namespace", namespace)
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.RuntimeClientSet.Client(), namespace, func() error {
		if podSecurityEnforce, ok := deployment.Extension.Annotations[v1beta1constants.AnnotationPodSecurityEnforce]; ok {
			metav1.SetMetaDataLabel(&namespace.ObjectMeta, podsecurityadmissionapi.EnforceLevelLabel, podSecurityEnforce)
		} else {
			delete(namespace.Labels, podsecurityadmissionapi.EnforceLevelLabel)
		}
		return nil
	}); err != nil {
		log.Error(err, "Failed to create namespace for extension", "extension", extension.Name, "namespace", namespace)
		return err
	}

	archive := helmDeployment.RawChart
	if len(archive) == 0 {
		var err error
		log.Info("Pulling Helm Chart from OCI Repository", "extension", extension.Name)
		archive, err = r.HelmRegistry.Pull(ctx, helmDeployment.OCIRepository)
		if err != nil {
			log.Error(err, "Failed to pull Helm Chart from OCI Repository", "extension", extension.Name, "repository", helmDeployment.OCIRepository)
			return err
		}
	}

	// Mix-in some standard values for garden.
	featureToEnabled := make(map[featuregate.Feature]bool)
	for feature := range features.DefaultFeatureGate.GetAll() {
		featureToEnabled[feature] = features.DefaultFeatureGate.Enabled(feature)
	}

	gardenerValues := map[string]interface{}{
		"gardener": map[string]interface{}{
			"garden": map[string]interface{}{
				"isGardenCluster": true,
			},
			"gardener-operator": map[string]interface{}{
				"featureGates": featureToEnabled,
			},
		},
	}

	log.Info("Rendering chart for extension", "extension", extension.Name)
	release, err := r.RuntimeClientSet.ChartRenderer().RenderArchive(archive, extension.Name, namespace.Name, utils.MergeMaps(helmValues, gardenerValues))
	if err != nil {
		log.Error(err, "Failed to render archive for extension", "extension", extension.Name)
		return err
	}

	secretData := release.AsSecretData()

	extensionLabels := map[string]string{
		ManagedByLabel:     ControllerName,
		ExtensionNameLabel: extension.Name,
	}

	log.Info("Creating managed resource for extension", "extension", extension.Name)
	if err := managedresources.CreateForSeedWithLabels(
		ctx,
		r.RuntimeClientSet.Client(),
		v1beta1constants.GardenNamespace,
		extension.Name,
		false,
		utils.MergeStringMaps(managedResourceLabels, extensionLabels),
		secretData); err != nil {
		return err
	}

	log.Info("Finished to deploy managed resource for extension", "extension", extension.Name)
	return nil
}

func (r *Reconciler) deleteManagedResourceIfNotNeeded(ctx context.Context, log logr.Logger, mr *resourcesv1alpha1.ManagedResource, usedExtensions []operatorv1alpha1.Extension) error {
	extensionName := mr.Labels["extension-name"]
	for _, ext := range usedExtensions {
		if ext.Name == extensionName {
			return nil
		}
	}

	ext := &operatorv1alpha1.Extension{}
	err := r.RuntimeClientSet.Client().Get(ctx, client.ObjectKey{
		Name: extensionName,
	}, ext)
	if err != nil {
		return fmt.Errorf("failed to get extension %q: %w", extensionName, err)
	}
	requiredCondition := v1beta1helper.GetCondition(ext.Status.Conditions, operatorv1alpha1.ExtensionRequired)
	if requiredCondition.Status == gardencorev1beta1.ConditionTrue {
		return nil
	}

	log.Info("Removing managed resource for unused extension", "managed-resource", mr.Name, "extension", extensionName)
	err = r.RuntimeClientSet.Client().Delete(ctx, mr)
	if err != nil {
		log.Error(err, "Failed to remove managed resource for unused extension", "managed-resource", mr.Name, "extension", extensionName)
		return err
	}
	return nil
}

// RuntimeConfigConditions contains all conditions of the garden status subresource.
type RuntimeConfigConditions struct {
	runtimeConfigReconciled gardencorev1beta1.Condition
}

// ConvertToSlice returns the garden conditions as a slice.
func (g RuntimeConfigConditions) ConvertToSlice() []gardencorev1beta1.Condition {
	return []gardencorev1beta1.Condition{
		g.runtimeConfigReconciled,
	}
}

// ConditionTypes returns all garden condition types.
func (g RuntimeConfigConditions) ConditionTypes() []gardencorev1beta1.ConditionType {
	return []gardencorev1beta1.ConditionType{
		g.runtimeConfigReconciled.Type,
	}
}

// NewGardenConfigConditions returns a new instance of RuntimeConfigConditions.
// All conditions are retrieved from the given 'status' or newly initialized.
func NewGardenConfigConditions(clock clock.Clock, status operatorv1alpha1.ExtensionStatus) RuntimeConfigConditions {
	return RuntimeConfigConditions{
		runtimeConfigReconciled: v1beta1helper.GetOrInitConditionWithClock(clock, status.Conditions, operatorv1alpha1.RuntimeConfigReconciled),
	}
}

func getNamespaceForExtension(extension *operatorv1alpha1.Extension) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "extension-" + extension.Name,
		},
	}
}
