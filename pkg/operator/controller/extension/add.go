// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	"github.com/gardener/gardener/pkg/operator/controller/extension/gardenerconfig"
)

// AddToManager adds all Garden controllers to the given manager.
func AddToManager(
	_ context.Context,
	mgr manager.Manager,
	cfg *config.OperatorConfiguration,
	identity *gardencorev1beta1.Gardener,
	gardenClientMap clientmap.ClientMap,
) error {
	if gardenClientMap == nil {
		return fmt.Errorf("gardenClientMap cannot be nil")
	}

	if err := (&gardenerconfig.Reconciler{
		Config:          *cfg,
		GardenClientMap: gardenClientMap,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding Garden controller: %w", err)
	}

	return nil
}
