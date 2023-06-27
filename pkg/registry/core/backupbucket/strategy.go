// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package backupbucket

import (
	"context"
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/validation"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type backupBucketStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for BackupBuckets.
var Strategy = backupBucketStrategy{api.Scheme, names.SimpleNameGenerator}

func (backupBucketStrategy) NamespaceScoped() bool {
	return false
}

func (backupBucketStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	backupBucket := obj.(*core.BackupBucket)

	backupBucket.Generation = 1
	backupBucket.Status = core.BackupBucketStatus{}
}

func (backupBucketStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newBackupBucket := obj.(*core.BackupBucket)
	oldBackupBucket := old.(*core.BackupBucket)
	newBackupBucket.Status = oldBackupBucket.Status

	if mustIncreaseGeneration(oldBackupBucket, newBackupBucket) {
		newBackupBucket.Generation = oldBackupBucket.Generation + 1
	}
}

func mustIncreaseGeneration(oldBackupBucket, newBackupBucket *core.BackupBucket) bool {
	// The BackupBucket specification changes.
	if !apiequality.Semantic.DeepEqual(oldBackupBucket.Spec, newBackupBucket.Spec) {
		return true
	}

	// The deletion timestamp was set.
	if oldBackupBucket.DeletionTimestamp == nil && newBackupBucket.DeletionTimestamp != nil {
		return true
	}

	if kubernetesutils.HasMetaDataAnnotation(&newBackupBucket.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile) {
		delete(newBackupBucket.Annotations, v1beta1constants.GardenerOperation)
		return true
	}

	return false
}

func (backupBucketStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	backupBucket := obj.(*core.BackupBucket)
	return validation.ValidateBackupBucket(backupBucket)
}

func (backupBucketStrategy) Canonicalize(obj runtime.Object) {
}

func (backupBucketStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (backupBucketStrategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldBackupBucket, newBackupBucket := oldObj.(*core.BackupBucket), newObj.(*core.BackupBucket)
	return validation.ValidateBackupBucketUpdate(newBackupBucket, oldBackupBucket)
}

func (backupBucketStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// WarningsOnCreate returns warnings to the client performing a create.
func (backupBucketStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (backupBucketStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

type backupBucketStatusStrategy struct {
	backupBucketStrategy
}

// StatusStrategy defines the storage strategy for the status subresource of BackupBuckets.
var StatusStrategy = backupBucketStatusStrategy{Strategy}

func (backupBucketStatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newBackupBucket := obj.(*core.BackupBucket)
	oldBackupBucket := old.(*core.BackupBucket)
	newBackupBucket.Spec = oldBackupBucket.Spec
}

func (backupBucketStatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateBackupBucketStatusUpdate(obj.(*core.BackupBucket), old.(*core.BackupBucket))
}

// ToSelectableFields returns a field set that represents the object
func ToSelectableFields(backupBucket *core.BackupBucket) fields.Set {
	// The purpose of allocation with a given number of elements is to reduce
	// amount of allocations needed to create the fields.Set. If you add any
	// field here or the number of object-meta related fields changes, this should
	// be adjusted.
	backupBucketSpecificFieldsSet := make(fields.Set, 3)
	backupBucketSpecificFieldsSet[core.BackupBucketSeedName] = getSeedName(backupBucket)
	return generic.AddObjectMetaFieldsSet(backupBucketSpecificFieldsSet, &backupBucket.ObjectMeta, true)
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	backupBucket, ok := obj.(*core.BackupBucket)
	if !ok {
		return nil, nil, fmt.Errorf("not a backupBucket")
	}
	return labels.Set(backupBucket.ObjectMeta.Labels), ToSelectableFields(backupBucket), nil
}

// MatchBackupBucket returns a generic matcher for a given label and field selector.
func MatchBackupBucket(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:       label,
		Field:       field,
		GetAttrs:    GetAttrs,
		IndexFields: []string{core.BackupBucketSeedName},
	}
}

// SeedNameTriggerFunc returns spec.seedName of given BackupBucket.
func SeedNameTriggerFunc(obj runtime.Object) string {
	backupBucket, ok := obj.(*core.BackupBucket)
	if !ok {
		return ""
	}

	return getSeedName(backupBucket)
}

func getSeedName(backupBucket *core.BackupBucket) string {
	if backupBucket.Spec.SeedName == nil {
		return ""
	}
	return *backupBucket.Spec.SeedName
}
