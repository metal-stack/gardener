// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package nodeagent

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// GenerateRBACResourcesData returns a map of serialized Kubernetes resources that allow the gardener-node-agent to
// access the list of given secrets. Additionally, serialized resources providing permissions to allow initiating the
// Kubernetes TLS bootstrapping process will be returned.
func GenerateRBACResourcesData(secretNames []string) (map[string][]byte, error) {
	var (
		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener-node-agent",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"nodes"},
					Verbs:     []string{"get", "list", "watch", "update", "patch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"get", "list", "watch", "create", "patch", "update"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener-node-agent",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      AccessSecretName,
				Namespace: metav1.NamespaceSystem,
			}},
		}

		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-node-agent",
				Namespace: metav1.NamespaceSystem,
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "list", "watch"},
			}},
		}

		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-node-agent",
				Namespace: metav1.NamespaceSystem,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "Role",
				Name:     role.Name,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: rbacv1.GroupKind,
					Name: bootstraptokenapi.BootstrapDefaultGroup,
				},
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      AccessSecretName,
					Namespace: metav1.NamespaceSystem,
				},
			},
		}

		clusterRoleBindingNodeBootstrapper = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:node-bootstrapper",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "ClusterRole",
				Name:     "system:node-bootstrapper",
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     rbacv1.GroupKind,
				Name:     bootstraptokenapi.BootstrapDefaultGroup,
			}},
		}

		clusterRoleBindingNodeClient = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:certificates.k8s.io:certificatesigningrequests:nodeclient",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "ClusterRole",
				Name:     "system:certificates.k8s.io:certificatesigningrequests:nodeclient",
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     rbacv1.GroupKind,
				Name:     bootstraptokenapi.BootstrapDefaultGroup,
			}},
		}

		clusterRoleBindingSelfNodeClient = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:certificates.k8s.io:certificatesigningrequests:selfnodeclient",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "ClusterRole",
				Name:     "system:certificates.k8s.io:certificatesigningrequests:selfnodeclient",
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     rbacv1.GroupKind,
				Name:     user.NodesGroup,
			}},
		}
	)

	return managedresources.
		NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer).
		AddAllAndSerialize(
			clusterRole,
			clusterRoleBinding,
			role,
			roleBinding,
			clusterRoleBindingNodeBootstrapper,
			clusterRoleBindingNodeClient,
			clusterRoleBindingSelfNodeClient,
		)
}
