// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig_test

import (
	"fmt"
	"io/fs"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
)

var _ = Describe("ContainerdReconciler", func() {
	Describe("#ContainerdReconciler", func() {
		var (
			fakeFS afero.Afero

			reconciler *operatingsystemconfig.Reconciler

			criConfig *extensionsv1alpha1.CRIConfig
		)

		BeforeEach(func() {
			fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}

			reconciler = &operatingsystemconfig.Reconciler{
				FS: fakeFS,
			}

			criConfig = &extensionsv1alpha1.CRIConfig{}
		})

		It("should ensure the containerd registry configuration", func() {
			criConfig = &extensionsv1alpha1.CRIConfig{
				Containerd: &extensionsv1alpha1.ContainerdConfig{
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
				},
			}

			expected := `
# managed by gardener-node-agent
server = "https://registry-1.docker.io"

[host]

  [host."https://docker-mirror.internal"]
    ca = ["docker-mirror.crt"]
    capabilities = ["pull", "resolve"]

  [host."https://public-mirror.example.com"]
    capabilities = ["pull"]
`

			Expect(reconciler.EnsureContainerdRegistries(criConfig.Containerd.Registries)).To(Succeed())
			assertFileOnDisk(fakeFS, "/etc/containerd/certs.d/registry-1.docker.io/hosts.toml", expected, 0644)
		})

		It("should ensure the containerd cgroup configuration", func() {
			criConfig = &extensionsv1alpha1.CRIConfig{
				CRICgroupDriver: extensionsv1alpha1.CRICgroupDriverSystemd,
			}

			sampleContent := `
disabled_plugins = []
imports = []
required_plugins = []
root = "/var/lib/containerd"
state = "/run/containerd"
temp = ""
version = 2

[cgroup]
  path = ""

[plugins]

  [plugins."io.containerd.gc.v1.scheduler"]
    deletion_threshold = 0
    mutation_threshold = 100
    pause_threshold = 0.02
    schedule_delay = "0s"
    startup_delay = "100ms"

  [plugins."io.containerd.grpc.v1.cri"]
    device_ownership_from_security_context = false

    [plugins."io.containerd.grpc.v1.cri".cni]
      bin_dir = "/opt/cni/bin"

    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"

      [plugins."io.containerd.grpc.v1.cri".containerd.default_runtime]
        base_runtime_spec = ""

        [plugins."io.containerd.grpc.v1.cri".containerd.default_runtime.options]

      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]

        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          base_runtime_spec = ""

          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
            SystemdCgroup = false
`
			expected := `disabled_plugins = []
imports = []
required_plugins = []
root = "/var/lib/containerd"
state = "/run/containerd"
temp = ""
version = 2

[cgroup]
  path = ""

[plugins]

  [plugins."io.containerd.gc.v1.scheduler"]
    deletion_threshold = 0
    mutation_threshold = 100
    pause_threshold = 0.02
    schedule_delay = "0s"
    startup_delay = "100ms"

  [plugins."io.containerd.grpc.v1.cri"]
    device_ownership_from_security_context = false

    [plugins."io.containerd.grpc.v1.cri".cni]
      bin_dir = "/opt/cni/bin"

    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"

      [plugins."io.containerd.grpc.v1.cri".containerd.default_runtime]
        base_runtime_spec = ""

        [plugins."io.containerd.grpc.v1.cri".containerd.default_runtime.options]

      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]

        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          base_runtime_spec = ""

          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
            SystemdCgroup = true
`

			Expect(fakeFS.WriteFile(extensionsv1alpha1.ContainerDConfigFile, []byte(sampleContent), 0644)).To(Succeed())
			Expect(reconciler.EnsureContainerdCgroupDriver(criConfig)).To(Succeed())
			assertFileOnDisk(fakeFS, extensionsv1alpha1.ContainerDConfigFile, expected, 0644)
		})
	})

	Describe("Traverse", func() {
		It("should set the value into a existing map", func() {
			var (
				m = map[string]any{
					"a": map[string]any{
						"b": map[string]any{
							"c": 1,
						},
					},
				}
				want = map[string]any{
					"a": map[string]any{
						"b": map[string]any{
							"c": 2,
						},
					},
				}
			)

			got, err := operatingsystemconfig.Traverse(m, 2, "a", "b", "c")
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(want))
		})

		It("should populate a nil map", func() {
			var (
				m    map[string]any
				want = map[string]any{
					"a": map[string]any{
						"b": map[string]any{
							"c": true,
						},
					},
				}
			)

			got, err := operatingsystemconfig.Traverse(m, true, "a", "b", "c")
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(want))
		})
	})
})

func assertFileOnDisk(fakeFS afero.Afero, path, expectedContent string, fileMode uint32) {
	description := "file path " + path

	content, err := fakeFS.ReadFile(path)
	fmt.Println(string(content))
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), description)
	ExpectWithOffset(1, string(content)).To(Equal(expectedContent), description)

	fileInfo, err := fakeFS.Stat(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), description)
	ExpectWithOffset(1, fileInfo.Mode()).To(Equal(fs.FileMode(fileMode)), description)
}
