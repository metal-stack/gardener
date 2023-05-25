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

	"github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
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
func ReadNodeAgentConfiguration() (*v1alpha1.NodeAgentConfiguration, error) {
	content, err := ReadTrimmedFile(v1alpha1.NodeAgentConfigPath)
	if err != nil {
		return nil, err
	}

	config := &v1alpha1.NodeAgentConfiguration{}

	err = yaml.Unmarshal([]byte(content), config)
	if err != nil {
		return nil, err
	}

	return config, nil
}
