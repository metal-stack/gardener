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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,shortName="glet"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`,description="creation timestamp"

// Gardenlet describes a gardenlet to be managed by the gardener-operator.
type Gardenlet struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec contains the specification of this garden.
	Spec GardenletSpec `json:"spec,omitempty"`
	// Status contains the status of this garden.
	Status GardenletStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GardenletList is a list of Gardenlet objects.
type GardenletList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of Gardenlets.
	Items []Gardenlet `json:"items"`
}

// GardenletSpec is the specification of a Gardenlet.
type GardenletSpec struct {
	// KubeconfigSecretRef is a reference to a kubeconfig in which the Gardenlet is supposed to be deployed. If not
	// provided, the gardenlet is deployed into the same cluster where gardener-operator runs.
	// +optional
	KubeconfigSecretRef *corev1.SecretReference `json:"kubeconfigSecretRef,omitempty"`

	// Backup contains the object store configuration for backups for the seed that is managed by this gardenlet.
	// +optional
	Backup *GardenletBackup `json:"backup,omitempty"`

	// Gardenlet specifies that the Gardenlet controller should deploy a gardenlet into the cluster
	// with the given deployment parameters and GardenletConfiguration.
	// +optional
	Gardenlet *seedmanagementv1alpha1.Gardenlet `json:"gardenlet,omitempty"`
}

// GardenletBackup contains the object store configuration for backups for the virtual garden etcd.
type GardenletBackup struct {
	// Provider is a provider name. This field is immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Provider is immutable"
	Provider string `json:"provider"`

	// SecretRef is a reference to a Secret object containing the cloud provider credentials for the object store where
	// backups should be stored. It should have enough privileges to manipulate the objects as well as buckets.
	SecretRef corev1.SecretReference `json:"secretRef"`
}

// GardenletStatus is the status of a gardenlet.
type GardenletStatus struct {
	// Conditions is a list of conditions.
	Conditions []gardencorev1beta1.Condition `json:"conditions,omitempty"`
	// ObservedGeneration is the most recent generation observed for this resource.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}
