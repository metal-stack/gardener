// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
)

// ValidateControllerDeployment validates a ControllerDeployment object.
func ValidateControllerDeployment(controllerDeployment *core.ControllerDeployment) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&controllerDeployment.ObjectMeta, false, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)

	var (
		isBuiltInType  = false
		deploymentType = controllerDeployment.Type
	)

	switch {
	case controllerDeployment.Helm != nil:
		isBuiltInType = true
		deploymentType = "helm"

		allErrs = append(allErrs, validateHelmControllerDeployment(controllerDeployment.Helm, field.NewPath("helm"))...)
	}

	if isBuiltInType {
		// a built-in type is configured: type and providerConfig must be empty
		if len(controllerDeployment.Type) > 0 {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("type"), fmt.Sprintf("must not provide type if a built-in deployment type (%s) is used", deploymentType)))
		}
		if controllerDeployment.ProviderConfig != nil {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("providerConfig"), fmt.Sprintf("must not provide providerConfig if a built-in deployment type (%s) is used", deploymentType)))
		}
	} else if len(controllerDeployment.Type) == 0 {
		allErrs = append(allErrs, field.Forbidden(field.NewPath(""), "must use either helm or a custom deployment configuration"))
	}
	// If a custom type is configured, only type and providerConfig can be set, and other fields must be empty.
	// We don't need to validate this case, as it is covered by the built-in type case. In other words, configuring a
	// built-in type takes precedence over configuring a custom type in the validation.

	return allErrs
}

// ValidateControllerDeploymentUpdate validates a ControllerDeployment object before an update.
func ValidateControllerDeploymentUpdate(new, _ *core.ControllerDeployment) field.ErrorList {
	return ValidateControllerDeployment(new)
}

func validateHelmControllerDeployment(helmControllerDeployment *core.HelmControllerDeployment, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(helmControllerDeployment.RawChart) == 0 && helmControllerDeployment.OCIRepository == nil {
		allErrs = append(allErrs, field.Required(fldPath, "must provide either rawChart or ociRepository must be set"))
	}

	allErrs = append(allErrs, validateOCIRepository(helmControllerDeployment.OCIRepository, fldPath.Child("ociRepository"))...)

	return allErrs
}

func validateOCIRepository(oci *core.OCIRepository, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if oci == nil {
		return allErrs
	}

	if oci.URL == "" && oci.Repository == "" {
		allErrs = append(allErrs, field.Required(fldPath, "must provide either url or repository"))
	}

	if oci.URL != "" {
		// TODO: check that other fields are empty
		return allErrs
	}

	if oci.Repository == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("repository"), ""))
	}
	if oci.Tag == "" && oci.Digest == "" {
		allErrs = append(allErrs, field.Required(fldPath, "must provide either tag or digest must be set"))
	}
	if oci.Digest != "" && !strings.HasPrefix(oci.Digest, "sha256:") {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("digest"), oci.Digest, "must start with 'sha256:'"))
	}

	return allErrs
}
