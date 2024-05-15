// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"slices"
	"strings"

	"github.com/go-test/deep"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// ValidateOperatingSystemConfig validates a OperatingSystemConfig object.
func ValidateOperatingSystemConfig(osc *extensionsv1alpha1.OperatingSystemConfig) field.ErrorList {
	pathsFromFiles := sets.New[string]()
	for _, file := range append(osc.Spec.Files, osc.Status.ExtensionFiles...) {
		pathsFromFiles.Insert(file.Path)
	}

	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&osc.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateOperatingSystemConfigSpec(&osc.Spec, pathsFromFiles, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateOperatingSystemConfigStatus(&osc.Status, pathsFromFiles, field.NewPath("status"))...)

	allErrs = append(allErrs, validateFileDuplicates(osc)...)

	return allErrs
}

// ValidateOperatingSystemConfigUpdate validates a OperatingSystemConfig object before an update.
func ValidateOperatingSystemConfigUpdate(new, old *extensionsv1alpha1.OperatingSystemConfig) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateOperatingSystemConfigSpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateOperatingSystemConfig(new)...)

	return allErrs
}

// ValidateOperatingSystemConfigSpec validates the specification of a OperatingSystemConfig object.
func ValidateOperatingSystemConfigSpec(spec *extensionsv1alpha1.OperatingSystemConfigSpec, pathsFromFiles sets.Set[string], fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "field is required"))
	}

	if len(spec.Purpose) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("purpose"), "field is required"))
	} else {
		if spec.Purpose != extensionsv1alpha1.OperatingSystemConfigPurposeProvision && spec.Purpose != extensionsv1alpha1.OperatingSystemConfigPurposeReconcile {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("purpose"), spec.Purpose, []string{string(extensionsv1alpha1.OperatingSystemConfigPurposeProvision), string(extensionsv1alpha1.OperatingSystemConfigPurposeReconcile)}))
		}
	}

	allErrs = append(allErrs, ValidateCriConfig(spec.CRIConfig, fldPath.Child("criConfig"))...)
	allErrs = append(allErrs, ValidateUnits(spec.Units, pathsFromFiles, fldPath.Child("units"))...)
	allErrs = append(allErrs, ValidateFiles(spec.Files, fldPath.Child("files"))...)

	return allErrs
}

// ValidateOperatingSystemConfigStatus validates the status of a OperatingSystemConfig object.
func ValidateOperatingSystemConfigStatus(status *extensionsv1alpha1.OperatingSystemConfigStatus, pathsFromFiles sets.Set[string], fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateUnits(status.ExtensionUnits, pathsFromFiles, fldPath.Child("extensionUnits"))...)
	allErrs = append(allErrs, ValidateFiles(status.ExtensionFiles, fldPath.Child("extensionFiles"))...)

	return allErrs
}

// ValidateUnits validates operating system config units.
func ValidateUnits(units []extensionsv1alpha1.Unit, pathsFromFiles sets.Set[string], fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, unit := range units {
		idxPath := fldPath.Index(i)

		if len(unit.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "field is required"))
		}

		for j, dropIn := range unit.DropIns {
			jdxPath := idxPath.Child("dropIns").Index(j)

			if len(dropIn.Name) == 0 {
				allErrs = append(allErrs, field.Required(jdxPath.Child("name"), "field is required"))
			}
			if len(dropIn.Content) == 0 {
				allErrs = append(allErrs, field.Required(jdxPath.Child("content"), "field is required"))
			}
		}

		allErrs = append(allErrs, validateFilePaths(unit.FilePaths, pathsFromFiles, idxPath.Child("filePaths"))...)
	}

	return allErrs
}

func validateFileDuplicates(osc *extensionsv1alpha1.OperatingSystemConfig) field.ErrorList {
	allErrs := field.ErrorList{}

	paths := sets.New[string]()

	check := func(files []extensionsv1alpha1.File, fldPath *field.Path) {
		for i, file := range files {
			idxPath := fldPath.Index(i)

			if file.Path != "" {
				if paths.Has(file.Path) {
					allErrs = append(allErrs, field.Duplicate(idxPath.Child("path"), file.Path))
				}

				paths.Insert(file.Path)
			}
		}
	}

	check(osc.Spec.Files, field.NewPath("spec.files"))
	check(osc.Status.ExtensionFiles, field.NewPath("status.extensionFiles"))

	return allErrs
}

func validateFilePaths(filePaths []string, paths sets.Set[string], fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, filePath := range filePaths {
		idxPath := fldPath.Index(i)
		if !paths.Has(filePath) {
			allErrs = append(allErrs, field.Invalid(idxPath, filePath, "'filePath' requires a matching 'path' in 'spec.files'"))
		}
	}

	return allErrs
}

// ValidateFiles validates operating system config files.
func ValidateFiles(files []extensionsv1alpha1.File, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, file := range files {
		idxPath := fldPath.Index(i)

		if len(file.Path) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("path"), "field is required"))
		}

		switch {
		case file.Content.SecretRef == nil && file.Content.Inline == nil && file.Content.ImageRef == nil:
			allErrs = append(allErrs, field.Required(idxPath.Child("content"), "either 'secretRef', 'inline' or 'imageRef' must be provided"))
		case file.Content.SecretRef != nil && file.Content.Inline != nil || file.Content.SecretRef != nil && file.Content.ImageRef != nil || file.Content.Inline != nil && file.Content.ImageRef != nil:
			allErrs = append(allErrs, field.Invalid(idxPath.Child("content"), file.Content, "either 'secretRef', 'inline' or 'imageRef' must be provided, not multiple at the same time"))
		case file.Content.SecretRef != nil:
			if len(file.Content.SecretRef.Name) == 0 {
				allErrs = append(allErrs, field.Required(idxPath.Child("content", "secretRef", "name"), "field is required"))
			}
			if len(file.Content.SecretRef.DataKey) == 0 {
				allErrs = append(allErrs, field.Required(idxPath.Child("content", "secretRef", "dataKey"), "field is required"))
			}
		case file.Content.Inline != nil:
			encodings := []string{string(extensionsv1alpha1.PlainFileCodecID), string(extensionsv1alpha1.B64FileCodecID)}
			if !slices.Contains(encodings, file.Content.Inline.Encoding) {
				allErrs = append(allErrs, field.NotSupported(idxPath.Child("content", "inline", "encoding"), file.Content.Inline.Encoding, encodings))
			}

			if len(file.Content.Inline.Data) == 0 {
				allErrs = append(allErrs, field.Required(idxPath.Child("content", "inline", "data"), "field is required"))
			}
		case file.Content.ImageRef != nil:
			if len(file.Content.ImageRef.Image) == 0 {
				allErrs = append(allErrs, field.Required(idxPath.Child("content", "imageRef", "image"), "field is required"))
			}
			if len(file.Content.ImageRef.FilePathInImage) == 0 {
				allErrs = append(allErrs, field.Required(idxPath.Child("content", "imageRef", "filePathInImage"), "field is required"))
			}
		}
	}

	return allErrs
}

// ValidateOperatingSystemConfigSpecUpdate validates the spec of a OperatingSystemConfig object before an update.
func ValidateOperatingSystemConfigSpecUpdate(new, old *extensionsv1alpha1.OperatingSystemConfigSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		if diff := deep.Equal(new, old); diff != nil {
			return field.ErrorList{field.Forbidden(fldPath, strings.Join(diff, ","))}
		}
		return apivalidation.ValidateImmutableField(new, old, fldPath)
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Type, old.Type, fldPath.Child("type"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Purpose, old.Purpose, fldPath.Child("purpose"))...)

	return allErrs
}

func ValidateCriConfig(config *extensionsv1alpha1.CRIConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config == nil {
		return allErrs
	}

	allErrs = append(allErrs, ValidateContainerdConfig(config.Containerd, fldPath.Child("containerd"))...)

	return allErrs
}

func ValidateContainerdConfig(config *extensionsv1alpha1.ContainerdConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config == nil {
		return allErrs
	}

	// TODO: implement!

	// Verify cri name is set and valid
	// Verify cgroup driver name is set and valid

	// Verify registry upstream is unique
	// Verify capabilities are valid
	// Verify URL is parsable URL
	// Verify server is parsable URL

	// Verify sandbox image is set

	// Verify that path is not empty
	// Verify that plugin values are of type map[string]any

	return allErrs
}
