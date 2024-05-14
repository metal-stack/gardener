// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	_ "embed"
	"fmt"

	"k8s.io/apimachinery/pkg/util/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/yaml"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

var (
	//go:embed extensions.yaml
	extensionsYAML string
	extensions     map[string]operatorv1alpha1.ExtensionSpec
)

// Extension is the default specification for an `operator.gardener.cloud/v1alpha1.Extension` object.
type Extension struct {
	// Name is the name of the extension (without `gardener-extension-` prefix).
	Name string `json:"name" yaml:"name"`
	// ExtensionSpec is the specification of the `operator.gardener.cloud/v1alpha1.Extension` object.
	operatorv1alpha1.ExtensionSpec `json:",inline" yaml:",inline"`
}

func init() {
	extensionList := struct {
		Extensions []Extension `json:"extensions" yaml:"extensions"`
	}{}
	utilruntime.Must(yaml.Unmarshal([]byte(extensionsYAML), &extensionList))

	extensions = make(map[string]operatorv1alpha1.ExtensionSpec, len(extensionList.Extensions))
	for _, extension := range extensionList.Extensions {
		extensions[extension.Name] = extension.ExtensionSpec
	}
}

// Extensions returns a map whose keys are extension names and whose values are their default specs.
func Extensions() map[string]operatorv1alpha1.ExtensionSpec {
	return extensions
}

// ExtensionSpecFor returns the spec for a given extension name. It also returns a bool indicating whether a default
// spec is known or not.
func ExtensionSpecFor(name string) (operatorv1alpha1.ExtensionSpec, bool) {
	spec, ok := Extensions()[name]
	return spec, ok
}

// MergeExtensionSpecs takes a name and a spec. If a default spec for the given extension name is known, it merges it
// with the provided spec. The provided spec always overrides fields in the default spec. If a default spec is not
// known, then the provided spec will be returned.
func MergeExtensionSpecs(name string, spec operatorv1alpha1.ExtensionSpec) (operatorv1alpha1.ExtensionSpec, error) {
	defaultSpec, ok := ExtensionSpecFor(name)
	if !ok {
		return spec, nil
	}

	defaultSpecJSON, err := json.Marshal(defaultSpec)
	if err != nil {
		return operatorv1alpha1.ExtensionSpec{}, fmt.Errorf("failed to marshal default extension spec: %w", err)
	}

	specJSON, err := json.Marshal(spec)
	if err != nil {
		return operatorv1alpha1.ExtensionSpec{}, fmt.Errorf("failed to marshal extension spec: %w", err)
	}

	resultJSON, err := strategicpatch.StrategicMergePatch(defaultSpecJSON, specJSON, &operatorv1alpha1.ExtensionSpec{})
	if err != nil {
		return operatorv1alpha1.ExtensionSpec{}, fmt.Errorf("failed to merge extension specs: %w", err)
	}

	var result operatorv1alpha1.ExtensionSpec
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		return operatorv1alpha1.ExtensionSpec{}, fmt.Errorf("failed to unmarshal extension spec: %w", err)
	}

	return result, nil
}
