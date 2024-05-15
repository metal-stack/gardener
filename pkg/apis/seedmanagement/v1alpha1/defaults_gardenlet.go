// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	gardenletv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

// SetDefaults_Gardenlet sets default values for Gardenlet objects.
func SetDefaults_Gardenlet(obj *Gardenlet) {
	setDefaultsGardenletSpec(&obj.Spec, obj.Name, obj.Namespace)
}

func setDefaultsGardenletSpec(obj *GardenletSpec, name, namespace string) {
	// Decode gardenlet config to an external version
	// Without defaults, since we don't want to set gardenlet config defaults in the resource at this point
	gardenletConfig, err := encoding.DecodeGardenletConfiguration(&obj.Config, false)
	if err != nil {
		return
	}

	// If the gardenlet config was decoded without errors to nil,
	// initialize it with an empty config
	if gardenletConfig == nil {
		gardenletConfig = &gardenletv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
		}
	}

	// Set gardenlet config defaults
	setDefaultsGardenletConfiguration(gardenletConfig, name, namespace)

	// Set gardenlet config back to obj.Config
	// Encoding back to bytes is not needed, it will be done by the custom conversion code
	obj.Config = runtime.RawExtension{Object: gardenletConfig}
}
