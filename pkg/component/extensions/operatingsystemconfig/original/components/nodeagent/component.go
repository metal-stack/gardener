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
	"fmt"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

// AccessSecretName is a constant for the secret name for the gardener-node-agent's shoot access secret.
const AccessSecretName = "gardener-node-agent"

var codec runtime.Codec

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(nodeagentv1alpha1.AddToScheme(scheme))

	ser := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
	versions := schema.GroupVersions([]schema.GroupVersion{nodeagentv1alpha1.SchemeGroupVersion})
	codec = serializer.NewCodecFactory(scheme).CodecForVersions(ser, ser, versions, versions)
}

type component struct{}

// New returns a new Gardener user component.
func New() *component {
	return &component{}
}

func (component) Name() string {
	return "gardener-node-agent"
}

func (component) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	var caBundle []byte
	if ctx.CABundle != nil {
		caBundle = []byte(*ctx.CABundle)
	}

	files, err := Files(ComponentConfig(ctx.Key, ctx.KubernetesVersion, ctx.APIServerURL, caBundle, ctx.OSCSyncJitterPeriod))
	if err != nil {
		return nil, nil, fmt.Errorf("failed generating files: %w", err)
	}

	return []extensionsv1alpha1.Unit{{
			Name:    nodeagentv1alpha1.UnitName,
			Enable:  pointer.Bool(true),
			Content: pointer.String(UnitContent()),
			Files: append(files, extensionsv1alpha1.File{
				Path:        v1beta1constants.OperatingSystemConfigFilePathBinaries + "/gardener-node-agent",
				Permissions: pointer.Int32(0755),
				Content: extensionsv1alpha1.FileContent{
					ImageRef: &extensionsv1alpha1.FileContentImageRef{
						Image:           ctx.Images[imagevector.ImageNameGardenerNodeAgent].String(),
						FilePathInImage: "/ko-app/gardener-node-agent",
					},
				},
			}),
		}},
		// Return unit files also as regular files to make migration from cloud-config-downloader to gardener-node-agent
		// work (CCD does not understand unit files).
		// TODO(rfranzke): Return nil once UseGardenerNodeAgent feature gate gets removed.
		files,
		nil
}

// UnitContent returns the systemd unit content for the gardener-node-agent unit.
func UnitContent() string {
	return `[Unit]
Description=Gardener Node Agent
After=network-online.target

[Service]
LimitMEMLOCK=infinity
ExecStart=` + nodeagentv1alpha1.BinaryDir + `/gardener-node-agent --config=` + nodeagentv1alpha1.ConfigFilePath + `
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target`
}

// ComponentConfig returns the component configuration for the gardener-node-agent.
func ComponentConfig(
	oscSecretName string,
	kubernetesVersion *semver.Version,
	apiServerURL string,
	caBundle []byte,
	syncJitterPeriod *metav1.Duration,
) *nodeagentv1alpha1.NodeAgentConfiguration {
	return &nodeagentv1alpha1.NodeAgentConfiguration{
		APIServer: nodeagentv1alpha1.APIServer{
			Server:   apiServerURL,
			CABundle: caBundle,
		},
		Controllers: nodeagentv1alpha1.ControllerConfiguration{
			OperatingSystemConfig: nodeagentv1alpha1.OperatingSystemConfigControllerConfig{
				SecretName:        oscSecretName,
				KubernetesVersion: kubernetesVersion,
				SyncJitterPeriod:  syncJitterPeriod,
			},
			Token: nodeagentv1alpha1.TokenControllerConfig{
				SecretName: AccessSecretName,
			},
		},
	}
}

// Files returns the files related to the gardener-node-agent unit.
func Files(config *nodeagentv1alpha1.NodeAgentConfiguration) ([]extensionsv1alpha1.File, error) {
	configRaw, err := runtime.Encode(codec, config)
	if err != nil {
		return nil, fmt.Errorf("failed encoding component config: %w", err)
	}

	return []extensionsv1alpha1.File{{
		Path:        nodeagentv1alpha1.ConfigFilePath,
		Permissions: pointer.Int32(0600),
		Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64(configRaw)}},
	}}, nil
}
