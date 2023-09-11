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

package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeAgentConfiguration defines the configuration for the gardener-node-agent.
type NodeAgentConfiguration struct {
	metav1.TypeMeta

	// APIServer contains the connection configuration for the gardener-node-agent to
	// access the shoot api server.
	APIServer APIServer

	// SecretName defines the name of the secret in the shoot cluster control plane, which contains
	// the OSC for the gardener-node-agent.
	OSCSecretName string

	// TokenSecretName defines the name of the secret in the shoot cluster control plane, which contains
	// the `kube-apiserver` access token for the gardener-node-agent.
	TokenSecretName string

	// Image is the container image reference to the gardener-node-agent.
	Image string
	// HyperkubeImage is the container image reference to the hyperkube containing kubelet.
	HyperkubeImage string

	// KubernetesVersion contains the kubernetes version of the kubelet, used for annotating
	// the corresponding node resource with a kubernetes version annotation.
	KubernetesVersion string

	// KubeletDataVolumeSize sets the data volume size of an unformatted disk on the worker node,
	// which is used for /var/lib on the worker.
	KubeletDataVolumeSize *int64
}

// APIServer contains the connection configuration for the gardener-node-agent to
type APIServer struct {
	// URL is the url to the api server.
	URL string
	// CA is the ca certificate for the api server.
	CA string
	// BootstrapToken is the initial token to fetch the shoot access token for
	// kubelet and the gardener-node-agent.
	BootstrapToken string
}