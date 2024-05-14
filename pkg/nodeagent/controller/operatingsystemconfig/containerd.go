package operatingsystemconfig

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	toml "github.com/pelletier/go-toml"
)

func (r *Reconciler) ReconcileContainerdConfig(ctx context.Context, log logr.Logger, criConfig *extensionsv1alpha1.CRIConfig) error {
	if criConfig == nil {
		return nil
	}

	log.Info("Applying containerd configuration")

	err := r.ensureContainerdDefaultConfig(ctx)
	if err != nil {
		return err
	}

	err = r.EnsureContainerdCgroupDriver(criConfig)
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

func (r *Reconciler) ensureContainerdDefaultConfig(ctx context.Context) error {
	err := r.FS.MkdirAll(extensionsv1alpha1.ContainerDConfigDir, defaultDirPermissions)
	if err != nil {
		return fmt.Errorf("unable to ensure containerd config directory: %w", err)
	}

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

func (r *Reconciler) EnsureContainerdCgroupDriver(criConfig *extensionsv1alpha1.CRIConfig) error {
	config, err := r.FS.ReadFile(extensionsv1alpha1.ContainerDConfigFile)
	if err != nil {
		return fmt.Errorf("unable to read containerd config.toml: %w", err)
	}

	content := map[string]any{}

	err = toml.Unmarshal(config, &content)
	if err != nil {
		return fmt.Errorf("unable to decode containerd default config: %w", err)
	}

	systemdDriverEnabled := false
	if criConfig.CRICgroupDriver == extensionsv1alpha1.CRICgroupDriverSystemd {
		systemdDriverEnabled = true
	}

	patched, err := Traverse(content, systemdDriverEnabled, "plugins", "io.containerd.grpc.v1.cri", "containerd", "runtimes", "runc", "options", "SystemdCgroup")
	if err != nil {
		return fmt.Errorf("unable patching toml content: %w", err)
	}

	f, err := r.FS.OpenFile(extensionsv1alpha1.ContainerDConfigFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("unable to open containerd config.toml: %w", err)
	}
	defer func() {
		err = f.Close()
	}()

	err = toml.NewEncoder(f).Encode(patched)
	if err != nil {
		return fmt.Errorf("unable to encode hosts.toml: %w", err)
	}

	return err
}

func (r *Reconciler) EnsureContainerdRegistries(registries []extensionsv1alpha1.RegistryConfig) error {
	for _, registryConfig := range registries {
		u, err := url.Parse(registryConfig.Server)
		if err != nil {
			return fmt.Errorf("unable to parse registry server url: %w", err)
		}

		baseDir := path.Join(extensionsv1alpha1.ContainerDCertsDir, u.Host)
		err = r.FS.MkdirAll(baseDir, defaultDirPermissions)
		if err != nil {
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
					Server string                `toml:"server" comment:"managed by gardener-node-agent"`
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

func Traverse(m map[string]any, value any, keys ...string) (map[string]any, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("at least one key for patching is required")
	}

	if m == nil {
		m = map[string]any{}
	}

	key := keys[0]

	if len(keys) == 1 {
		m[key] = value
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

	var err error
	m[key], err = Traverse(childMap, value, keys[1:]...)
	if err != nil {
		return nil, err
	}

	return m, err
}
