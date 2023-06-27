// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/validation"
)

// Strategy defines the strategy for storing seeds.
type Strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// NewStrategy defines the storage strategy for Seeds.
func NewStrategy() Strategy {
	return Strategy{api.Scheme, names.SimpleNameGenerator}
}

// NamespaceScoped returns true if the object must be within a namespace.
func (Strategy) NamespaceScoped() bool {
	return false
}

// PrepareForCreate mutates some fields in the object before it's created.
func (s Strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	seed := obj.(*core.Seed)

	seed.Generation = 1
	syncDependencyWatchdogSettings(seed)
	seed.Status = core.SeedStatus{}
}

// PrepareForUpdate is invoked on update before validation to normalize
// the object.  For example: remove fields that are not to be persisted,
// sort order-insensitive list fields, etc.  This should not remove fields
// whose presence would be considered a validation error.
func (s Strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newSeed := obj.(*core.Seed)
	oldSeed := old.(*core.Seed)
	syncDependencyWatchdogSettings(newSeed)
	newSeed.Status = oldSeed.Status

	if mustIncreaseGeneration(oldSeed, newSeed) {
		newSeed.Generation = oldSeed.Generation + 1
	}
}

func mustIncreaseGeneration(oldSeed, newSeed *core.Seed) bool {
	// The spec changed
	if !apiequality.Semantic.DeepEqual(oldSeed.Spec, newSeed.Spec) {
		return true
	}

	// The deletion timestamp was set
	if oldSeed.DeletionTimestamp == nil && newSeed.DeletionTimestamp != nil {
		return true
	}

	// bump the generation in case certain operations were triggered
	if oldSeed.Annotations[v1beta1constants.GardenerOperation] != newSeed.Annotations[v1beta1constants.GardenerOperation] {
		switch newSeed.Annotations[v1beta1constants.GardenerOperation] {
		case v1beta1constants.SeedOperationRenewGardenAccessSecrets:
			return true
		}
	}

	return false
}

func syncDependencyWatchdogSettings(seed *core.Seed) {
	if seed.Spec.Settings == nil || seed.Spec.Settings.DependencyWatchdog == nil {
		return
	}
	// keeping the field `weeder` and `endpoint` in sync
	// TODO(himanshu-kun): Once the deprecated `Endpoint` / `Probe` fields are removed from the API, move the defaulting code back to `pkg/apis/core/*/defaults.go`.
	// Case 1: If weeder is specified, endpoint isn't -> set endpoint=weeder
	// Case 2: If weeder isn't specified, endpoint is -> make weeder=endpoint
	// Case 3: If both are specified, give preference to weeder field -> set endpoint=weeder
	// Case 4: If both not specified, default weeder.enabled to true
	if seed.Spec.Settings.DependencyWatchdog.Weeder == nil {
		seed.Spec.Settings.DependencyWatchdog.Weeder = &core.SeedSettingDependencyWatchdogWeeder{Enabled: true}
		if seed.Spec.Settings.DependencyWatchdog.Endpoint != nil {
			seed.Spec.Settings.DependencyWatchdog.Weeder = &core.SeedSettingDependencyWatchdogWeeder{Enabled: seed.Spec.Settings.DependencyWatchdog.Endpoint.Enabled}
		}
	}
	seed.Spec.Settings.DependencyWatchdog.Endpoint = &core.SeedSettingDependencyWatchdogEndpoint{Enabled: seed.Spec.Settings.DependencyWatchdog.Weeder.Enabled}

	// keeping the field `prober` and `probe` in sync
	// Case 1: If prober is specified, probe isn't -> set probe=prober
	// Case 2: If prober isn't specified, probe is -> make prober=probe
	// Case 3: If both are specified, give preference to prober field -> set probe=prober
	// Case 4: If both not specified, default prober.enabled to true
	if seed.Spec.Settings.DependencyWatchdog.Prober == nil {
		seed.Spec.Settings.DependencyWatchdog.Prober = &core.SeedSettingDependencyWatchdogProber{Enabled: true}
		if seed.Spec.Settings.DependencyWatchdog.Probe != nil {
			seed.Spec.Settings.DependencyWatchdog.Prober = &core.SeedSettingDependencyWatchdogProber{Enabled: seed.Spec.Settings.DependencyWatchdog.Probe.Enabled}
		}
	}
	seed.Spec.Settings.DependencyWatchdog.Probe = &core.SeedSettingDependencyWatchdogProbe{Enabled: seed.Spec.Settings.DependencyWatchdog.Prober.Enabled}
}

// Validate validates the given object.
func (Strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	seed := obj.(*core.Seed)
	return validation.ValidateSeed(seed)
}

// Canonicalize allows an object to be mutated into a canonical form. This
// ensures that code that operates on these objects can rely on the common
// form for things like comparison.  Canonicalize is invoked after
// validation has succeeded but before the object has been persisted.
// This method may mutate the object.
func (Strategy) Canonicalize(obj runtime.Object) {
	seed := obj.(*core.Seed)
	dropOwnerChecksField(seed)
}

func dropOwnerChecksField(seed *core.Seed) {
	if seed.Spec.Settings != nil && seed.Spec.Settings.OwnerChecks != nil {
		seed.Spec.Settings.OwnerChecks = nil
	}
}

// AllowCreateOnUpdate returns true if the object can be created by a PUT.
func (Strategy) AllowCreateOnUpdate() bool {
	return false
}

// AllowUnconditionalUpdate returns true if the object can be updated
// unconditionally (irrespective of the latest resource version), when
// there is no resource version specified in the object.
func (Strategy) AllowUnconditionalUpdate() bool {
	return true
}

// ValidateUpdate validates the update on the given old and new object.
func (Strategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldSeed, newSeed := oldObj.(*core.Seed), newObj.(*core.Seed)
	return validation.ValidateSeedUpdate(newSeed, oldSeed)
}

// WarningsOnCreate returns warnings to the client performing a create.
func (Strategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (Strategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

// StatusStrategy defines the strategy for storing seeds statuses.
type StatusStrategy struct {
	Strategy
}

// NewStatusStrategy defines the storage strategy for the status subresource of Seeds.
func NewStatusStrategy() StatusStrategy {
	return StatusStrategy{NewStrategy()}
}

// PrepareForUpdate is invoked on update before validation to normalize
// the object.  For example: remove fields that are not to be persisted,
// sort order-insensitive list fields, etc.  This should not remove fields
// whose presence would be considered a validation error.
func (s StatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newSeed := obj.(*core.Seed)
	oldSeed := old.(*core.Seed)
	newSeed.Spec = oldSeed.Spec
	syncDependencyWatchdogSettings(newSeed)
}

// ValidateUpdate validates the update on the given old and new object.
func (StatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateSeedStatusUpdate(obj.(*core.Seed), old.(*core.Seed))
}
