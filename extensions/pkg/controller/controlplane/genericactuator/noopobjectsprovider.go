// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator

import (
	"context"
	"errors"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var NotImplemented error = errors.New("not implemented")

// NoopObjectsProvider provides no-op implementation of ObjectsProvider. This can be anonymously composed by actual ObjectsProviders for convenience.
type NoopObjectsProvider struct{}

var _ ObjectsProvider = NoopObjectsProvider{}

// GetConfigChartValues returns the values for the config chart applied by this actuator.
func (vp NoopObjectsProvider) GetConfigObjects(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) ([]client.Object, error) {
	return nil, NotImplemented
}

// GetControlPlaneChartValues returns the values for the control plane chart applied by this actuator.
func (vp NoopObjectsProvider) GetControlPlaneObjects(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster, secretsReader secretsmanager.Reader, checksums map[string]string, scaledDown bool) ([]client.Object, error) {
	return nil, NotImplemented
}

// GetControlPlaneShootChartValues returns the values for the control plane shoot chart applied by this actuator.
func (vp NoopObjectsProvider) GetControlPlaneShootObjects(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster, secretsReader secretsmanager.Reader, checksums map[string]string) ([]client.Object, error) {
	return nil, NotImplemented
}

// GetControlPlaneShootCRDsChartValues returns the values for the control plane shoot CRDs chart applied by this actuator.
func (vp NoopObjectsProvider) GetControlPlaneShootCRDsObjects(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) ([]client.Object, error) {
	return nil, NotImplemented
}

// GetStorageClassesChartValues returns the values for the storage classes chart applied by this actuator.
func (vp NoopObjectsProvider) GetStorageClassesObjects(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) ([]client.Object, error) {
	return nil, NotImplemented
}

// GetControlPlaneExposureChartValues returns the values for the control plane exposure chart applied by this actuator.
//
// Deprecated: Control plane with purpose `exposure` is being deprecated and will be removed in gardener v1.123.0.
// TODO(theoddora): Remove this function in v1.123.0 when the Purpose field is removed.
func (vp NoopObjectsProvider) GetControlPlaneExposureObjects(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster, secretsReader secretsmanager.Reader, checksums map[string]string) ([]client.Object, error) {
	return nil, NotImplemented
}
