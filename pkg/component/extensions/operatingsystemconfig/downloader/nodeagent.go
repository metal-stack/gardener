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

package downloader

import (
	_ "embed"

	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/Masterminds/sprig"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"sigs.k8s.io/yaml"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

var (
	agentTplName = "gardener-node-agent-template"
	//go:embed templates/scripts/gardener-node-init.tpl.sh
	agentTplContent string
	agentTpl        *template.Template
)

func init() {
	var err error
	agentTpl, err = template.
		New(agentTplName).
		Funcs(sprig.TxtFuncMap()).
		Parse(agentTplContent)
	if err != nil {
		panic(err)
	}
}

func NodeAgentConfig(cloudConfigUserDataSecretName, oscSecretName, apiServerURL, clusterCA string, imageVector imagevector.ImageVector, worker gardencorev1beta1.Worker, kubernetesVersion *semver.Version) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	nodeAgentImage, err := imageVector.FindImage(images.ImageNameGardenerNodeAgent)
	if err != nil {
		return nil, nil, err
	}

	hyperKubeImage, err := imageVector.FindImage(images.ImageNameHyperkube, imagevector.RuntimeVersion(kubernetesVersion.String()), imagevector.TargetVersion(kubernetesVersion.String()))
	if err != nil {
		return nil, nil, err
	}

	config := &nodeagentv1alpha1.NodeAgentConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NodeAgentConfiguration",
			APIVersion: "nodeagent.config.gardener.cloud/v1alpha1",
		},
		APIServer: nodeagentv1alpha1.APIServer{
			URL:            apiServerURL,
			CA:             clusterCA,
			BootstrapToken: BootstrapTokenPlaceholder,
		},
		OSCSecretName:     oscSecretName,
		TokenSecretName:   nodeagentv1alpha1.NodeAgentTokenSecretName,
		Image:             nodeAgentImage.String(),
		HyperkubeImage:    hyperKubeImage.String(),
		KubernetesVersion: kubernetesVersion.String(),
	}

	if len(worker.DataVolumes) > 0 && worker.KubeletDataVolumeName != nil {
		var (
			sizeInBytes int64
			found       bool
		)

		for _, dv := range worker.DataVolumes {
			if dv.Name == *worker.KubeletDataVolumeName {
				size := dv.VolumeSize
				parsed, err := resource.ParseQuantity(size)
				if err != nil {
					continue
				}
				sizeInBytes, found = parsed.AsInt64()
			}
		}
		if found {
			config.KubeletDataVolumeSize = &sizeInBytes
		}
	}

	raw, err := yaml.Marshal(config)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal node agent configuration: %w", err)
	}

	layerURL, binaryPath, err := imageRefToLayerURL(nodeAgentImage.String(), pointer.StringDeref(worker.Machine.Architecture, ""))
	if err != nil {
		return nil, nil, err
	}

	var initScript bytes.Buffer
	if err := agentTpl.Execute(&initScript, map[string]string{
		"layerURL":       layerURL.String(),
		"binaryPath":     binaryPath,
		"nodeAgentImage": nodeAgentImage.String(),
	}); err != nil {
		return nil, nil, err
	}

	units := []extensionsv1alpha1.Unit{
		GetNodeAgentInitUnit(),
	}

	files := []extensionsv1alpha1.File{
		{
			Path:        nodeagentv1alpha1.NodeAgentInitScriptPath,
			Permissions: pointer.Int32(0744),
			Content: extensionsv1alpha1.FileContent{
				Inline: &extensionsv1alpha1.FileContentInline{
					Encoding: "b64",
					Data:     utils.EncodeBase64(initScript.Bytes()),
				},
			},
		},
		{
			Path:        nodeagentv1alpha1.NodeAgentConfigPath,
			Permissions: pointer.Int32(0644),
			Content: extensionsv1alpha1.FileContent{
				Inline: &extensionsv1alpha1.FileContentInline{
					Data: string(raw),
				},
				TransmitUnencoded: pointer.Bool(true), // cannot be encoded because otherwise the MCM cannot inject the bootstrap token into the placeholder
			},
		},
	}

	return units, files, nil
}

func GetNodeAgentInitUnit() extensionsv1alpha1.Unit {
	return extensionsv1alpha1.Unit{
		Name:    nodeagentv1alpha1.NodeInitUnitName,
		Command: pointer.String("start"),
		Enable:  pointer.Bool(true),
		Content: pointer.String(`[Unit]
Description=Downloads the gardener-node-agent binary from the registry and bootstraps it.
After=network-online.target
Wants=network-online.target
[Service]
Restart=always
RestartSec=30
RuntimeMaxSec=120
EnvironmentFile=/etc/environment
ExecStart=` + nodeagentv1alpha1.NodeAgentInitScriptPath + `
[Install]
WantedBy=multi-user.target`),
	}
}

func nodeAgentRBACResources() []client.Object {
	var (
		// TODO check if we can move the RBAC in one ClusterRole
		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-node-agent",
				Namespace: metav1.NamespaceSystem,
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
				{
					// TODO: check if this can be narrowed down to osc secret + token secret in kube-system namespace
					APIGroups: []string{""},
					Resources: []string{"secrets"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-node-agent",
				Namespace: metav1.NamespaceSystem,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: rbacv1.GroupKind,
					Name: bootstraptokenapi.BootstrapDefaultGroup,
				},
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      "gardener-node-agent",
					Namespace: metav1.NamespaceSystem,
				},
			},
		}
	)

	return []client.Object{
		clusterRole,
		clusterRoleBinding,
	}
}

func imageRefToLayerURL(image string, arch string) (*url.URL, string, error) {
	// TODO(rfranzke): figure this out after breakfast
	imageRef, err := name.ParseReference(strings.ReplaceAll(image, "localhost:5001", "garden.local.gardener.cloud:5001"), name.Insecure)
	if err != nil {
		return nil, "", err
	}

	if arch == "" {
		arch = v1beta1constants.ArchitectureAMD64
	}

	remoteImage, err := remote.Image(imageRef, remote.WithPlatform(v1.Platform{OS: "linux", Architecture: arch}))
	if err != nil {
		return nil, "", err
	}

	imageConfig, err := remoteImage.ConfigFile()
	if err != nil {
		return nil, "", err
	}
	entrypoint := imageConfig.Config.Entrypoint[0]

	manifest, err := remoteImage.Manifest()
	if err != nil {
		return nil, "", err
	}

	finalLayer := manifest.Layers[len(manifest.Layers)-1]

	// This is what the library does internally as well. It doesn't expose a func for it though.
	return &url.URL{
		Scheme: imageRef.Context().Scheme(),
		Host:   imageRef.Context().RegistryStr(),
		Path:   fmt.Sprintf("/v2/%s/%s/%s", imageRef.Context().RepositoryStr(), "blobs", finalLayer.Digest),
	}, entrypoint, nil
}
