// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"strconv"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

const (
	sysFsHugePage2MPath = "/sys/kernel/mm/hugepages/hugepages-2048kB"
	sysFsHugePage1GPath = "/sys/kernel/mm/hugepages/hugepages-1048576kB"
	nrHugePagesFile     = "nr_hugepages"
)

// ReconcileHugePageConfiguration sets desired hugepage number of hugepages
func (r *Reconciler) ReconcileHugePageConfiguration(hugepageConfig *extensionsv1alpha1.HugePageConfig) error {
	if hugepageConfig == nil {
		return nil
	}

	if hugepageConfig.HugePages1G != nil {
		err := r.ensureHugePageSetting(*hugepageConfig.HugePages1G, sysFsHugePage1GPath)
		if err != nil {
			return err
		}
	}

	if hugepageConfig.HugePages2M != nil {
		err := r.ensureHugePageSetting(*hugepageConfig.HugePages2M, sysFsHugePage2MPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) ensureHugePageSetting(hugePageNr int, hugePagePath string) error {
	hugePagePathExists, err := r.FS.DirExists(hugePagePath)
	if err != nil {
		return err
	}
	if !hugePagePathExists {
		return fmt.Errorf("system does not support %s", path.Base(hugePagePath))
	}

	hugePageFile := path.Join(hugePagePath, nrHugePagesFile)
	err = r.FS.WriteFile(hugePageFile, []byte(strconv.Itoa(hugePageNr)), 0644)
	if err != nil {
		return err
	}

	h, err := r.FS.ReadFile(hugePageFile)
	if err != nil {
		return err
	}

	actualHugePageNr, err := strconv.Atoi(string(h))
	if err != nil {
		return err
	}

	if actualHugePageNr < hugePageNr {
		return fmt.Errorf("could not request %d hugepages, only got %d", hugePageNr, actualHugePageNr)
	}

	return nil
}

// RestartKubeletUponHugepageChanges restarts the kubelet in case of a changed HugepageConfig
func (r *Reconciler) RestartKubeletUponHugepageChanges(ctx context.Context, log logr.Logger, node *corev1.Node, oldHugepageConfig, newHugepageConfig *extensionsv1alpha1.HugePageConfig) error {
	mustRestartKubelet := !reflect.DeepEqual(oldHugepageConfig, newHugepageConfig)

	if mustRestartKubelet {
		log.Info("Restarting kubelet upon modified hugepage configuration")
		if err := r.DBus.Restart(ctx, r.Recorder, node, v1beta1constants.OperatingSystemConfigUnitNameKubeletService); err != nil {
			return fmt.Errorf("unable to restart unit %q: %w", v1beta1constants.OperatingSystemConfigUnitNameKubeletService, err)
		}
		log.Info("Successfully restarted unit", "unitName", v1beta1constants.OperatingSystemConfigUnitNameKubeletService)
	}
	return nil
}
