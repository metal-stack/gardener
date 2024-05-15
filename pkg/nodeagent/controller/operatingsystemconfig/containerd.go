package operatingsystemconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"slices"

	"github.com/go-logr/logr"
	"github.com/pelletier/go-toml"
	"k8s.io/apimachinery/pkg/util/sets"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig/mappatch"
)

// ReconcileContainerdConfig sets required values of the given containerd configuration.
func (r *Reconciler) ReconcileContainerdConfig(ctx context.Context, log logr.Logger, oldCRIConfig, newCRIConfig *extensionsv1alpha1.CRIConfig) error {
	if newCRIConfig == nil {
		return nil
	}

	log.Info("Applying containerd configuration")

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

	err = r.EnsureContainerdConfiguration(newCRIConfig)
	if err != nil {
		return err
	}

	if newCRIConfig.Containerd != nil {
		var oldRegistries []extensionsv1alpha1.RegistryConfig
		if oldCRIConfig.Containerd != nil {
			oldRegistries = oldCRIConfig.Containerd.Registries
		}

		err = r.EnsureContainerdRegistries(oldRegistries, newCRIConfig.Containerd.Registries)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) ensureContainerdConfigDirectories() error {
	for _, dir := range []string{
		extensionsv1alpha1.ContainerDBaseDir,
		extensionsv1alpha1.ContainerDConfigDir,
		extensionsv1alpha1.ContainerDCertsDir,
		extensionsv1alpha1.ContainerDRuntimeContainersBinFolder,
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

func (r *Reconciler) EnsureContainerdEnvironment() error {
	const (
		containerdUnitDropin = "/etc/systemd/system/containerd.service.d/30-env_config.conf"
		unitDropin           = `[Service]
Environment="PATH=` + extensionsv1alpha1.ContainerDRuntimeContainersBinFolder + `:$PATH"
`
	)

	exists, err := r.fileExists(containerdUnitDropin)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	err = r.FS.WriteFile(containerdUnitDropin, []byte(unitDropin), 0644)
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

				return criConfig.CRICgroupDriver == extensionsv1alpha1.CRICgroupDriverSystemd, nil
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
func (r *Reconciler) EnsureContainerdRegistries(oldRegistries, newRegistries []extensionsv1alpha1.RegistryConfig) error {
	upstreamsInUse := sets.New[string]()

	for _, registryConfig := range newRegistries {
		baseDir := path.Join(extensionsv1alpha1.ContainerDCertsDir, registryConfig.Upstream)
		if err := r.FS.MkdirAll(baseDir, defaultDirPermissions); err != nil {
			return fmt.Errorf("unable to ensure registry config base directory: %w", err)
		}

		f, err := r.FS.OpenFile(path.Join(baseDir, "hosts.toml"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("unable to open hosts.toml: %w", err)
		}

		err = func() error {
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
		}()
		if err != nil {
			return err
		}

		upstreamsInUse.Insert(registryConfig.Upstream)
	}

	registriesToRemove := slices.DeleteFunc(oldRegistries, func(config extensionsv1alpha1.RegistryConfig) bool {
		return upstreamsInUse.Has(config.Upstream)
	})

	for _, registryConfig := range registriesToRemove {
		if err := r.FS.RemoveAll(path.Join(extensionsv1alpha1.ContainerDCertsDir, registryConfig.Upstream)); err != nil {
			return fmt.Errorf("failed to cleanup obsolete registry directory: %w", err)
		}
	}

	return nil
}
