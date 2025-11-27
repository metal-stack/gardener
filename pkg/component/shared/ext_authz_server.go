// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/extauthzserver"
)

// NewExtAuthzServer instantiates a new `ext-authz-server` component.
func NewExtAuthzServer(
	c client.Client,
	namespace string,
	enabled bool,
	replicas int32,
	priorityClassName string,
) (
	deployer component.DeployWaiter,
	err error,
) {
	extAuthzServerImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameExtAuthzServer)
	if err != nil {
		return nil, err
	}

	deployer = extauthzserver.New(
		c,
		namespace,
		extauthzserver.Values{
			Image:             extAuthzServerImage.String(),
			PriorityClassName: priorityClassName,
			Replicas:          replicas,
		},
	)

	if !enabled {
		deployer = component.OpDestroyAndWait(deployer)
	}

	return deployer, nil
}
