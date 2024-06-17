// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig_test

import (
	"context"
	"path"
	"strconv"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
	fakedbus "github.com/gardener/gardener/pkg/nodeagent/dbus/fake"
)

const (
	hugePageCount      int = 1337
	hugePageFilePath2M     = "/sys/kernel/mm/hugepages/hugepages-2048kB/nr_hugepages"
	hugePageFilePath1G     = "/sys/kernel/mm/hugepages/hugepages-1048576kB/nr_hugepages"
)

var _ = Describe("HugePageConfigurationReconciler", func() {
	Describe("#ContainerdReconciler", func() {
		var (
			fakeFS         afero.Afero
			fakeDbus       *fakedbus.DBus
			reconciler     *operatingsystemconfig.Reconciler
			hugePageConfig *extensionsv1alpha1.HugePageConfig
			ctx            context.Context
			log            logr.Logger
		)

		BeforeEach(func() {
			ctx = context.Background()
			log = logr.Discard()

			fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}
			err := fakeFS.MkdirAll(path.Dir(hugePageFilePath2M), 0755)
			Expect(err).ToNot(HaveOccurred())
			err = fakeFS.MkdirAll(path.Dir(hugePageFilePath1G), 0755)
			Expect(err).ToNot(HaveOccurred())

			fakeDbus = fakedbus.New()

			reconciler = &operatingsystemconfig.Reconciler{
				FS:   fakeFS,
				DBus: fakeDbus,
			}

			hugePageConfig = &extensionsv1alpha1.HugePageConfig{}
		})

		It("should set 2M hugepages", func() {
			hugePageConfig.HugePages2M = ptr.To(hugePageCount)

			Expect(reconciler.ReconcileHugePageConfiguration(hugePageConfig)).To(Succeed())

			exists2M, exists1G := hugePageFilesExist(fakeFS)
			Expect(exists2M).To(BeTrue())
			Expect(exists1G).To(BeFalse())

			Expect(readHugePages(fakeFS, hugePageFilePath2M)).To(Equal(hugePageCount))
		})

		It("should set 1G hugepages", func() {
			hugePageConfig.HugePages1G = ptr.To(hugePageCount)

			Expect(reconciler.ReconcileHugePageConfiguration(hugePageConfig)).To(Succeed())

			exists2M, exists1G := hugePageFilesExist(fakeFS)
			Expect(exists2M).To(BeFalse())
			Expect(exists1G).To(BeTrue())

			Expect(readHugePages(fakeFS, hugePageFilePath1G)).To(Equal(hugePageCount))
		})

		It("should set 2M and 1G hugepages", func() {
			hugePageConfig.HugePages2M = ptr.To(hugePageCount)
			hugePageConfig.HugePages1G = ptr.To(hugePageCount)

			Expect(reconciler.ReconcileHugePageConfiguration(hugePageConfig)).To(Succeed())

			exists2M, exists1G := hugePageFilesExist(fakeFS)
			Expect(exists2M).To(BeTrue())
			Expect(exists1G).To(BeTrue())

			Expect(readHugePages(fakeFS, hugePageFilePath2M)).To(Equal(hugePageCount))
			Expect(readHugePages(fakeFS, hugePageFilePath1G)).To(Equal(hugePageCount))
		})

		It("should restart the kubelet if the hugepageConfig changes", func() {
			oldHugepageConfig := hugePageConfig.DeepCopy()
			hugePageConfig.HugePages2M = ptr.To(hugePageCount)

			Expect(reconciler.RestartKubeletUponHugepageChanges(ctx, log, &corev1.Node{}, oldHugepageConfig, hugePageConfig)).To(Succeed())
			Expect(fakeDbus.Actions).To(ContainElement(fakedbus.SystemdAction{
				Action:    fakedbus.ActionRestart,
				UnitNames: []string{v1beta1constants.OperatingSystemConfigUnitNameKubeletService},
			}))
		})

		It("should not restart the kubelet if the hugepageConfig did not change", func() {
			oldHugepageConfig := hugePageConfig.DeepCopy()

			Expect(reconciler.RestartKubeletUponHugepageChanges(ctx, log, &corev1.Node{}, oldHugepageConfig, hugePageConfig)).To(Succeed())
			Expect(fakeDbus.Actions).To(BeEmpty())
		})
	})
})

func readHugePages(fs afero.Afero, hugePageFilePath string) int {
	c, err := fs.ReadFile(hugePageFilePath)
	Expect(err).NotTo(HaveOccurred())
	hugepageNumber, err := strconv.Atoi(string(c))
	Expect(err).NotTo(HaveOccurred())
	return hugepageNumber
}

func hugePageFilesExist(fs afero.Afero) (bool, bool) {
	hugePageFile2MExists, err := fs.Exists(hugePageFilePath2M)
	Expect(err).ToNot(HaveOccurred())
	hugePageFile1GExists, err := fs.Exists(hugePageFilePath1G)
	Expect(err).ToNot(HaveOccurred())
	return hugePageFile2MExists, hugePageFile1GExists
}
