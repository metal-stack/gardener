// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig_test

import (
	"io/fs"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
)

var _ = Describe("Reconciler", func() {
	Describe("#Reconciler", func() {
		var (
			log = logr.Discard()

			fakeFS afero.Afero

			reconciler *operatingsystemconfig.Reconciler

			containerdConfig *extensionsv1alpha1.ContainerdConfig
		)

		BeforeEach(func() {
			fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}

			reconciler = &operatingsystemconfig.Reconciler{
				FS: fakeFS,
			}

			containerdConfig = &extensionsv1alpha1.ContainerdConfig{}
		})

		It("should write the containerd configuration", func() {
			containerdConfig = &extensionsv1alpha1.ContainerdConfig{
				Registries: []extensionsv1alpha1.RegistryConfig{
					{
						Server: "https://registry-1.docker.io",
						Hosts: []extensionsv1alpha1.RegistryHost{
							{
								URL:          "https://public-mirror.example.com",
								Capabilities: []string{"pull"},
							},
							{
								URL:          "https://docker-mirror.internal",
								Capabilities: []string{"pull", "resolve"},
								CACerts:      []string{"docker-mirror.crt"},
							},
						},
					},
				},
			}

			expected := `server = "https://registry-1.docker.io"

["host.\"https://docker-mirror.internal\""]
  capabilities = ["pull", "resolve"]
  ca = ["docker-mirror.crt"]

["host.\"https://public-mirror.example.com\""]
  capabilities = ["pull"]
`

			Expect(reconciler.ReconcileContainerdConfig(log, containerdConfig)).To(Succeed())
			assertFileOnDisk(fakeFS, "/etc/containerd/certs.d/registry-1.docker.io/hosts.toml", expected, 0644)
		})
	})
})

func assertFileOnDisk(fakeFS afero.Afero, path, expectedContent string, fileMode uint32) {
	description := "file path " + path

	content, err := fakeFS.ReadFile(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), description)
	ExpectWithOffset(1, string(content)).To(Equal(expectedContent), description)

	fileInfo, err := fakeFS.Stat(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), description)
	ExpectWithOffset(1, fileInfo.Mode()).To(Equal(fs.FileMode(fileMode)), description)
}
