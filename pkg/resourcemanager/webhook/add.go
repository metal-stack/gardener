// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package webhook

import (
	"fmt"

	"github.com/Masterminds/semver"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/resourcemanager/apis/config"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/crddeletionprotection"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/endpointslicehints"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/extensionvalidation"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/highavailabilityconfig"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/kubernetesservicehost"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/podschedulername"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/podtopologyspreadconstraints"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/projectedtokenmount"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/seccompprofile"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/systemcomponentsconfig"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/tokeninvalidator"
)

// AddToManager adds all webhook handlers to the given manager.
func AddToManager(mgr manager.Manager, sourceCluster, targetCluster cluster.Cluster, cfg *config.ResourceManagerConfiguration) error {
	kubernetesClient, err := kubernetesclientset.NewForConfig(targetCluster.GetConfig())
	if err != nil {
		return fmt.Errorf("failed creating Kubernetes client: %w", err)
	}

	targetServerVersion, err := kubernetesClient.DiscoveryClient.ServerVersion()
	if err != nil {
		return err
	}
	targetVersion, err := semver.NewVersion(targetServerVersion.GitVersion)
	if err != nil {
		return err
	}

	if cfg.Webhooks.CRDDeletionProtection.Enabled {
		if err := (&crddeletionprotection.Handler{
			Logger:       mgr.GetLogger().WithName("webhook").WithName(crddeletionprotection.HandlerName),
			SourceReader: sourceCluster.GetAPIReader(),
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", crddeletionprotection.HandlerName, err)
		}
	}

	if cfg.Webhooks.EndpointSliceHints.Enabled {
		if err := (&endpointslicehints.Handler{
			Logger: mgr.GetLogger().WithName("webhook").WithName(endpointslicehints.HandlerName),
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", endpointslicehints.HandlerName, err)
		}
	}

	if cfg.Webhooks.ExtensionValidation.Enabled {
		if err := extensionvalidation.AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handlers: %w", extensionvalidation.HandlerName, err)
		}
	}

	if cfg.Webhooks.HighAvailabilityConfig.Enabled {
		if err := (&highavailabilityconfig.Handler{
			Logger:        mgr.GetLogger().WithName("webhook").WithName(highavailabilityconfig.HandlerName),
			TargetClient:  targetCluster.GetClient(),
			TargetVersion: targetVersion,
			Config:        cfg.Webhooks.HighAvailabilityConfig,
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", highavailabilityconfig.HandlerName, err)
		}
	}

	if cfg.Webhooks.KubernetesServiceHost.Enabled {
		if err := (&kubernetesservicehost.Handler{
			Logger: mgr.GetLogger().WithName("webhook").WithName(kubernetesservicehost.HandlerName),
			Host:   cfg.Webhooks.KubernetesServiceHost.Host,
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", kubernetesservicehost.HandlerName, err)
		}
	}

	if cfg.Webhooks.SystemComponentsConfig.Enabled {
		if err := (&systemcomponentsconfig.Handler{
			Logger:          mgr.GetLogger().WithName("webhook").WithName(systemcomponentsconfig.HandlerName),
			TargetClient:    targetCluster.GetClient(),
			NodeSelector:    cfg.Webhooks.SystemComponentsConfig.NodeSelector,
			PodNodeSelector: cfg.Webhooks.SystemComponentsConfig.PodNodeSelector,
			PodTolerations:  cfg.Webhooks.SystemComponentsConfig.PodTolerations,
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", systemcomponentsconfig.HandlerName, err)
		}
	}

	if cfg.Webhooks.PodSchedulerName.Enabled {
		if err := (&podschedulername.Handler{
			SchedulerName: *cfg.Webhooks.PodSchedulerName.SchedulerName,
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", podschedulername.HandlerName, err)
		}
	}

	if cfg.Webhooks.PodTopologySpreadConstraints.Enabled {
		if err := (&podtopologyspreadconstraints.Handler{
			Logger: mgr.GetLogger().WithName("webhook").WithName(podtopologyspreadconstraints.HandlerName),
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", podtopologyspreadconstraints.HandlerName, err)
		}
	}

	if cfg.Webhooks.ProjectedTokenMount.Enabled {
		if err := (&projectedtokenmount.Handler{
			Logger:            mgr.GetLogger().WithName("webhook").WithName(projectedtokenmount.HandlerName),
			TargetReader:      targetCluster.GetCache(),
			ExpirationSeconds: *cfg.Webhooks.ProjectedTokenMount.ExpirationSeconds,
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", projectedtokenmount.HandlerName, err)
		}
	}

	if cfg.Webhooks.SeccompProfile.Enabled {
		if err := (&seccompprofile.Handler{
			Logger: mgr.GetLogger().WithName("webhook").WithName(seccompprofile.HandlerName),
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", seccompprofile.HandlerName, err)
		}
	}

	if cfg.Webhooks.TokenInvalidator.Enabled {
		if err := (&tokeninvalidator.Handler{
			Logger: mgr.GetLogger().WithName("webhook").WithName(tokeninvalidator.HandlerName),
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", tokeninvalidator.HandlerName, err)
		}
	}

	return nil
}
