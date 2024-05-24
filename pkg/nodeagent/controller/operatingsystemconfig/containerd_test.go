// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig_test

import (
	"context"
	"io/fs"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
)

var _ = Describe("ContainerdReconciler", func() {
	Describe("#ContainerdReconciler", func() {
		var (
			fakeFS afero.Afero

			reconciler *operatingsystemconfig.Reconciler

			criConfig *extensionsv1alpha1.CRIConfig

			logger = logr.Discard()
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
							Upstream: "docker.io",
							Server:   ptr.To("https://registry-1.docker.io"),
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

			Expect(reconciler.EnsureContainerdRegistries(context.Background(), logger, criConfig.Containerd.Registries)).To(Succeed())
			assertFileOnDisk(fakeFS, "/etc/containerd/certs.d/docker.io/hosts.toml", expected, 0644)
		})

		It("should ensure the containerd unit dropin configuration", func() {
			expected := `[Service]
Environment="PATH=/var/bin/containerruntimes:/i/am/foo"
`

			te.Setenv("PATH", "/i/am/foo")
			Expect(reconciler.EnsureContainerdEnvironment()).To(Succeed())
			assertFileOnDisk(fakeFS, "/etc/systemd/system/containerd.service.d/30-env_config.conf", expected, 0644)
		})

		It("should ensure the containerd configuration", func() {
			values := &apiextensionsv1.JSON{
				Raw: []byte(`
				{
					"runtime_type": "io.containerd.runsc.v1",
					"bool":         true,
					"list":         ["a"]
				}
				`),
			}
			criConfig = &extensionsv1alpha1.CRIConfig{
				CgroupDriver: extensionsv1alpha1.CRICgroupDriverSystemd,
				Containerd: &extensionsv1alpha1.ContainerdConfig{
					SandboxImage: "pause",
					Plugins: []extensionsv1alpha1.PluginConfig{
						{
							Path:   []string{"io.containerd.grpc.v1.cri", "containerd", "runtimes", "gvisor"},
							Values: values,
						},
					},
				},
			}

			originalContent := `
disabled_plugins = []
imports = ["/an/existing/path/*.toml"]
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
	systemd_cgroup = false

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
imports = ["/an/existing/path/*.toml", "/etc/containerd/conf.d/*.toml"]
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
    sandbox_image = "pause"
    systemd_cgroup = false

    [plugins."io.containerd.grpc.v1.cri".cni]
      bin_dir = "/opt/cni/bin"

    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"

      [plugins."io.containerd.grpc.v1.cri".containerd.default_runtime]
        base_runtime_spec = ""

        [plugins."io.containerd.grpc.v1.cri".containerd.default_runtime.options]

      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]

        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.gvisor]
          bool = true
          list = ["a"]
          runtime_type = "io.containerd.runsc.v1"

        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          base_runtime_spec = ""

          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
            SystemdCgroup = true

    [plugins."io.containerd.grpc.v1.cri".registry]
      config_path = "/etc/containerd/certs.d"
`

			Expect(fakeFS.WriteFile(extensionsv1alpha1.ContainerDConfigFile, []byte(originalContent), 0644)).To(Succeed())
			Expect(reconciler.EnsureContainerdConfiguration(criConfig)).To(Succeed())
			assertFileOnDisk(fakeFS, extensionsv1alpha1.ContainerDConfigFile, expected, 0644)
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
