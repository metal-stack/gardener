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

package controller

import (
	"fmt"
	"os"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/nodeagent/controller/common"
	"github.com/gardener/gardener/pkg/nodeagent/controller/kubeletupgrade"
	"github.com/gardener/gardener/pkg/nodeagent/controller/node"
	"github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/nodeagent/controller/selfupgrade"
	"github.com/gardener/gardener/pkg/nodeagent/controller/token"
)

// AddToManager adds all gardener-node-agent controllers to the given manager.
func AddToManager(mgr manager.Manager) error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	nodeName := strings.TrimSpace(strings.ToLower(hostname))

	config, err := common.ReadNodeAgentConfiguration()
	if err != nil {
		return err
	}

	selfUpgradeChannel := make(chan event.GenericEvent, 1)
	kubeletUpradeChannel := make(chan event.GenericEvent, 1)

	if err := (&node.Reconciler{
		NodeName:   nodeName,
		SyncPeriod: 10 * time.Minute,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding node controller: %w", err)
	}

	if err := (&operatingsystemconfig.Reconciler{
		NodeName:   nodeName,
		Config:     config,
		SyncPeriod: 10 * time.Minute,
		TriggerChannels: []chan event.GenericEvent{
			selfUpgradeChannel,
			kubeletUpradeChannel,
		},
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding operatingsystemconfig controller: %w", err)
	}

	if err := (&kubeletupgrade.Reconciler{
		Config:           config,
		TargetBinaryPath: "/opt/bin/kubelet",
		SyncPeriod:       10 * time.Minute,
		TriggerChannel:   kubeletUpradeChannel,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding kubelet upgrade controller: %w", err)
	}

	if err := (&selfupgrade.Reconciler{
		NodeName:       nodeName,
		Config:         config,
		SelfBinaryPath: os.Args[0],
		SyncPeriod:     10 * time.Minute,
		TriggerChannel: selfUpgradeChannel,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding self upgrade controller: %w", err)
	}

	if err := (&token.Reconciler{
		Config:     config,
		SyncPeriod: 10 * time.Minute,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding token controller: %w", err)
	}

	return nil
}
