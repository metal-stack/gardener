// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"slices"
	"time"

	"github.com/go-logr/logr"
	"github.com/pelletier/go-toml"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig/mappatch"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// ReconcileContainerdConfig sets required values of the given containerd configuration.
func (r *Reconciler) ReconcileContainerdConfig(ctx context.Context, criConfig *extensionsv1alpha1.CRIConfig) error {
	err := r.ensureContainerdConfigDirectories()
	if err != nil {
		return err
	}

	err = r.ensureContainerdDefaultConfig(ctx)
	if err != nil {
		return err
	}

	err = r.EnsureContainerdEnvironment()
	if err != nil {
		return err
	}

	err = r.EnsureContainerdConfiguration(criConfig)
	if err != nil {
		return err
	}

	return nil
}

const containerdDropinDir = "/etc/systemd/system/containerd.service.d"

func (r *Reconciler) ensureContainerdConfigDirectories() error {
	for _, dir := range []string{
		extensionsv1alpha1.ContainerDBaseDir,
		extensionsv1alpha1.ContainerDConfigDir,
		extensionsv1alpha1.ContainerDCertsDir,
		extensionsv1alpha1.ContainerDRuntimeContainersBinFolder,
		containerdDropinDir,
	} {
		err := r.FS.MkdirAll(dir, defaultDirPermissions)
		if err != nil {
			return fmt.Errorf("unable to ensure containerd config directory %q: %w", dir, err)
		}
	}

	return nil
}

func (r *Reconciler) ensureContainerdDefaultConfig(ctx context.Context) error {
	exists, err := r.fileExists(extensionsv1alpha1.ContainerDConfigFile)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	output, err := exec.CommandContext(ctx, "containerd", "config", "default").Output()
	if err != nil {
		return fmt.Errorf("error creating containerd default config: %w", err)
	}

	return r.FS.WriteFile(extensionsv1alpha1.ContainerDConfigFile, output, 0644)
}

// EnsureContainerdEnvironment ensures the containerd environment
func (r *Reconciler) EnsureContainerdEnvironment() error {
	var (
		containerdEnvFileName = "30-env_config.conf"
		unitDropin            = `[Service]
Environment="PATH=` + extensionsv1alpha1.ContainerDRuntimeContainersBinFolder + `:` + os.Getenv("PATH") + `"
`
	)

	containerdEnvFilePath := path.Join(containerdDropinDir, containerdEnvFileName)
	exists, err := r.fileExists(containerdEnvFilePath)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	err = r.FS.WriteFile(containerdEnvFilePath, []byte(unitDropin), 0644)
	if err != nil {
		return fmt.Errorf("unable to write unit dropin: %w", err)
	}

	return nil
}

// EnsureContainerdConfiguration sets the configuration for the containerd.
func (r *Reconciler) EnsureContainerdConfiguration(criConfig *extensionsv1alpha1.CRIConfig) error {
	config, err := r.FS.ReadFile(extensionsv1alpha1.ContainerDConfigFile)
	if err != nil {
		return fmt.Errorf("unable to read containerd config.toml: %w", err)
	}

	content := map[string]any{}

	err = toml.Unmarshal(config, &content)
	if err != nil {
		return fmt.Errorf("unable to decode containerd default config: %w", err)
	}

	type (
		patch struct {
			name  string
			path  mappatch.MapPath
			setFn mappatch.SetFn
		}

		patches []patch
	)

	ps := patches{
		{
			name: "cgroup driver",
			path: mappatch.MapPath{"plugins", "io.containerd.grpc.v1.cri", "containerd", "runtimes", "runc", "options", "SystemdCgroup"},
			setFn: func(value any) (any, error) {
				if criConfig == nil {
					return value, nil
				}

				return criConfig.CgroupDriver == extensionsv1alpha1.CRICgroupDriverSystemd, nil
			},
		},
		{
			name: "registry config path",
			path: mappatch.MapPath{"plugins", "io.containerd.grpc.v1.cri", "registry", "config_path"},
			setFn: func(_ any) (any, error) {
				return extensionsv1alpha1.ContainerDCertsDir, nil
			},
		},
		{
			name: "imports paths",
			path: mappatch.MapPath{"imports"},
			setFn: func(value any) (any, error) {
				importPath := path.Join(extensionsv1alpha1.ContainerDConfigDir, "*.toml")

				imports, ok := value.([]any)
				if !ok {
					return []string{importPath}, nil
				}

				for _, imp := range imports {
					path, ok := imp.(string)
					if !ok {
						continue
					}

					if path == importPath {
						return value, nil
					}
				}

				return append(imports, importPath), nil
			},
		},
		{
			name: "sandbox image",
			path: mappatch.MapPath{"plugins", "io.containerd.grpc.v1.cri", "sandbox_image"},
			setFn: func(value any) (any, error) {
				if criConfig == nil || criConfig.Containerd == nil {
					return value, nil
				}

				return criConfig.Containerd.SandboxImage, nil
			},
		},
	}

	if criConfig.Containerd != nil {
		for _, pluginConfig := range criConfig.Containerd.Plugins {
			ps = append(ps, patch{
				name: "plugin configuration",
				path: append(mappatch.MapPath{"plugins"}, pluginConfig.Path...),
				setFn: func(any) (any, error) {
					values := map[string]any{}

					err := json.Unmarshal(pluginConfig.Values.Raw, &values)
					if err != nil {
						return nil, err
					}

					return values, nil
				},
			})
		}
	}

	for _, p := range ps {
		content, err = mappatch.SetMapEntry(content, p.path, p.setFn)
		if err != nil {
			return fmt.Errorf("unable setting %s in containerd config.toml: %w", p.name, err)
		}
	}

	f, err := r.FS.OpenFile(extensionsv1alpha1.ContainerDConfigFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("unable to open containerd config.toml: %w", err)
	}
	defer func() {
		err = f.Close()
	}()

	err = toml.NewEncoder(f).Encode(content)
	if err != nil {
		return fmt.Errorf("unable to encode hosts.toml: %w", err)
	}

	return err
}

// EnsureContainerdRegistries configures containerd to use the desired image registries.
func (r *Reconciler) EnsureContainerdRegistries(ctx context.Context, log logr.Logger, newRegistries []extensionsv1alpha1.RegistryConfig) error {
	var (
		fns        = make([]flow.TaskFn, 0, len(newRegistries))
		httpClient = http.Client{Timeout: 1 * time.Second}
	)

	for _, registryConfig := range newRegistries {
		fns = append(fns, func(ctx context.Context) error {
			baseDir := path.Join(extensionsv1alpha1.ContainerDCertsDir, registryConfig.Upstream)
			if err := r.FS.MkdirAll(baseDir, defaultDirPermissions); err != nil {
				return fmt.Errorf("unable to ensure registry config base directory: %w", err)
			}

			hostsTomlFilePath := path.Join(baseDir, "hosts.toml")
			exists, err := r.FS.Exists(hostsTomlFilePath)
			if err != nil {
				return fmt.Errorf("unable to check if registry config file exists: %w", err)
			}

			// Check if registry endpoints are reachable if the config is new.
			// This is especially required when registries run within the cluster and during bootstrap,
			// the Kubernetes deployments are not ready yet.
			if !exists && registryConfig.ProbeHosts {
				log.Info("Probing endpoints for image registry", "upstream", registryConfig.Upstream)
				if err := retry.Until(ctx, 2*time.Second, func(ctx context.Context) (done bool, err error) {
					for _, registryHosts := range registryConfig.Hosts {
						req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryHosts.URL, nil)
						if err != nil {
							return false, fmt.Errorf("failed to construct http request %s for upstream %s: %w", registryHosts.URL, registryConfig.Upstream, err)
						}

						_, err = httpClient.Do(req)
						if err != nil {
							return false, fmt.Errorf("failed to reach registry %s for upstream %s: %w", registryHosts.URL, registryConfig.Upstream, err)
						}
					}
					return true, nil
				}); err != nil {
					return err
				}

				log.Info("Probing endpoints for image registry succeeded", "upstream", registryConfig.Upstream)
			}

			f, err := r.FS.OpenFile(hostsTomlFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return fmt.Errorf("unable to open hosts.toml: %w", err)
			}

			defer func() {
				err = f.Close()
			}()

			type (
				hostConfig struct {
					Capabilities []string `toml:"capabilities,omitempty"`
					CaCerts      []string `toml:"ca,omitempty"`
				}

				config struct {
					Server *string               `toml:"server,omitempty" comment:"managed by gardener-node-agent"`
					Host   map[string]hostConfig `toml:"host,omitempty"`
				}
			)

			content := config{
				Server: registryConfig.Server,
				Host:   map[string]hostConfig{},
			}

			for _, host := range registryConfig.Hosts {
				h := hostConfig{}

				if len(host.Capabilities) > 0 {
					h.Capabilities = host.Capabilities
				}
				if len(host.CACerts) > 0 {
					h.CaCerts = host.CACerts
				}

				content.Host[host.URL] = h
			}

			err = toml.NewEncoder(f).Encode(content)
			if err != nil {
				return fmt.Errorf("unable to encode hosts.toml: %w", err)
			}

			return err
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// CleanupUnusedContainerdRegistries removes unused containerd registries them from the file system.
func (r *Reconciler) CleanupUnusedContainerdRegistries(log logr.Logger, oldRegistries, newRegistries []extensionsv1alpha1.RegistryConfig) error {
	upstreamsInUse := sets.New[string]()
	for _, registryConfig := range newRegistries {
		upstreamsInUse.Insert(registryConfig.Upstream)
	}

	registriesToRemove := slices.DeleteFunc(oldRegistries, func(config extensionsv1alpha1.RegistryConfig) bool {
		return upstreamsInUse.Has(config.Upstream)
	})

	for _, registryConfig := range registriesToRemove {
		log.Info("Removing obsolete registry directory", "upstream", registryConfig.Upstream)
		if err := r.FS.RemoveAll(path.Join(extensionsv1alpha1.ContainerDCertsDir, registryConfig.Upstream)); err != nil {
			return fmt.Errorf("failed to cleanup obsolete registry directory: %w", err)
		}
	}

	return nil
}

func (r *Reconciler) finalizeContainerdHandling(
	ctx context.Context,
	log logr.Logger,
	oldCRIConfig *extensionsv1alpha1.CRIConfig,
	newCRIConfig *extensionsv1alpha1.CRIConfig,
	node *corev1.Node,
	mustRestartConainerd bool,
) error {
	containerdConfig := newCRIConfig.Containerd
	if containerdConfig == nil {
		containerdConfig = &extensionsv1alpha1.ContainerdConfig{}
	}

	var oldRegistries []extensionsv1alpha1.RegistryConfig
	if oldCRIConfig != nil && oldCRIConfig.Containerd != nil {
		oldRegistries = oldCRIConfig.Containerd.Registries
	}

	if err := r.EnsureContainerdRegistries(ctx, log, containerdConfig.Registries); err != nil {
		return err
	}
	if err := r.CleanupUnusedContainerdRegistries(log, oldRegistries, containerdConfig.Registries); err != nil {
		return err
	}

	if mustRestartConainerd {
		if err := r.DBus.Restart(ctx, r.Recorder, node, v1beta1constants.OperatingSystemConfigUnitNameContainerDService); err != nil {
			return fmt.Errorf("unable to restart unit %q: %w", v1beta1constants.OperatingSystemConfigUnitNameContainerDService, err)
		}
		log.Info("Successfully restarted unit", "unitName", v1beta1constants.OperatingSystemConfigUnitNameContainerDService)
	}

	return nil
}
