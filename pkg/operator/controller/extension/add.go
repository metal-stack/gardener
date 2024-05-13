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
	clientmapbuilder "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/builder"
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
	var err error

	if gardenClientMap == nil {
		gardenClientMap, err = clientmapbuilder.
			NewGardenClientMapBuilder().
			WithRuntimeClient(mgr.GetClient()).
			WithClientConnectionConfig(&cfg.VirtualClientConnection).
			Build(mgr.GetLogger())
		if err != nil {
			return fmt.Errorf("failed to build garden ClientMap: %w", err)
		}
		if err := mgr.Add(gardenClientMap); err != nil {
			return err
		}
	}

	if err := (&gardenerconfig.Reconciler{
		Config:          *cfg,
		Identity:        identity,
		GardenClientMap: gardenClientMap,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding Garden controller: %w", err)
	}

	return nil
}
