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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"

	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/executor"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	"github.com/gardener/gardener/pkg/utils"

	"github.com/gardener/gardener/pkg/nodeagent/controller/common"

	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

// Reconciler fetches the shoot access token for gardener-node-agent and writes the token to disk.
type Reconciler struct {
	Client   client.Client
	Recorder record.EventRecorder

	NodeName string

	Config *nodeagentv1alpha1.NodeAgentConfiguration

	TriggerChannels []chan event.GenericEvent

	SyncPeriod time.Duration
}

func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	log.Info("reconciling osc secret")

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, request.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	osc, oscRaw, err := r.extractOSCFromSecret(secret)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to extract osc from secret %w", err)
	}

	oscCheckSum := utils.ComputeSHA256Hex(oscRaw)

	err = yaml.Unmarshal(oscRaw, osc)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to unmarshal osc from secret data %w", err)
	}

	oscChanges, err := common.CalculateChangedUnitsAndRemovedFiles(osc)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to calculate osc changes from previous run %w", err)
	}

	node := &corev1.Node{}
	err = r.Client.Get(ctx, client.ObjectKey{Name: r.NodeName}, node)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, fmt.Errorf("unable to fetch node %w", err)
	}

	if node != nil && node.Annotations[executor.AnnotationKeyChecksum] == oscCheckSum {
		log.Info("node is up to date, osc did not change, returning")
		return reconcile.Result{}, nil
	}

	tmpDir, err := os.MkdirTemp("/tmp", "gardener-node-agent-*")
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to create temp dir %w", err)
	}

	for _, f := range osc.Spec.Files {
		if f.Content.Inline == nil {
			continue
		}

		err = os.MkdirAll(filepath.Dir(f.Path), fs.ModeDir)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to create directory %q %w", f.Path, err)
		}
		perm := fs.FileMode(0600)
		if f.Permissions != nil {
			perm = fs.FileMode(*f.Permissions)
		}

		data, err := helper.Decode(f.Content.Inline.Encoding, []byte(f.Content.Inline.Data))
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to decode file %q data %w", f.Path, err)
		}

		tmpFilePath := filepath.Join(tmpDir, filepath.Base(f.Path))
		err = os.WriteFile(tmpFilePath, data, perm)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to create file %q %w", f.Path, err)
		}

		err = os.Rename(tmpFilePath, f.Path)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to move temporary file to %q %w", f.Path, err)
		}
	}

	for _, u := range oscChanges.ChangedUnits {
		if u.Content == nil {
			continue
		}

		systemdUnitFilePath := path.Join("/etc/systemd/system", u.Name)
		existingUnitContent, err := os.ReadFile(systemdUnitFilePath)
		if err != nil && !os.IsNotExist(err) {
			return reconcile.Result{}, fmt.Errorf("unable to read systemd unit %q %w", u.Name, err)
		}

		newUnitContent := []byte(*u.Content)
		if bytes.Equal(newUnitContent, existingUnitContent) {
			continue
		}

		err = os.WriteFile(systemdUnitFilePath, newUnitContent, 0600)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to write unit %q %w", u.Name, err)
		}
		if u.Enable != nil && *u.Enable {
			err = dbus.Enable(ctx, u.Name)
			if err != nil {
				return reconcile.Result{}, fmt.Errorf("unable to enable unit %q %w", u.Name, err)
			}
		}
		if u.Enable != nil && !*u.Enable {
			err = dbus.Disable(ctx, u.Name)
			if err != nil {
				return reconcile.Result{}, fmt.Errorf("unable to disable unit %q %w", u.Name, err)
			}
		}
		log.Info("processed writing unit", "name", u.Name, "command", pointer.StringDeref(u.Command, ""))
	}
	for _, u := range oscChanges.DeletedUnits {
		err = dbus.Stop(ctx, r.Recorder, node, u.Name)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to stop deleted unit %q %w", u.Name, err)
		}

		err = dbus.Disable(ctx, u.Name)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to disable deleted unit %q %w", u.Name, err)
		}

		err = os.Remove(path.Join("/etc/systemd/system", u.Name))
		if err != nil && !os.IsNotExist(err) {
			return reconcile.Result{}, fmt.Errorf("unable to delete systemd unit of deleted %q %w", u.Name, err)
		}
	}

	err = dbus.DaemonReload(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	g, groupCtx := errgroup.WithContext(timeoutCtx)

	for _, u := range oscChanges.ChangedUnits {
		if u.Content == nil {
			continue
		}
		u := u

		// There are some units without a command specified,
		// the old bash based implementation didn't care about command and always restarted all units
		// mimic this this behavior at least for these units.
		if u.Command == nil {
			u.Command = pointer.String("start")
		}

		g.Go(func() error {
			switch *u.Command {
			// FIXME make this accessible constants
			case "start", "restart":
				err = dbus.Restart(groupCtx, r.Recorder, node, u.Name)
				if err != nil {
					return fmt.Errorf("unable to restart %q: %w", u.Name, err)
				}
			case "stop":
				err = dbus.Stop(groupCtx, r.Recorder, node, u.Name)
				if err != nil {
					return fmt.Errorf("unable to stop %q: %w", u.Name, err)
				}
			}

			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		log.Error(err, "error ensuring states of systemd units")
	}

	var deletionErrors []error
	for _, f := range oscChanges.DeletedFiles {
		err := os.Remove(f.Path)
		if err != nil && !os.IsNotExist(err) {
			deletionErrors = append(deletionErrors, err)
		}
	}
	if len(deletionErrors) > 0 {
		return reconcile.Result{}, fmt.Errorf("unable to delete all files which must not exist anymore: %w", errors.Join(deletionErrors...))
	}

	// Persist current OSC for comparison with next one
	err = os.WriteFile(nodeagentv1alpha1.NodeAgentOSCOldConfigPath, oscRaw, 0644)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to write previous osc to file %w", err)
	}

	log.Info("Successfully processed operating system configs", "files", len(osc.Spec.Files), "units", len(osc.Spec.Units))

	// notifying other controllers about possible change in applied files (e.g. configuration.yaml)
	for _, c := range r.TriggerChannels {
		c <- event.GenericEvent{}
	}

	r.Recorder.Event(node, corev1.EventTypeNormal, "OSCApplied", "all osc files and units have been applied successfully")

	if node == nil || node.Name == "" || node.Annotations == nil {
		return reconcile.Result{
			RequeueAfter: 10 * time.Second,
		}, fmt.Errorf("still waiting for node to get registered")
	}

	node.Annotations[v1beta1constants.LabelWorkerKubernetesVersion] = r.Config.KubernetesVersion
	node.Annotations[executor.AnnotationKeyChecksum] = oscCheckSum

	err = r.Client.Update(ctx, node, &client.UpdateOptions{})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to set node annotation %w", err)
	}

	log.V(1).Info("Requeuing", "requeueAfter", r.SyncPeriod)
	return reconcile.Result{RequeueAfter: r.SyncPeriod}, nil
}

func (r *Reconciler) extractOSCFromSecret(secret *corev1.Secret) (*v1alpha1.OperatingSystemConfig, []byte, error) {
	oscRaw, ok := secret.Data[nodeagentv1alpha1.NodeAgentOSCSecretKey]
	if !ok {
		return nil, nil, fmt.Errorf("no token found in secret")
	}

	osc := &v1alpha1.OperatingSystemConfig{}
	err := yaml.Unmarshal(oscRaw, osc)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to unmarshal osc from secret data %w", err)
	}
	return osc, oscRaw, nil
}
