// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/component/coredns"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultCoreDNS returns a deployer for the CoreDNS.
func (b *Botanist) DefaultCoreDNS() (coredns.Interface, error) {
	image, err := b.ImageVector.FindImage(images.ImageNameCoredns, imagevector.RuntimeVersion(b.ShootVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	values := coredns.Values{
		// resolve conformance test issue (https://github.com/kubernetes/kubernetes/blob/master/test/e2e/network/dns.go#L44)
		// before changing
		ClusterDomain:                   gardencorev1beta1.DefaultDomain,
		ClusterIP:                       b.Shoot.Networks.CoreDNS.String(),
		Image:                           image.String(),
		PodNetworkCIDR:                  b.Shoot.Networks.Pods.String(),
		NodeNetworkCIDR:                 b.Shoot.GetInfo().Spec.Networking.Nodes,
		AutoscalingMode:                 gardencorev1beta1.CoreDNSAutoscalingModeHorizontal,
		KubernetesVersion:               b.Shoot.KubernetesVersion,
		SearchPathRewritesEnabled:       v1beta1helper.IsCoreDNSRewritingEnabled(features.DefaultFeatureGate.Enabled(features.CoreDNSQueryRewriting), b.Shoot.GetInfo().GetAnnotations()),
		SearchPathRewriteCommonSuffixes: getCommonSuffixesForRewriting(b.Shoot.GetInfo().Spec.SystemComponents),
	}

	if b.ShootUsesDNS() {
		values.APIServerHost = pointer.String(b.outOfClusterAPIServerFQDN())
	}

	if v1beta1helper.IsCoreDNSAutoscalingModeUsed(b.Shoot.GetInfo().Spec.SystemComponents, gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional) {
		image, err = b.ImageVector.FindImage(images.ImageNameClusterProportionalAutoscaler, imagevector.RuntimeVersion(b.ShootVersion()), imagevector.TargetVersion(b.ShootVersion()))
		if err != nil {
			return nil, err
		}
		values.ClusterProportionalAutoscalerImage = image.String()
		values.AutoscalingMode = gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional
		values.WantsVerticalPodAutoscaler = b.Shoot.WantsVerticalPodAutoscaler
	}

	return coredns.New(b.SeedClientSet.Client(), b.Shoot.SeedNamespace, values), nil
}

// DeployCoreDNS deploys the CoreDNS system component.
func (b *Botanist) DeployCoreDNS(ctx context.Context) error {
	restartedAtAnnotations, err := b.getCoreDNSRestartedAtAnnotations(ctx)
	if err != nil {
		return err
	}
	b.Shoot.Components.SystemComponents.CoreDNS.SetPodAnnotations(restartedAtAnnotations)

	return b.Shoot.Components.SystemComponents.CoreDNS.Deploy(ctx)
}

// NowFunc is a function returning the current time.
// Exposed for testing.
var NowFunc = time.Now

func (b *Botanist) getCoreDNSRestartedAtAnnotations(ctx context.Context) (map[string]string, error) {
	const key = "gardener.cloud/restarted-at"

	if controllerutils.HasTask(b.Shoot.GetInfo().Annotations, v1beta1constants.ShootTaskRestartCoreAddons) {
		return map[string]string{key: NowFunc().UTC().Format(time.RFC3339)}, nil
	}

	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: coredns.DeploymentName, Namespace: metav1.NamespaceSystem}}
	if err := b.ShootClientSet.Client().Get(ctx, client.ObjectKeyFromObject(deployment), deployment); client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	if val, ok := deployment.Spec.Template.ObjectMeta.Annotations[key]; ok {
		return map[string]string{key: val}, nil
	}

	return nil, nil
}

func getCommonSuffixesForRewriting(systemComponents *gardencorev1beta1.SystemComponents) []string {
	if features.DefaultFeatureGate.Enabled(features.CoreDNSQueryRewriting) && systemComponents != nil && systemComponents.CoreDNS != nil && systemComponents.CoreDNS.Rewriting != nil {
		return systemComponents.CoreDNS.Rewriting.CommonSuffixes
	}
	return []string{}
}
