// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/shared"
)

// DefaultExtAuthServer returns a deployer for ExtAuthServer.
func (b *Botanist) DefaultExtAuthServer() (component.DeployWaiter, error) {
	return shared.NewExtAuthzServer(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		true,
		b.Shoot.GetReplicas(1),
		v1beta1constants.PriorityClassNameShootControlPlane100,
	)
}

// DeployExtAuthServer deploys the extAuthServer in the Seed cluster.
func (b *Botanist) DeployExtAuthServer(ctx context.Context) error {
	// disable extAuthServer if no observability components are needed
	if !b.WantsObservabilityComponents() {
		return b.Shoot.Components.ControlPlane.ExtAuthServer.Destroy(ctx)
	}

	return b.Shoot.Components.ControlPlane.ExtAuthServer.Deploy(ctx)
}
