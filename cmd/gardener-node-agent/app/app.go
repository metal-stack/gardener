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

package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/component-base/version/verflag"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/cmd/utils"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/controllerutils/routes"
	"github.com/gardener/gardener/pkg/features"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/nodeagent/apis/config"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/bootstrap"
	"github.com/gardener/gardener/pkg/nodeagent/controller"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Name is a const for the name of this component.
const Name = "gardener-node-agent"

// NewCommand creates a new cobra.Command for running gardener-node-agent.
func NewCommand() *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   Name,
		Short: "Launch the " + Name,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			log, err := utils.InitRun(cmd, opts, Name)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithCancel(cmd.Context())
			return run(ctx, cancel, log, opts.config)
		},
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	opts.addFlags(flags)

	cmd.AddCommand(getBootstrapCommand(opts))
	return cmd
}

func getBootstrapCommand(opts *options) *cobra.Command {
	bootstrapCmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap the " + Name,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			log, err := utils.InitRun(cmd, opts, "gardener-node-init")
			if err != nil {
				return err
			}
			return bootstrap.Bootstrap(cmd.Context(), log, afero.Afero{Fs: afero.NewOsFs()}, dbus.New(), opts.config.Bootstrap)
		},
	}

	flags := bootstrapCmd.Flags()
	verflag.AddFlags(flags)
	opts.addFlags(flags)

	return bootstrapCmd
}

func run(ctx context.Context, cancel context.CancelFunc, log logr.Logger, cfg *config.NodeAgentConfiguration) error {
	log.Info("Feature Gates", "featureGates", features.DefaultFeatureGate)

	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cfg.ClientConnection.Kubeconfig = kubeconfig
	}

	log.Info("Getting rest config")
	var (
		restConfig *rest.Config
		err        error
	)

	if len(cfg.ClientConnection.Kubeconfig) > 0 {
		restConfig, err = kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.ClientConnection, nil, kubernetes.AuthTokenFile)
		if err != nil {
			return fmt.Errorf("failed getting REST config from client connection configuration: %w", err)
		}
	} else {
		var mustFetchAccessToken bool
		restConfig, mustFetchAccessToken, err = getRESTConfig(log, cfg)
		if err != nil {
			return fmt.Errorf("failed getting REST config: %w", err)
		}

		if mustFetchAccessToken {
			log.Info("Fetching access token")
			if err := fetchAccessToken(ctx, log, restConfig, cfg.Controllers.Token.SecretName); err != nil {
				return fmt.Errorf("failed fetching access token: %w", err)
			}
		}
	}

	var extraHandlers map[string]http.Handler
	if cfg.Debugging != nil && cfg.Debugging.EnableProfiling {
		extraHandlers = routes.ProfilingHandlers
		if cfg.Debugging.EnableContentionProfiling {
			goruntime.SetBlockProfileRate(1)
		}
	}

	log.Info("Fetching hostname")
	hostName, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed fetching hostname: %w", err)
	}

	log.Info("Checking whether kubelet bootstrap kubeconfig needs to be created")
	fs := afero.Afero{Fs: afero.NewOsFs()}
	if err := createKubeletBootstrapKubeconfig(log, fs, cfg.APIServer); err != nil {
		return err
	}

	log.Info("Setting up manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Logger:                  log,
		Scheme:                  kubernetes.SeedScheme,
		GracefulShutdownTimeout: pointer.Duration(5 * time.Second),

		HealthProbeBindAddress: net.JoinHostPort(cfg.Server.HealthProbes.BindAddress, strconv.Itoa(cfg.Server.HealthProbes.Port)),
		Metrics: metricsserver.Options{
			BindAddress:   net.JoinHostPort(cfg.Server.Metrics.BindAddress, strconv.Itoa(cfg.Server.Metrics.Port)),
			ExtraHandlers: extraHandlers,
		},

		Cache: cache.Options{ByObject: map[client.Object]cache.ByObject{
			&corev1.Secret{}: {Namespaces: map[string]cache.Config{metav1.NamespaceSystem: {}}},
			&corev1.Node{}:   {Label: labels.SelectorFromSet(labels.Set{corev1.LabelHostname: hostName})},
		}},
		LeaderElection: false,
		Controller: controllerconfig.Controller{
			RecoverPanic: pointer.Bool(true),
		},
	})
	if err != nil {
		return err
	}

	log.Info("Setting up health check endpoints")
	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
		return err
	}

	log.Info("Adding controllers to manager")
	if err := controller.AddToManager(cancel, mgr, cfg, hostName); err != nil {
		return fmt.Errorf("failed adding controllers to manager: %w", err)
	}

	log.Info("Adding legacy cloud-config-downloader cleaner to manager")
	if err := mgr.Add(cleanupLegacyCloudConfigDownloader(log, fs, dbus.New())); err != nil {
		return fmt.Errorf("failed adding legacy cloud-config-downloader cleaner to manager: %w", err)
	}

	log.Info("Starting manager")
	return mgr.Start(ctx)
}

func getRESTConfig(log logr.Logger, cfg *config.NodeAgentConfiguration) (*rest.Config, bool, error) {
	restConfig := &rest.Config{
		Burst: int(cfg.ClientConnection.Burst),
		QPS:   cfg.ClientConnection.QPS,
		ContentConfig: rest.ContentConfig{
			AcceptContentTypes: cfg.ClientConnection.AcceptContentTypes,
			ContentType:        cfg.ClientConnection.ContentType,
		},
		Host:            cfg.APIServer.Server,
		TLSClientConfig: rest.TLSClientConfig{CAData: cfg.APIServer.CABundle},
		BearerTokenFile: nodeagentv1alpha1.TokenFilePath,
	}

	if _, err := os.Stat(restConfig.BearerTokenFile); err != nil && !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("failed checking whether token file %q exists: %w", restConfig.BearerTokenFile, err)
	} else if err == nil {
		log.Info("Token file already exists, nothing to be done", "path", restConfig.BearerTokenFile)
		return restConfig, false, nil
	}

	if _, err := os.Stat(downloader.PathCredentialsToken); err != nil && !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("failed checking whether cloud-config-downloader token file %q exists: %w", downloader.PathCredentialsToken, err)
	} else if err == nil {
		log.Info("Token file does not exist, but legacy cloud-config-downloader token file does - using it", "path", downloader.PathCredentialsToken)
		restConfig.BearerTokenFile = downloader.PathCredentialsToken
		return restConfig, true, nil
	}

	if _, err := os.Stat(nodeagentv1alpha1.BootstrapTokenFilePath); err != nil && !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("failed checking whether bootstrap token file %q exists: %w", nodeagentv1alpha1.BootstrapTokenFilePath, err)
	} else if err == nil {
		log.Info("Token file does not exist, but bootstrap token file does - using it", "path", nodeagentv1alpha1.BootstrapTokenFilePath)
		restConfig.BearerTokenFile = nodeagentv1alpha1.BootstrapTokenFilePath
		return restConfig, true, nil
	}

	return nil, false, fmt.Errorf("unable to construct REST config (neither token file %q nor bootstrap token file %q exist)", nodeagentv1alpha1.TokenFilePath, nodeagentv1alpha1.BootstrapTokenFilePath)
}

func fetchAccessToken(ctx context.Context, log logr.Logger, restConfig *rest.Config, tokenSecretName string) error {
	c, err := client.New(restConfig, client.Options{})
	if err != nil {
		return fmt.Errorf("unable to create client with bootstrap token: %w", err)
	}

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: tokenSecretName, Namespace: metav1.NamespaceSystem}}
	log.Info("Reading access token secret", "secret", client.ObjectKeyFromObject(secret))
	if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		return fmt.Errorf("failed fetching access token from API server: %w", err)
	}

	token := secret.Data[resourcesv1alpha1.DataKeyToken]
	if len(token) == 0 {
		return fmt.Errorf("secret key %q does not exist or empty", resourcesv1alpha1.DataKeyToken)
	}

	log.Info("Writing downloaded access token to disk", "path", nodeagentv1alpha1.TokenFilePath)
	if err := os.MkdirAll(filepath.Dir(nodeagentv1alpha1.TokenFilePath), fs.ModeDir); err != nil {
		return fmt.Errorf("unable to create directory %q: %w", filepath.Dir(nodeagentv1alpha1.TokenFilePath), err)
	}
	if err := os.WriteFile(nodeagentv1alpha1.TokenFilePath, token, 0600); err != nil {
		return fmt.Errorf("unable to write access token to %s: %w", nodeagentv1alpha1.TokenFilePath, err)
	}

	log.Info("Token written to disk")
	restConfig.BearerTokenFile = nodeagentv1alpha1.TokenFilePath
	return nil
}

func cleanupLegacyCloudConfigDownloader(log logr.Logger, fs afero.Afero, db dbus.DBus) manager.RunnableFunc {
	return func(ctx context.Context) error {
		log.Info("Removing legacy directory", "path", downloader.PathCCDDirectory)
		if err := fs.RemoveAll(downloader.PathCCDDirectory); err != nil {
			return fmt.Errorf("failed to remove legacy directory %q: %w", downloader.PathCCDDirectory)
		}

		if _, err := fs.Stat(path.Join("/", "etc", "systemd", "system", downloader.UnitName)); err != nil {
			if !errors.Is(err, afero.ErrFileNotFound) {
				return fmt.Errorf("failed checking whether unit file for %q still exists: %w", downloader.UnitName, err)
			}
			return nil
		}

		log.Info("Stopping legacy unit", "unit", downloader.UnitName)
		if err := db.Stop(ctx, nil, nil, downloader.UnitName); err != nil {
			return fmt.Errorf("failed to stop legacy system unit %s: %w", downloader.UnitName, err)
		}

		log.Info("Disabling legacy unit", "unit", downloader.UnitName)
		if err := db.Disable(ctx, downloader.UnitName); err != nil {
			return fmt.Errorf("failed to disable legacy system unit %s: %w", downloader.UnitName, err)
		}

		return nil
	}
}

func createKubeletBootstrapKubeconfig(log logr.Logger, fs afero.Afero, apiServerConfig config.APIServer) error {
	bootstrapToken, err := fs.ReadFile(nodeagentv1alpha1.BootstrapTokenFilePath)
	if err != nil {
		if !errors.Is(err, afero.ErrFileNotFound) {
			return fmt.Errorf("failed checking whether bootstrap token file %q already exists: %w", nodeagentv1alpha1.BootstrapTokenFilePath, err)
		}
		log.Info("Bootstrap token file does not exist, nothing to be done", "path", nodeagentv1alpha1.BootstrapTokenFilePath)
		return nil
	}

	if _, err := fs.Stat(kubelet.PathKubeconfigReal); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
		return fmt.Errorf("failed checking whether kubelet kubeconfig file %q already exists: %w", kubelet.PathKubeconfigReal, err)
	} else if err == nil {
		log.Info("Kubelet kubeconfig with client certificates already exists, nothing to be done", "path", kubelet.PathKubeconfigReal)
		return nil
	}

	kubeletClientCertificatePath := filepath.Join(kubelet.PathKubeletDirectory, "pki", "kubelet-client-current.pem")
	if _, err := fs.Stat(kubeletClientCertificatePath); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
		return fmt.Errorf("failed checking whether kubelet client certificate file %q already exists: %w", kubeletClientCertificatePath, err)
	} else if err == nil {
		log.Info("Kubelet client certificates file already exists, nothing to be done", "path", kubeletClientCertificatePath)
		return nil
	}

	log.Info("Creating kubelet directory", "path", kubelet.PathKubeletDirectory)
	if err := fs.MkdirAll(kubelet.PathKubeletDirectory, os.ModeDir); err != nil {
		return fmt.Errorf("unable to create kubelet directory %q: %w", kubelet.PathKubeletDirectory, err)
	}

	kubeconfig, err := runtime.Encode(clientcmdlatest.Codec, kubernetesutils.NewKubeconfig(
		"kubelet-bootstrap",
		clientcmdv1.Cluster{Server: apiServerConfig.Server, CertificateAuthorityData: apiServerConfig.CABundle},
		clientcmdv1.AuthInfo{Token: strings.TrimSpace(string(bootstrapToken))},
	))
	if err != nil {
		return fmt.Errorf("unable to encode kubeconfig: %w", err)
	}

	log.Info("Writing kubelet bootstrap kubeconfig file", "path", kubelet.PathKubeconfigBootstrap)
	return fs.WriteFile(kubelet.PathKubeconfigBootstrap, kubeconfig, 0600)
}
