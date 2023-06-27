// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// MigrateExtensionResourcesInParallel migrates extension CRs.
// CRs with kind "Extension" are handled separately and are not migrated by this function.
func (b *Botanist) MigrateExtensionResourcesInParallel(ctx context.Context) (err error) {
	return b.runParallelTaskForEachComponent(ctx, b.Shoot.GetExtensionComponentsForParallelMigration(), func(c component.DeployMigrateWaiter) func(context.Context) error {
		return c.Migrate
	})
}

// WaitUntilExtensionResourcesMigrated waits until extension CRs have been successfully migrated.
// CRs with kind "Extension" are handled separately and are not waited by this function.
func (b *Botanist) WaitUntilExtensionResourcesMigrated(ctx context.Context) error {
	return b.runParallelTaskForEachComponent(ctx, b.Shoot.GetExtensionComponentsForParallelMigration(), func(c component.DeployMigrateWaiter) func(context.Context) error {
		return c.WaitMigrate
	})
}

// DestroyExtensionResourcesInParallel deletes extension CRs from the Shoot namespace.
// CRs with kind "Extension" are handled separately and are not deleted by this function.
func (b *Botanist) DestroyExtensionResourcesInParallel(ctx context.Context) error {
	return b.runParallelTaskForEachComponent(ctx, b.Shoot.GetExtensionComponentsForParallelMigration(), func(c component.DeployMigrateWaiter) func(context.Context) error {
		return c.Destroy
	})
}

// WaitUntilExtensionResourcesDeleted waits until extension CRs have been deleted from the Shoot namespace.
// CRs with kind "Extension" are handled separately and are not waited by this function.
func (b *Botanist) WaitUntilExtensionResourcesDeleted(ctx context.Context) error {
	return b.runParallelTaskForEachComponent(ctx, b.Shoot.GetExtensionComponentsForParallelMigration(), func(c component.DeployMigrateWaiter) func(context.Context) error {
		return c.WaitCleanup
	})
}

// DestroyDNSRecords deletes all DNSRecord resources from the Shoot namespace.
func (b *Botanist) DestroyDNSRecords(ctx context.Context) error {
	return b.runParallelTaskForEachComponent(ctx, b.Shoot.GetDNSRecordComponentsForMigration(), func(c component.DeployMigrateWaiter) func(context.Context) error {
		return c.Destroy
	})
}

func (b *Botanist) runParallelTaskForEachComponent(ctx context.Context, components []component.DeployMigrateWaiter, fn func(component.DeployMigrateWaiter) func(context.Context) error) error {
	var fns []flow.TaskFn
	for _, component := range components {
		fns = append(fns, fn(component))
	}
	return flow.Parallel(fns...)(ctx)
}

// IsCopyOfBackupsRequired check if etcd backups need to be copied between seeds.
func (b *Botanist) IsCopyOfBackupsRequired(ctx context.Context) (bool, error) {
	if b.Seed.GetInfo().Spec.Backup == nil || !b.IsRestorePhase() {
		return false, nil
	}

	// First we check if the etcd-main Etcd resource has been created. This is only true if backups have been copied.
	if _, err := b.Shoot.Components.ControlPlane.EtcdMain.Get(ctx); client.IgnoreNotFound(err) != nil {
		return false, err
	} else if err == nil {
		return false, nil
	}

	backupEntry, err := b.Shoot.Components.BackupEntry.Get(ctx)
	if err != nil {
		return false, fmt.Errorf("error while retrieving BackupEntry: %w", err)
	}

	// If the Shoot's original BackupEntry has not been switched to the destination Seed's BackupBucket, then backup copying has not been started yet
	// and the source BackupEntry has not been created.
	if backupEntry.Spec.BucketName != string(b.Seed.GetInfo().UID) {
		return true, nil
	}

	sourceBackupEntry, err := b.Shoot.Components.SourceBackupEntry.Get(ctx)
	if err != nil {
		return false, fmt.Errorf("error while retrieving source BackupEntry: %w", err)
	}

	// If the source BackupEntry exists, then the Shoot's original BackupEntry must have had its bucketName switched to the BackupBucket of the
	// destination Seed and the source BackupEntry's bucketName must point to the BackupBucket of the source seed. Otherwise copy of backups is
	// impossible and data loss will occur.
	if sourceBackupEntry.Spec.BucketName == backupEntry.Spec.BucketName {
		return false, fmt.Errorf("backups have not been copied and source and target backupentry point to the same bucket: %s. ", sourceBackupEntry.Spec.BucketName)
	}

	return true, nil
}

// IsRestorePhase returns true when the shoot is in phase 'restore'.
func (b *Botanist) IsRestorePhase() bool {
	return v1beta1helper.ShootHasOperationType(b.Shoot.GetInfo().Status.LastOperation, gardencorev1beta1.LastOperationTypeRestore)
}
