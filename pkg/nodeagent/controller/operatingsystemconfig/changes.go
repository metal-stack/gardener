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

package operatingsystemconfig

import (
	"errors"
	"fmt"
	"slices"

	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

var decoder runtime.Decoder

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))
	decoder = serializer.NewCodecFactory(scheme).UniversalDeserializer()
}

func extractOSCFromSecret(secret *corev1.Secret) (*extensionsv1alpha1.OperatingSystemConfig, []byte, string, error) {
	oscRaw, ok := secret.Data[nodeagentv1alpha1.DataKeyOperatingSystemConfig]
	if !ok {
		return nil, nil, "", fmt.Errorf("no %s key found in OSC secret", nodeagentv1alpha1.DataKeyOperatingSystemConfig)
	}

	osc := &extensionsv1alpha1.OperatingSystemConfig{}
	if err := runtime.DecodeInto(decoder, oscRaw, osc); err != nil {
		return nil, nil, "", fmt.Errorf("unable to decode OSC from secret data key %s: %w", nodeagentv1alpha1.DataKeyOperatingSystemConfig, err)
	}

	return osc, oscRaw, secret.Annotations[nodeagentv1alpha1.AnnotationKeyChecksumDownloadedOperatingSystemConfig], nil
}

type operatingSystemConfigChanges struct {
	units units
	files files
}

type units struct {
	changed []changedUnit
	deleted []extensionsv1alpha1.Unit
}

type changedUnit struct {
	extensionsv1alpha1.Unit
	dropIns dropIns
}

type dropIns struct {
	changed []extensionsv1alpha1.DropIn
	deleted []extensionsv1alpha1.DropIn
}

type files struct {
	changed []extensionsv1alpha1.File
	deleted []extensionsv1alpha1.File
}

func computeOperatingSystemConfigChanges(fs afero.Afero, newOSC *extensionsv1alpha1.OperatingSystemConfig) (*operatingSystemConfigChanges, error) {
	changes := &operatingSystemConfigChanges{}

	// osc.files and osc.unit.files should be changed the same way by OSC controller.
	// The reason for assigning files to units is the detection of changes which require the restart of a unit.
	newOSCFiles := collectAllFiles(newOSC)

	oldOSCRaw, err := fs.ReadFile(lastAppliedOperatingSystemConfigFilePath)
	if err != nil {
		if !errors.Is(err, afero.ErrFileNotFound) {
			return nil, fmt.Errorf("error reading last applied OSC from file path %s: %w", lastAppliedOperatingSystemConfigFilePath, err)
		}

		var unitChanges []changedUnit
		for _, unit := range mergeUnits(newOSC.Spec.Units, newOSC.Status.ExtensionUnits) {
			unitChanges = append(unitChanges, changedUnit{
				Unit:    unit,
				dropIns: dropIns{changed: unit.DropIns},
			})
		}

		changes.files.changed = newOSCFiles
		changes.units.changed = unitChanges
		return changes, nil
	}

	oldOSC := &extensionsv1alpha1.OperatingSystemConfig{}
	if err := runtime.DecodeInto(decoder, oldOSCRaw, oldOSC); err != nil {
		return nil, fmt.Errorf("unable to decode the old OSC read from file path %s: %w", lastAppliedOperatingSystemConfigFilePath, err)
	}

	changes.units = computeUnitDiffs(
		mergeUnits(oldOSC.Spec.Units, oldOSC.Status.ExtensionUnits),
		mergeUnits(newOSC.Spec.Units, newOSC.Status.ExtensionUnits),
	)

	oldOSCFiles := collectAllFiles(oldOSC)
	// File changes have to be computed in one step for all files,
	// because moving a file from osc.unit.files to osc.files or vice versa should not result in a change and a delete event.
	changes.files = computeFileDiffs(oldOSCFiles, newOSCFiles)

	return changes, nil
}

func computeUnitDiffs(oldUnits, newUnits []extensionsv1alpha1.Unit) units {
	var u units

	for _, oldUnit := range oldUnits {
		if !slices.ContainsFunc(newUnits, func(newUnit extensionsv1alpha1.Unit) bool {
			return oldUnit.Name == newUnit.Name
		}) {
			u.deleted = append(u.deleted, oldUnit)
		}
	}

	for _, newUnit := range newUnits {
		oldUnitIndex := slices.IndexFunc(oldUnits, func(oldUnit extensionsv1alpha1.Unit) bool {
			return oldUnit.Name == newUnit.Name
		})

		if oldUnitIndex == -1 {
			u.changed = append(u.changed, changedUnit{
				Unit:    newUnit,
				dropIns: dropIns{changed: newUnit.DropIns},
			})
		} else if !apiequality.Semantic.DeepEqual(oldUnits[oldUnitIndex], newUnit) {
			var d dropIns

			for _, oldDropIn := range oldUnits[oldUnitIndex].DropIns {
				if !slices.ContainsFunc(newUnit.DropIns, func(newDropIn extensionsv1alpha1.DropIn) bool {
					return oldDropIn.Name == newDropIn.Name
				}) {
					d.deleted = append(d.deleted, oldDropIn)
				}
			}

			for _, newDropIn := range newUnit.DropIns {
				oldDropInIndex := slices.IndexFunc(oldUnits[oldUnitIndex].DropIns, func(oldDropIn extensionsv1alpha1.DropIn) bool {
					return oldDropIn.Name == newDropIn.Name
				})

				if oldDropInIndex == -1 || !apiequality.Semantic.DeepEqual(oldUnits[oldUnitIndex].DropIns[oldDropInIndex], newDropIn) {
					d.changed = append(d.changed, newDropIn)
					continue
				}
			}

			u.changed = append(u.changed, changedUnit{
				Unit:    newUnit,
				dropIns: d,
			})
		}
	}

	return u
}

func computeFileDiffs(oldFiles, newFiles []extensionsv1alpha1.File) files {
	var f files

	for _, oldFile := range oldFiles {
		if !slices.ContainsFunc(newFiles, func(newFile extensionsv1alpha1.File) bool {
			return oldFile.Path == newFile.Path
		}) {
			f.deleted = append(f.deleted, oldFile)
		}
	}

	for _, newFile := range newFiles {
		oldFileIndex := slices.IndexFunc(oldFiles, func(oldFile extensionsv1alpha1.File) bool {
			return oldFile.Path == newFile.Path
		})

		if oldFileIndex == -1 || !apiequality.Semantic.DeepEqual(oldFiles[oldFileIndex], newFile) {
			f.changed = append(f.changed, newFile)
			continue
		}
	}

	return f
}

func mergeUnits(specUnits, statusUnits []extensionsv1alpha1.Unit) []extensionsv1alpha1.Unit {
	var out []extensionsv1alpha1.Unit

	for _, unit := range append(specUnits, statusUnits...) {
		unitIndex := slices.IndexFunc(out, func(existingUnit extensionsv1alpha1.Unit) bool {
			return existingUnit.Name == unit.Name
		})

		if unitIndex == -1 {
			out = append(out, unit)
			continue
		}

		if unit.Enable != nil {
			out[unitIndex].Enable = unit.Enable
		}
		if unit.Command != nil {
			out[unitIndex].Command = unit.Command
		}
		if unit.Content != nil {
			out[unitIndex].Content = unit.Content
		}
		out[unitIndex].DropIns = append(out[unitIndex].DropIns, unit.DropIns...)
		out[unitIndex].Files = append(out[unitIndex].Files, unit.Files...)
	}

	return out
}

func collectUnitFiles(units []extensionsv1alpha1.Unit) []extensionsv1alpha1.File {
	var unitFiles []extensionsv1alpha1.File

	for _, unit := range units {
		unitFiles = append(unitFiles, unit.Files...)
	}

	return unitFiles
}

func collectAllFiles(osc *extensionsv1alpha1.OperatingSystemConfig) []extensionsv1alpha1.File {
	unitFiles := collectUnitFiles(mergeUnits(osc.Spec.Units, osc.Status.ExtensionUnits))
	oscFiles := append(osc.Spec.Files, osc.Status.ExtensionFiles...)
	oscFiles = append(oscFiles, unitFiles...)

	return oscFiles
}
