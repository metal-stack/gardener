// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package common

import (
	"fmt"
	"os"
	"strings"

	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/slices"

	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"gopkg.in/yaml.v2"
)

// ReadTrimmedFile reads the file from the given path, strips the content
// and returns an error in case the file is empty.
func ReadTrimmedFile(name string) (string, error) {
	content, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		// sometimes files are empty when being replaced
		// under no circumstances these contents should be used
		// for further processing in the controllers.
		return "", fmt.Errorf("file %q is empty", name)
	}

	return trimmed, nil
}

// ReadNodeAgentConfiguration returns the node agent configuration
// as written to the worker node's file system.
func ReadNodeAgentConfiguration() (*nodeagentv1alpha1.NodeAgentConfiguration, error) {
	content, err := ReadTrimmedFile(nodeagentv1alpha1.NodeAgentConfigPath)
	if err != nil {
		return nil, err
	}

	config := &nodeagentv1alpha1.NodeAgentConfiguration{}

	err = yaml.Unmarshal([]byte(content), config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

type OSCChanges struct {
	// ChangedUnits contains units which change the content or have been added
	ChangedUnits []v1alpha1.Unit
	DeletedUnits []v1alpha1.Unit
	DeletedFiles []v1alpha1.File
}

func CalculateChangedUnitsAndRemovedFiles(currentOSC *v1alpha1.OperatingSystemConfig) (*OSCChanges, error) {

	previousOSCFile, err := os.ReadFile(nodeagentv1alpha1.NodeAgentOSCOldConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &OSCChanges{}, nil
		}
		return nil, fmt.Errorf("error retrieving previous osc from file: %w", err)
	}

	previousOSC := &v1alpha1.OperatingSystemConfig{}
	err = yaml.Unmarshal(previousOSCFile, previousOSC)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling previous osc: %w", err)
	}

	return calculateDiff(currentOSC, previousOSC), nil
}

func calculateDiff(current, previous *v1alpha1.OperatingSystemConfig) *OSCChanges {
	oscChanges := &OSCChanges{}

	for _, pf := range previous.Spec.Files {
		if !slices.ContainsFunc(current.Spec.Files, func(cf v1alpha1.File) bool {
			return pf.Path == cf.Path
		}) {
			oscChanges.DeletedFiles = append(oscChanges.DeletedFiles, pf)
		}
	}

	for _, pu := range previous.Spec.Units {
		if !slices.ContainsFunc(current.Spec.Units, func(cu v1alpha1.Unit) bool {
			return pu.Name == cu.Name
		}) {
			oscChanges.DeletedUnits = append(oscChanges.DeletedUnits, pu)
		}
	}

	for _, cu := range current.Spec.Units {
		pi := slices.IndexFunc(previous.Spec.Units, func(pu v1alpha1.Unit) bool {
			return pu.Name == cu.Name
		})
		if pi == -1 {
			oscChanges.ChangedUnits = append(oscChanges.ChangedUnits, cu)
			continue
		}
		pu := previous.Spec.Units[pi]
		if cmp.Equal(cu, pu) {
			continue
		}
		oscChanges.ChangedUnits = append(oscChanges.ChangedUnits, cu)
	}

	return oscChanges
}
