// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/validation"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// GardenletStrategy define the strategy for storing gardenlets.
type GardenletStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// NewStrategy return a storage strategy for gardenlets.
func NewStrategy() GardenletStrategy {
	return GardenletStrategy{
		api.Scheme,
		names.SimpleNameGenerator,
	}
}

// NamespaceScoped indicates if the object is namespaced scoped.
func (GardenletStrategy) NamespaceScoped() bool { return true }

func (GardenletStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	gardenlet := obj.(*seedmanagement.Gardenlet)

	gardenlet.Generation = 1
}

func (GardenletStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newGardenlet := obj.(*seedmanagement.Gardenlet)
	oldGardenlet := old.(*seedmanagement.Gardenlet)

	if mustIncreaseGeneration(oldGardenlet, newGardenlet) {
		newGardenlet.Generation = oldGardenlet.Generation + 1
	}
}

func mustIncreaseGeneration(oldGardenlet, newGardenlet *seedmanagement.Gardenlet) bool {
	// The ControllerRegistration specification changes.
	if !apiequality.Semantic.DeepEqual(oldGardenlet.Spec, newGardenlet.Spec) {
		return true
	}

	// The deletion timestamp was set.
	if oldGardenlet.DeletionTimestamp == nil && newGardenlet.DeletionTimestamp != nil {
		return true
	}

	// The operation annotation was added with value "reconcile"
	if kubernetesutils.HasMetaDataAnnotation(&newGardenlet.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile) {
		delete(newGardenlet.Annotations, v1beta1constants.GardenerOperation)
		return true
	}

	// The operation annotation was added with value "renew-kubeconfig"
	if kubernetesutils.HasMetaDataAnnotation(&newGardenlet.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRenewKubeconfig) {
		return true
	}

	return false
}

// Validate allow to validate the object.
func (GardenletStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	gardenlet := obj.(*seedmanagement.Gardenlet)
	return validation.ValidateGardenlet(gardenlet)
}

// ValidateUpdate validates the update on the object.
// The old and the new version of the object are passed in.
func (GardenletStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldGardenlet, newGardenlet := oldObj.(*seedmanagement.Gardenlet), newObj.(*seedmanagement.Gardenlet)
	return validation.ValidateGardenletUpdate(newGardenlet, oldGardenlet)
}

// Canonicalize can be used to transform the object into its canonical format.
func (GardenletStrategy) Canonicalize(_ runtime.Object) {
}

// AllowCreateOnUpdate indicates if the object can be created via a PUT operation.
func (GardenletStrategy) AllowCreateOnUpdate() bool {
	return false
}

// AllowUnconditionalUpdate indicates if the object can be updated
// independently of the resource version.
func (GardenletStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// WarningsOnCreate returns warnings to the client performing a create.
func (GardenletStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (GardenletStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

// StatusStrategy defines the strategy for storing seeds statuses.
type StatusStrategy struct {
	GardenletStrategy
}

// NewStatusStrategy defines the storage strategy for the status subresource of Gardenlets.
func NewStatusStrategy() StatusStrategy {
	return StatusStrategy{NewStrategy()}
}

// PrepareForUpdate is invoked on update before validation to normalize
// the object.  For example: remove fields that are not to be persisted,
// sort order-insensitive list fields, etc.  This should not remove fields
// whose presence would be considered a validation error.
func (s StatusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newGardenlet := obj.(*seedmanagement.Gardenlet)
	oldGardenlet := old.(*seedmanagement.Gardenlet)
	newGardenlet.Spec = oldGardenlet.Spec
}

// ValidateUpdate validates the update on the given old and new object.
func (StatusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateGardenletStatusUpdate(obj.(*seedmanagement.Gardenlet), old.(*seedmanagement.Gardenlet))
}
