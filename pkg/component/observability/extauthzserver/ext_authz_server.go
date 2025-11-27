// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extauthzserver

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/istio"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"

	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"istio.io/api/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
)

const (
	managedResourceName = "ext-authz-server"

	rootMountPath = "/secrets"
)

// Values is the values for ext-authz-server configurations
type Values struct {
	// Image is the ext-authz-server image.
	Image string
	// PriorityClassName is the name of the priority class of the ext-authz-server.
	PriorityClassName string
	// Replicas is the number of pod replicas for the plutono.
	Replicas int32
}

type extAuthz struct {
	client    client.Client
	namespace string
	values    Values
}

// New creates a new instance of ext-authz-server deployer.
func New(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &extAuthz{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

func (e *extAuthz) Deploy(ctx context.Context) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		volumes       = []corev1.Volume{}
		volumeMounts  = []corev1.VolumeMount{}
		configPatches = []*v1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{}

		svc = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.DeploymentNameExtAuthzServer,
				Namespace: e.namespace,
			},
			Spec: corev1.ServiceSpec{
				Selector: getLabels(),
				Ports: []corev1.ServicePort{
					{
						Name:       "grpc",
						Port:       10000,
						TargetPort: intstr.FromInt32(10000),
					},
				},
			},
		}
	)

	utilruntime.Must(gardenerutils.InjectNetworkPolicyNamespaceSelectors(svc,
		metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress}},
		metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: v1beta1constants.LabelExposureClassHandlerName, Operator: metav1.LabelSelectorOpExists}}},
	))

	httpRouteList := &gwapiv1.HTTPRouteList{}
	err := e.client.List(ctx, httpRouteList, client.InNamespace(e.namespace), client.HasLabels{v1beta1constants.LabelBasicAuthSecretName})
	if err != nil {
		return fmt.Errorf("unable to list http routes: %w", err)
	}

	for _, route := range httpRouteList.Items {
		for _, hostname := range route.Spec.Hostnames {
			subdomain, _, found := strings.Cut(string(hostname), ".")
			if !found {
				continue
			}

			volumes = append(volumes, corev1.Volume{
				Name: subdomain,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: route.Labels[v1beta1constants.LabelBasicAuthSecretName],
						Items: []corev1.KeyToPath{
							{
								Key:  secrets.DataKeyAuth,
								Path: subdomain,
							},
						},
					},
				},
			})

			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      subdomain,
				MountPath: path.Join(rootMountPath, subdomain),
				SubPath:   subdomain,
			})

			configPatches = append(configPatches, &v1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
				ApplyTo: v1alpha3.EnvoyFilter_HTTP_FILTER,
				Match: &v1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
					Context: v1alpha3.EnvoyFilter_GATEWAY,
					ObjectTypes: &v1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Listener{
						Listener: &v1alpha3.EnvoyFilter_ListenerMatch{
							PortNumber: 9443,
							FilterChain: &v1alpha3.EnvoyFilter_ListenerMatch_FilterChainMatch{
								Filter: &v1alpha3.EnvoyFilter_ListenerMatch_FilterMatch{
									Name: "envoy.filters.network.http_connection_manager",
								},
								Sni: string(hostname),
							},
						},
					},
				},
				Patch: &v1alpha3.EnvoyFilter_Patch{
					Operation:   v1alpha3.EnvoyFilter_Patch_INSERT_BEFORE,
					FilterClass: v1alpha3.EnvoyFilter_Patch_AUTHZ,
					Value: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"name": structpb.NewStringValue("envoy.filters.http.ext_authz"),
							"typed_config": structpb.NewStructValue(&structpb.Struct{
								Fields: map[string]*structpb.Value{
									"@type":                 structpb.NewStringValue("type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthz"),
									"transport_api_version": structpb.NewStringValue("V3"),
									"grpc_service": structpb.NewStructValue(&structpb.Struct{
										Fields: map[string]*structpb.Value{
											"timeout": structpb.NewStringValue("2s"),
											"envoy_grpc": structpb.NewStructValue(&structpb.Struct{
												Fields: map[string]*structpb.Value{
													"cluster_name": structpb.NewStringValue(fmt.Sprintf("outbound|10000||%s.%s.svc.cluster.local", svc.Name, svc.Namespace)),
												},
											}),
										},
									}),
								},
							}),
						},
					},
				},
			},
			)
		}
	}

	destinationHost := kubernetesutils.FQDNForService(svc.Name, svc.Namespace)
	destinationRule := &istionetworkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameExtAuthzServer, Namespace: e.namespace}}
	if err := istio.DestinationRuleWithLocalityPreference(destinationRule, getLabels(), destinationHost)(); err != nil {
		return err
	}

	resources := []client.Object{
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.DeploymentNameExtAuthzServer,
				Namespace: e.namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: getLabels(),
				},
				Replicas: &e.values.Replicas,
				Strategy: appsv1.DeploymentStrategy{
					Type: appsv1.RollingUpdateDeploymentStrategyType,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: getLabels(),
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:            "server",
								Image:           e.values.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args:            []string{"--grpc-reflection"},
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 10000,
										Name:          "grpc",
									},
								},
								VolumeMounts: volumeMounts,
							},
						},
						Volumes: volumes,
					},
				},
			},
		},
		svc,
		destinationRule,
		&istionetworkingv1alpha3.EnvoyFilter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.DeploymentNameExtAuthzServer + "-" + e.namespace,
				Namespace: v1beta1constants.DefaultSNIIngressNamespace, // TODO: can be different depending on consumer
			},
			Spec: v1alpha3.EnvoyFilter{
				ConfigPatches: configPatches,
			},
		},
	}

	serializedResources, err := registry.AddAllAndSerialize(resources...)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeedWithLabels(ctx, e.client, e.namespace, managedResourceName, false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, serializedResources)
}

func (e *extAuthz) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, e.client, e.namespace, managedResourceName)
}

var timeoutWaitForManagedResources = 2 * time.Minute

func (e *extAuthz) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, e.client, e.namespace, managedResourceName)
}

func (e *extAuthz) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, e.client, e.namespace, managedResourceName)
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:   v1beta1constants.DeploymentNameExtAuthzServer,
		v1beta1constants.LabelRole:  v1beta1constants.LabelObservability,
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleObservability,
	}
}
