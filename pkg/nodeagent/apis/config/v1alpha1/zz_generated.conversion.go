//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by conversion-gen. DO NOT EDIT.

package v1alpha1

import (
	unsafe "unsafe"

	semver "github.com/Masterminds/semver"
	config "github.com/gardener/gardener/pkg/nodeagent/apis/config"
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func init() {
	localSchemeBuilder.Register(RegisterConversions)
}

// RegisterConversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterConversions(s *runtime.Scheme) error {
	if err := s.AddGeneratedConversionFunc((*APIServer)(nil), (*config.APIServer)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_APIServer_To_config_APIServer(a.(*APIServer), b.(*config.APIServer), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.APIServer)(nil), (*APIServer)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_APIServer_To_v1alpha1_APIServer(a.(*config.APIServer), b.(*APIServer), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*NodeAgentConfiguration)(nil), (*config.NodeAgentConfiguration)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_NodeAgentConfiguration_To_config_NodeAgentConfiguration(a.(*NodeAgentConfiguration), b.(*config.NodeAgentConfiguration), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.NodeAgentConfiguration)(nil), (*NodeAgentConfiguration)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_NodeAgentConfiguration_To_v1alpha1_NodeAgentConfiguration(a.(*config.NodeAgentConfiguration), b.(*NodeAgentConfiguration), scope)
	}); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1alpha1_APIServer_To_config_APIServer(in *APIServer, out *config.APIServer, s conversion.Scope) error {
	out.URL = in.URL
	out.CABundle = *(*[]byte)(unsafe.Pointer(&in.CABundle))
	out.BootstrapToken = in.BootstrapToken
	return nil
}

// Convert_v1alpha1_APIServer_To_config_APIServer is an autogenerated conversion function.
func Convert_v1alpha1_APIServer_To_config_APIServer(in *APIServer, out *config.APIServer, s conversion.Scope) error {
	return autoConvert_v1alpha1_APIServer_To_config_APIServer(in, out, s)
}

func autoConvert_config_APIServer_To_v1alpha1_APIServer(in *config.APIServer, out *APIServer, s conversion.Scope) error {
	out.URL = in.URL
	out.CABundle = *(*[]byte)(unsafe.Pointer(&in.CABundle))
	out.BootstrapToken = in.BootstrapToken
	return nil
}

// Convert_config_APIServer_To_v1alpha1_APIServer is an autogenerated conversion function.
func Convert_config_APIServer_To_v1alpha1_APIServer(in *config.APIServer, out *APIServer, s conversion.Scope) error {
	return autoConvert_config_APIServer_To_v1alpha1_APIServer(in, out, s)
}

func autoConvert_v1alpha1_NodeAgentConfiguration_To_config_NodeAgentConfiguration(in *NodeAgentConfiguration, out *config.NodeAgentConfiguration, s conversion.Scope) error {
	if err := Convert_v1alpha1_APIServer_To_config_APIServer(&in.APIServer, &out.APIServer, s); err != nil {
		return err
	}
	out.OperatingSystemConfigSecretName = in.OperatingSystemConfigSecretName
	out.AccessTokenSecretName = in.AccessTokenSecretName
	out.Image = in.Image
	out.HyperkubeImage = in.HyperkubeImage
	out.KubernetesVersion = (*semver.Version)(unsafe.Pointer(in.KubernetesVersion))
	out.KubeletDataVolumeSize = (*int64)(unsafe.Pointer(in.KubeletDataVolumeSize))
	return nil
}

// Convert_v1alpha1_NodeAgentConfiguration_To_config_NodeAgentConfiguration is an autogenerated conversion function.
func Convert_v1alpha1_NodeAgentConfiguration_To_config_NodeAgentConfiguration(in *NodeAgentConfiguration, out *config.NodeAgentConfiguration, s conversion.Scope) error {
	return autoConvert_v1alpha1_NodeAgentConfiguration_To_config_NodeAgentConfiguration(in, out, s)
}

func autoConvert_config_NodeAgentConfiguration_To_v1alpha1_NodeAgentConfiguration(in *config.NodeAgentConfiguration, out *NodeAgentConfiguration, s conversion.Scope) error {
	if err := Convert_config_APIServer_To_v1alpha1_APIServer(&in.APIServer, &out.APIServer, s); err != nil {
		return err
	}
	out.OperatingSystemConfigSecretName = in.OperatingSystemConfigSecretName
	out.AccessTokenSecretName = in.AccessTokenSecretName
	out.Image = in.Image
	out.HyperkubeImage = in.HyperkubeImage
	out.KubernetesVersion = (*semver.Version)(unsafe.Pointer(in.KubernetesVersion))
	out.KubeletDataVolumeSize = (*int64)(unsafe.Pointer(in.KubeletDataVolumeSize))
	return nil
}

// Convert_config_NodeAgentConfiguration_To_v1alpha1_NodeAgentConfiguration is an autogenerated conversion function.
func Convert_config_NodeAgentConfiguration_To_v1alpha1_NodeAgentConfiguration(in *config.NodeAgentConfiguration, out *NodeAgentConfiguration, s conversion.Scope) error {
	return autoConvert_config_NodeAgentConfiguration_To_v1alpha1_NodeAgentConfiguration(in, out, s)
}
