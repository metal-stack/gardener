// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"strconv"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// NewObjectsProvider creates a new ObjectsProvider for the generic actuator.
func NewObjectsProvider() genericactuator.ObjectsProvider {
	return &objectsProvider{}
}

type objectsProvider struct {
	genericactuator.NoopObjectsProvider
}

// GetStorageClassesObjects returns the objects for the storage classes applied to the shoot by this actuator.
func (op *objectsProvider) GetStorageClassesObjects(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) ([]client.Object, error) {
	return []client.Object{
		&storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
				Annotations: map[string]string{
					"storageclass.kubernetes.io/is-default-class": strconv.FormatBool(true),
					"defaultVolumeType":                           "local",
				},
			},
			Provisioner:       "rancher.io/local-path",
			ReclaimPolicy:     ptr.To(corev1.PersistentVolumeReclaimDelete),
			VolumeBindingMode: ptr.To(storagev1.VolumeBindingWaitForFirstConsumer),
		},
	}, nil
}
