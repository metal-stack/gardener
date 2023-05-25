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

package kubeletupgrade

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/nodeagent/controller/common"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	"github.com/gardener/gardener/pkg/nodeagent/registry"

	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

var (
	hyperkubeImageDownloadedPath = path.Join(nodeagentv1alpha1.NodeAgentBaseDir, "hyperkube-downloaded")
)

// Reconciler fetches the shoot access token for gardener-node-agent and writes the token to disk.
type Reconciler struct {
	Client   client.Client
	Recorder record.EventRecorder

	Config *nodeagentv1alpha1.NodeAgentConfiguration

	TargetBinaryPath string

	SyncPeriod time.Duration

	TriggerChannel <-chan event.GenericEvent

	NodeName string
}

func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	config, err := common.ReadNodeAgentConfiguration()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to update node agent config: %w", err)
	}

	r.Config = config

	imageRefDownloaded, err := common.ReadTrimmedFile(hyperkubeImageDownloadedPath)
	if err != nil && !os.IsNotExist(err) {
		return reconcile.Result{}, err
	} else {
		if r.Config.HyperkubeImage == imageRefDownloaded {
			log.Info("Desired kubelet binary hasn't changed, checking again later", "requeueAfter", r.SyncPeriod)
			return reconcile.Result{RequeueAfter: r.SyncPeriod}, nil
		}
	}

	log.Info("kubelet binary has changed, starting kubelet update", "imageRef", r.Config.HyperkubeImage)

	err = registry.ExtractFromLayer(r.Config.HyperkubeImage, "kubelet", r.TargetBinaryPath)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to extract binary from image: %w", err)
	}

	log.Info("Successfully downloaded new kubelet binary", "imageRef", r.Config.HyperkubeImage)

	if err := os.MkdirAll(nodeagentv1alpha1.NodeAgentBaseDir, fs.ModeDir); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed creating node agent directory: %w", err)
	}

	// Save most recently downloaded image ref
	if err := os.WriteFile(hyperkubeImageDownloadedPath, []byte(r.Config.HyperkubeImage), 0600); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed writing downloaded image ref: %w", err)
	}

	node := &corev1.Node{}
	err = r.Client.Get(ctx, client.ObjectKey{Name: r.NodeName}, node)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, fmt.Errorf("unable to fetch node %w", err)
	}

	log.Info("Restarting kubelet unit")
	if err = dbus.Restart(ctx, r.Recorder, node, "kubelet.service"); err != nil { // TODO: do not use hardcoded unit name
		return reconcile.Result{}, fmt.Errorf("unable restart service: %w", err)
	}

	log.V(1).Info("Requeuing", "requeueAfter", r.SyncPeriod)
	return reconcile.Result{RequeueAfter: r.SyncPeriod}, nil
}
