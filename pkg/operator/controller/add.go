// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	clientmapbuilder "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/builder"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/controller/service"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	"github.com/gardener/gardener/pkg/operator/controller/controllerregistrar"
	"github.com/gardener/gardener/pkg/operator/controller/extension"
	"github.com/gardener/gardener/pkg/operator/controller/garden"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// AddToManager adds all controllers to the given manager.
func AddToManager(ctx context.Context, mgr manager.Manager, cfg *config.OperatorConfiguration) error {
	identity, err := gardenerutils.DetermineIdentity()
	if err != nil {
		return err
	}

	gardenClientMap, err := clientmapbuilder.
		NewGardenClientMapBuilder().
		WithRuntimeClient(mgr.GetClient()).
		WithClientConnectionConfig(&cfg.VirtualClientConnection).
		WithGardenNamespace(v1beta1constants.GardenNamespace).
		Build(mgr.GetLogger())

	if err != nil {
		return fmt.Errorf("failed to build garden ClientMap: %w", err)
	}
	if err := mgr.Add(gardenClientMap); err != nil {
		return err
	}

	if err := garden.AddToManager(ctx, mgr, cfg, identity, gardenClientMap); err != nil {
		return err
	}

	// our stuff
	if err := extension.AddToManager(ctx, mgr, cfg, identity, gardenClientMap); err != nil {
		return err
	}

	if err := (&controllerregistrar.Reconciler{
		NetworkPolicyControllerConfiguration: cfg.Controllers.NetworkPolicy,
		VPAEvictionControllerConfiguration:   cfg.Controllers.VPAEvictionRequirements,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding NetworkPolicy Registrar controller: %w", err)
	}

	if os.Getenv("GARDENER_OPERATOR_LOCAL") == "true" {
		virtualGardenIstioIngressPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: sharedcomponent.GetIstioZoneLabels(nil, nil)})
		if err != nil {
			return err
		}

		nginxIngressPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: map[string]string{
			"app":       "nginx-ingress",
			"component": "controller",
		}})
		if err != nil {
			return err
		}

		if err := (&service.Reconciler{IsMultiZone: true}).AddToManager(mgr, predicate.Or(virtualGardenIstioIngressPredicate, nginxIngressPredicate)); err != nil {
			return fmt.Errorf("failed adding Service controller: %w", err)
		}
	}

	return nil
}
