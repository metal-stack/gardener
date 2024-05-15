package operatingsystemconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/go-logr/logr"
	"github.com/pelletier/go-toml"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// ReconcileContainerdConfig sets required values of the given containerd configuration.
func (r *Reconciler) ReconcileContainerdConfig(ctx context.Context, log logr.Logger, criConfig *extensionsv1alpha1.CRIConfig) error {
	if criConfig == nil {
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

	err = r.EnsureContainerdConfiguration(criConfig)
	if err != nil {
		return err
	}

	if criConfig.Containerd != nil {
		err = r.EnsureContainerdRegistries(criConfig.Containerd.Registries)
		if err != nil {
			return err
		}
	}

	// TODO: we probably need to check if something changed and decide whether we have to restart the containerd service

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
Environment="PATH=$BIN_PATH:$PATH"
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

	type patch struct {
		name    string
		path    []string
		patchFn func(any) (any, error)
	}
	type patches []patch

	ps := patches{
		{
			name: "cgroup driver",
			path: []string{"plugins", "io.containerd.grpc.v1.cri", "containerd", "runtimes", "runc", "options", "SystemdCgroup"},
			patchFn: func(value any) (any, error) {
				if criConfig == nil {
					return value, nil
				}

				return criConfig.CRICgroupDriver == extensionsv1alpha1.CRICgroupDriverSystemd, nil
			},
		},
		{
			name: "registry config path",
			path: []string{"plugins", "io.containerd.grpc.v1.cri", "registry", "config_path"},
			patchFn: func(_ any) (any, error) {
				return extensionsv1alpha1.ContainerDCertsDir, nil
			},
		},
		{
			name: "imports paths",
			path: []string{"imports"},
			patchFn: func(value any) (any, error) {
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
			path: []string{"plugins", "io.containerd.grpc.v1.cri", "sandbox_image"},
			patchFn: func(value any) (any, error) {
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
				path: append([]string{"plugins"}, pluginConfig.Path...),
				patchFn: func(any) (any, error) {
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
		content, err = Traverse(content, p.patchFn, p.path...)
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
func (r *Reconciler) EnsureContainerdRegistries(registries []extensionsv1alpha1.RegistryConfig) error {
	for _, registryConfig := range registries {
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
	}

	return nil
}

func Traverse(m map[string]any, patchFn func(value any) (any, error), keys ...string) (map[string]any, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("at least one key for patching is required")
	}
	if patchFn == nil {
		return nil, fmt.Errorf("patchFn must not be nil")
	}

	if m == nil {
		m = map[string]any{}
	}

	var (
		key = keys[0]
		err error
	)

	if len(keys) == 1 {
		value := m[key]
		m[key], err = patchFn(value)
		if err != nil {
			return nil, fmt.Errorf("error patching value: %w", err)
		}

		return m, nil
	}

	entry, ok := m[key]
	if !ok {
		entry = map[string]any{}
	}

	childMap, ok := entry.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unable to traverse into data structure because existing value is not a map at %q", key)
	}

	m[key], err = Traverse(childMap, patchFn, keys[1:]...)
	if err != nil {
		return nil, err
	}

	return m, nil
}
