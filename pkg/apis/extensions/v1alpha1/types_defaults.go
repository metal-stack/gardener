// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// DefaultSpec contains common status fields for every extension resource.
type DefaultSpec struct {
	// Type contains the instance of the resource's kind.
	Type string `json:"type"`
	// ProviderConfig is the provider specific configuration.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty"`
	// Class holds the resource class used to control the responsibility for multiple resource manager instances
	// +optional
	Class string `json:"class"`
}

// GetExtensionType implements Spec.
func (d *DefaultSpec) GetExtensionType() string {
	return d.Type
}

// GetExtensionPurpose implements Spec.
func (d *DefaultSpec) GetExtensionPurpose() *string {
	return nil
}

// GetProviderConfig implements Spec.
func (d *DefaultSpec) GetProviderConfig() *runtime.RawExtension {
	return d.ProviderConfig
}

// GetExtensionClass implements Spec.
func (d *DefaultSpec) GetExtensionClass() string {
	return d.Class
}

// DefaultStatus contains common status fields for every extension resource.
type DefaultStatus struct {
	// ProviderStatus contains provider-specific status.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	ProviderStatus *runtime.RawExtension `json:"providerStatus,omitempty"`
	// Conditions represents the latest available observations of a Seed's current state.
	// +optional
	Conditions []gardencorev1beta1.Condition `json:"conditions,omitempty"`
	// LastError holds information about the last occurred error during an operation.
	// +optional
	LastError *gardencorev1beta1.LastError `json:"lastError,omitempty"`
	// LastOperation holds information about the last operation on the resource.
	// +optional
	LastOperation *gardencorev1beta1.LastOperation `json:"lastOperation,omitempty"`
	// ObservedGeneration is the most recent generation observed for this resource.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// State can be filled by the operating controller with what ever data it needs.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	State *runtime.RawExtension `json:"state,omitempty"`
	// Resources holds a list of named resource references that can be referred to in the state by their names.
	// +optional
	Resources []gardencorev1beta1.NamedResourceReference `json:"resources,omitempty"`
}

// GetProviderStatus implements Status.
func (d *DefaultStatus) GetProviderStatus() *runtime.RawExtension {
	return d.ProviderStatus
}

// GetConditions implements Status.
func (d *DefaultStatus) GetConditions() []gardencorev1beta1.Condition {
	return d.Conditions
}

// SetConditions implements Status.
func (d *DefaultStatus) SetConditions(c []gardencorev1beta1.Condition) {
	d.Conditions = c
}

// GetLastOperation implements Status.
func (d *DefaultStatus) GetLastOperation() *gardencorev1beta1.LastOperation {
	return d.LastOperation
}

// SetLastOperation implements Status.
func (d *DefaultStatus) SetLastOperation(lastOp *gardencorev1beta1.LastOperation) {
	d.LastOperation = lastOp
}

// GetLastError implements Status.
func (d *DefaultStatus) GetLastError() *gardencorev1beta1.LastError {
	return d.LastError
}

// SetLastError implements Status.
func (d *DefaultStatus) SetLastError(lastErr *gardencorev1beta1.LastError) {
	d.LastError = lastErr
}

// GetObservedGeneration implements Status.
func (d *DefaultStatus) GetObservedGeneration() int64 {
	return d.ObservedGeneration
}

// SetObservedGeneration implements Status.
func (d *DefaultStatus) SetObservedGeneration(generation int64) {
	d.ObservedGeneration = generation
}

// GetState implements Status.
func (d *DefaultStatus) GetState() *runtime.RawExtension {
	return d.State
}

// SetState implements Status.
func (d *DefaultStatus) SetState(state *runtime.RawExtension) {
	d.State = state
}

// GetResources implements Status.
func (d *DefaultStatus) GetResources() []gardencorev1beta1.NamedResourceReference {
	return d.Resources
}

// SetResources implements Status.
func (d *DefaultStatus) SetResources(namedResourceReference []gardencorev1beta1.NamedResourceReference) {
	d.Resources = namedResourceReference
}
