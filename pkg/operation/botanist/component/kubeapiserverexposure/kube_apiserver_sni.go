// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeapiserverexposure

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/Masterminds/sprig"
	"google.golang.org/protobuf/types/known/durationpb"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istioapisecurityv1beta1 "istio.io/api/security/v1beta1"
	istiov1beta1 "istio.io/api/type/v1beta1"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	istiosecurity1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SNIValues configure the kube-apiserver service SNI.
type SNIValues struct {
	Hosts                    []string
	NamespaceUID             types.UID
	APIServerClusterIP       string
	APIServerInternalDNSName string
	IstioIngressGateway      IstioIngressGateway
	AccessControl            *gardencorev1beta1.AccessControl
}

// IstioIngressGateway contains the values for istio ingress gateway configuration.
type IstioIngressGateway struct {
	Namespace string
	Labels    map[string]string
}

// NewSNI creates a new instance of DeployWaiter which deploys Istio resources for
// kube-apiserver SNI access.
func NewSNI(
	client client.Client,
	applier kubernetes.Applier,
	namespace string,
	values *SNIValues,
) component.DeployWaiter {
	if values == nil {
		values = &SNIValues{}
	}

	return &sni{
		client:    client,
		applier:   applier,
		namespace: namespace,
		values:    values,
	}
}

type sni struct {
	client    client.Client
	applier   kubernetes.Applier
	namespace string
	values    *SNIValues
}

type envoyFilterTemplateValues struct {
	*SNIValues
	Name      string
	Namespace string
	Host      string
	Port      int
}

func (s *sni) Deploy(ctx context.Context) error {
	var (
		destinationRule = s.emptyDestinationRule()
		envoyFilter     = s.emptyEnvoyFilter()
		gateway         = s.emptyGateway()
		virtualService  = s.emptyVirtualService()
		accessControl   = s.allowAllAuthorizationPolicy()

		hostName        = fmt.Sprintf("%s.%s.svc.%s", v1beta1constants.DeploymentNameKubeAPIServer, s.namespace, gardencorev1beta1.DefaultDomain)
		envoyFilterSpec bytes.Buffer
	)

	if err := envoyFilterSpecTemplate.Execute(&envoyFilterSpec, envoyFilterTemplateValues{
		SNIValues: s.values,
		Name:      envoyFilter.Name,
		Namespace: envoyFilter.Namespace,
		Host:      hostName,
		Port:      kubeapiserver.Port,
	}); err != nil {
		return err
	}
	if err := s.applier.ApplyManifest(ctx, kubernetes.NewManifestReader(envoyFilterSpec.Bytes()), kubernetes.DefaultMergeFuncs); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, s.client, destinationRule, func() error {
		destinationRule.Labels = getLabels()
		destinationRule.Spec = istioapinetworkingv1beta1.DestinationRule{
			ExportTo: []string{"*"},
			Host:     hostName,
			TrafficPolicy: &istioapinetworkingv1beta1.TrafficPolicy{
				ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
					Tcp: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings{
						MaxConnections: 5000,
						TcpKeepalive: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings_TcpKeepalive{
							Time:     &durationpb.Duration{Seconds: 7200},
							Interval: &durationpb.Duration{Seconds: 75},
						},
					},
				},
				Tls: &istioapinetworkingv1beta1.ClientTLSSettings{
					Mode: istioapinetworkingv1beta1.ClientTLSSettings_DISABLE,
				},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, s.client, gateway, func() error {
		gateway.Labels = getLabels()
		gateway.Spec = istioapinetworkingv1beta1.Gateway{
			Selector: s.values.IstioIngressGateway.Labels,
			Servers: []*istioapinetworkingv1beta1.Server{{
				Hosts: s.values.Hosts,
				Port: &istioapinetworkingv1beta1.Port{
					Number:   kubeapiserver.Port,
					Name:     "tls",
					Protocol: "TLS",
				},
				Tls: &istioapinetworkingv1beta1.ServerTLSSettings{
					Mode: istioapinetworkingv1beta1.ServerTLSSettings_PASSTHROUGH,
				},
			}},
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, s.client, virtualService, func() error {
		virtualService.Labels = getLabels()
		virtualService.Spec = istioapinetworkingv1beta1.VirtualService{
			ExportTo: []string{"*"},
			Hosts:    s.values.Hosts,
			Gateways: []string{gateway.Name},
			Tls: []*istioapinetworkingv1beta1.TLSRoute{{
				Match: []*istioapinetworkingv1beta1.TLSMatchAttributes{{
					Port:     kubeapiserver.Port,
					SniHosts: s.values.Hosts,
				}},
				Route: []*istioapinetworkingv1beta1.RouteDestination{{
					Destination: &istioapinetworkingv1beta1.Destination{
						Host: hostName,
						Port: &istioapinetworkingv1beta1.PortSelector{Number: kubeapiserver.Port},
					},
				}},
			}},
		}
		return nil
	}); err != nil {
		return err
	}

	accessControlSpec := accessControl.Spec.DeepCopy()
	if s.values.AccessControl != nil {
		action, err := toIstioAuthPolicyAction(s.values.AccessControl.Action)
		if err != nil {
			return err
		}

		accessControlSpec = &istioapisecurityv1beta1.AuthorizationPolicy{
			Action: action,
			Rules: []*istioapisecurityv1beta1.Rule{{
				From: []*istioapisecurityv1beta1.Rule_From{{
					Source: &istioapisecurityv1beta1.Source{
						IpBlocks:          s.values.AccessControl.Source.IPBlocks,
						NotIpBlocks:       s.values.AccessControl.Source.NotIPBlocks,
						RemoteIpBlocks:    s.values.AccessControl.Source.RemoteIPBlocks,
						NotRemoteIpBlocks: s.values.AccessControl.Source.NotRemoteIPBlocks,
					},
				}},
			}},
		}
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, s.client, accessControl, func() error {
		accessControl.Labels = getLabels()
		accessControl.Spec = istioapisecurityv1beta1.AuthorizationPolicy{
			Action: accessControlSpec.Action,
			Rules:  accessControlSpec.Rules,
			Selector: &istiov1beta1.WorkloadSelector{
				MatchLabels: map[string]string{
					"app":   "istio-ingressgateway",
					"istio": "ingressgateway",
				},
			},
		}

		for i := range accessControl.Spec.Rules {
			accessControl.Spec.Rules[i].When = []*istioapisecurityv1beta1.Condition{{
				Key:    "connection.sni",
				Values: s.values.Hosts,
			}}
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (s *sni) Destroy(ctx context.Context) error {
	return kutil.DeleteObjects(
		ctx,
		s.client,
		s.emptyDestinationRule(),
		s.emptyEnvoyFilter(),
		s.emptyGateway(),
		s.emptyVirtualService(),
		s.allowAllAuthorizationPolicy(),
	)
}

func (s *sni) Wait(_ context.Context) error        { return nil }
func (s *sni) WaitCleanup(_ context.Context) error { return nil }

func (s *sni) emptyDestinationRule() *istionetworkingv1beta1.DestinationRule {
	return &istionetworkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: s.namespace}}
}

func (s *sni) emptyEnvoyFilter() *istionetworkingv1alpha3.EnvoyFilter {
	return &istionetworkingv1alpha3.EnvoyFilter{ObjectMeta: metav1.ObjectMeta{Name: s.namespace, Namespace: s.values.IstioIngressGateway.Namespace}}
}

func (s *sni) emptyGateway() *istionetworkingv1beta1.Gateway {
	return &istionetworkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: s.namespace}}
}

func (s *sni) emptyVirtualService() *istionetworkingv1beta1.VirtualService {
	return &istionetworkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: s.namespace}}
}

func (s *sni) allowAllAuthorizationPolicy() *istiosecurity1beta1.AuthorizationPolicy {
	return &istiosecurity1beta1.AuthorizationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.namespace,
			Namespace: "istio-ingress",
		},
		Spec: istioapisecurityv1beta1.AuthorizationPolicy{
			Action: istioapisecurityv1beta1.AuthorizationPolicy_ALLOW,
			Rules:  []*istioapisecurityv1beta1.Rule{{From: []*istioapisecurityv1beta1.Rule_From{}}},
		},
	}
}

// AnyDeployedSNI returns true if any SNI is deployed in the cluster.
func AnyDeployedSNI(ctx context.Context, c client.Client) (bool, error) {
	l := &unstructured.UnstructuredList{
		Object: map[string]interface{}{
			"apiVersion": istionetworkingv1beta1.SchemeGroupVersion.String(),
			"kind":       "VirtualServiceList",
		},
	}

	if err := c.List(ctx, l, client.MatchingFields{"metadata.name": "kube-apiserver"}, client.Limit(1)); err != nil && !meta.IsNoMatchError(err) {
		return false, err
	}

	return len(l.Items) > 0, nil
}

var (
	//go:embed templates/envoyfilter.yaml
	envoyFilterSpecTemplateContent string
	envoyFilterSpecTemplate        *template.Template
)

func toIstioAuthPolicyAction(action gardencorev1beta1.AuthorizationAction) (istioapisecurityv1beta1.AuthorizationPolicy_Action, error) {
	switch action {
	case gardencorev1beta1.AuthorizationActionAllow:
		return istioapisecurityv1beta1.AuthorizationPolicy_ALLOW, nil
	case gardencorev1beta1.AuthorizationActionDeny:
		return istioapisecurityv1beta1.AuthorizationPolicy_DENY, nil
	default:
		return istioapisecurityv1beta1.AuthorizationPolicy_Action(0), fmt.Errorf("unsupported authorization policy action: %s", action)
	}
}

func init() {
	var err error
	envoyFilterSpecTemplate, err = template.
		New("envoy-filter-spec").
		Funcs(sprig.TxtFuncMap()).
		Parse(envoyFilterSpecTemplateContent)
	utilruntime.Must(err)
}
