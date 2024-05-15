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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-base/featuregate"
	podsecurityadmissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/clock"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
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
		panic("deletion not implemented")
	}

	err := r.reconcile(ctx, garden)
	return reconcile.Result{}, err
}

func (r *Reconciler) reconcile(ctx context.Context, garden *operatorv1alpha1.Garden) error {
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

	required, err := r.computeRequiredExtensions(garden, extensionList)
	if err != nil {
		return err
	}

	var errs []error
	for _, ext := range required {
		errs = append(errs, r.deployExtension(ctx, ext))
	}

	return errors.Join(errs...)
}

func (r *Reconciler) computeRequiredExtensions(garden *operatorv1alpha1.Garden, extensionList *operatorv1alpha1.ExtensionList) ([]operatorv1alpha1.Extension, error) {

	providedResources := make(map[string]map[string]operatorv1alpha1.Extension)

	for _, ext := range extensionList.Items {
		for _, res := range ext.Spec.Resources {
			extensionsWithKind := providedResources[res.Kind]
			if extensionsWithKind == nil {
				extensionsWithKind = make(map[string]operatorv1alpha1.Extension)
				providedResources[res.Kind] = extensionsWithKind
			}
			extensionsWithKind[res.Type] = ext // TODO: may there be multiple extensions?
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

func (r *Reconciler) deployExtension(ctx context.Context, extension operatorv1alpha1.Extension) error {
	deployment := extension.Spec.Deployment
	if deployment == nil || deployment.Extension == nil {
		return fmt.Errorf("no deployment found for extension %q", extension.Name)
	}

	helmDeployment := deployment.Extension.Helm
	var helmValues map[string]interface{}
	if helmDeployment.Values != nil {
		if err := yaml.Unmarshal(helmDeployment.Values.Raw, &helmValues); err != nil {
			// TODO
			// conditionValid = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionValid, gardencorev1beta1.ConditionFalse, "ChartInformationInvalid", fmt.Sprintf("chart values cannot be unmarshalled: %+v", err))
			return err
		}
	}

	namespace := getNamespaceForExtension(&extension)
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.RuntimeClientSet.Client(), namespace, func() error {
		if podSecurityEnforce, ok := deployment.Extension.Annotations[v1beta1constants.AnnotationPodSecurityEnforce]; ok {
			metav1.SetMetaDataLabel(&namespace.ObjectMeta, podsecurityadmissionapi.EnforceLevelLabel, podSecurityEnforce)
		} else {
			delete(namespace.Labels, podsecurityadmissionapi.EnforceLevelLabel)
		}
		return nil
	}); err != nil {
		return err
	}

	archive := helmDeployment.RawChart
	if len(archive) == 0 {
		var err error
		archive, err = r.HelmRegistry.Pull(ctx, helmDeployment.OCIRepository)
		if err != nil {
			// TODO
			// conditionValid = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionValid, gardencorev1beta1.ConditionFalse, "OCIChartCannotBePulled", fmt.Sprintf("chart pulling process failed: %+v", err))
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

	release, err := r.RuntimeClientSet.ChartRenderer().RenderArchive(archive, extension.Name, namespace.Name, utils.MergeMaps(helmValues, gardenerValues))
	if err != nil {
		// conditionValid = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionValid, gardencorev1beta1.ConditionFalse, "ChartCannotBeRendered", fmt.Sprintf("chart rendering process failed: %+v", err))
		return err
	}

	// conditionValid = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionValid, gardencorev1beta1.ConditionTrue, "RegistrationValid", "chart could be rendered successfully.")
	secretData := release.AsSecretData()

	if err := managedresources.Create(
		ctx,
		r.RuntimeClientSet.Client(),
		v1beta1constants.GardenNamespace,
		extension.Name,
		map[string]string{"extension-name": extension.Name},
		false,
		v1beta1constants.SeedResourceManagerClass,
		secretData,
		nil,
		nil,
		nil,
	); err != nil {
		// conditionInstalled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionInstalled, gardencorev1beta1.ConditionFalse, "InstallationFailed", fmt.Sprintf("Creation of ManagedResource %q failed: %+v", controllerInstallation.Name, err))
		return err
	}

	// if conditionInstalled.Status == gardencorev1beta1.ConditionUnknown {
	// 	// initially set condition to Pending
	// 	// care controller will update condition based on 'ResourcesApplied' condition of ManagedResource
	// 	conditionInstalled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionInstalled, gardencorev1beta1.ConditionFalse, "InstallationPending", fmt.Sprintf("Installation of ManagedResource %q is still pending.", controllerInstallation.Name))
	// }

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

// NewGardenConfigConditions returns a new instance of GardenerConfigConditions.
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
