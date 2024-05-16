// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	"github.com/gardener/gardener/pkg/provider-local/controller/backupoptions"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

// ControllerName is the name of the controller.
const ControllerName = "backupbucket_controller"

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = backupoptions.AddOptions{}
)

// AddOptions are options to apply when adding the backupbucket controller to the manager.
type AddOptions struct {
	// BackupBucketLocalDir is the directory of the backupbucket.
	BackupBucketLocalDir string
	// Controller are the controller.Options.
	Controller controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(ctx context.Context, mgr manager.Manager, opts backupoptions.AddOptions) error {
	return backupbucket.Add(ctx, mgr, backupbucket.AddArgs{
		Actuator:          newActuator(mgr, opts.BackupBucketPath),
		ControllerOptions: opts.Controller,
		Predicates:        backupbucket.DefaultPredicates(opts.IgnoreOperationAnnotation),
		Type:              local.Type,
		Class:             opts.Class,
	})
}

// AddToManager adds a controller with the default Options.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}
