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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/automaxprocs/maxprocs"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	configlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfigv1alpha1 "sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/nodeagent/bootstrap"
	"github.com/gardener/gardener/pkg/nodeagent/controller"
	"github.com/gardener/gardener/pkg/nodeagent/controller/common"

	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

// Name is a const for the name of this component.
const name = "gardener-node-agent"

// NewCommand creates a new cobra.Command for running gardener-node-agent.
func NewCommand() *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch the " + name,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			verflag.PrintAndExitIfRequested()

			log, err := initLogging(opts)
			if err != nil {
				return err
			}

			logf.SetLogger(log)
			klog.SetLogger(log)

			log.Info("Starting "+name, "version", version.Get())
			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				log.Info(fmt.Sprintf("FLAG: --%s=%s", flag.Name, flag.Value)) //nolint:logcheck
			})

			// don't output usage on further errors raised during execution
			cmd.SilenceUsage = true
			// further errors will be logged properly, don't duplicate
			cmd.SilenceErrors = true

			return run(cmd.Context(), log, opts.config)
		},
	}

	bootstrapCmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "bootstrap the " + name,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			log, err := initLogging(opts)
			if err != nil {
				return err
			}

			return bootstrap.Bootstrap(cmd.Context(), log)
		},
	}

	cmd.AddCommand(bootstrapCmd)
	flags := cmd.Flags()
	verflag.AddFlags(flags)
	opts.addFlags(flags)

	return cmd
}

func run(ctx context.Context, log logr.Logger, cfg *config.ControllerManagerConfiguration) error {
	log.Info("Feature Gates", "featureGates", features.DefaultFeatureGate)

	// This is like importing the automaxprocs package for its init func (it will in turn call maxprocs.Set).
	// Here we pass a custom logger, so that the result of the library gets logged to the same logger we use for the
	// component itself.
	if _, err := maxprocs.Set(maxprocs.Logger(func(s string, i ...interface{}) {
		log.Info(fmt.Sprintf(s, i...)) //nolint:logcheck
	})); err != nil {
		log.Error(err, "Failed to set GOMAXPROCS")
	}

	// Check if token is present, else use bootstrap token to fetch token

	config, err := common.ReadNodeAgentConfiguration()
	if err != nil {
		return err
	}

	_, err = os.Stat(nodeagentv1alpha1.NodeAgentTokenFilePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to read token %w", err)
	}
	if err != nil && os.IsNotExist(err) {
		log.Info("token not present, fetching from token from api server")

		// Fetch token with bootstrap token and store it on disk
		restConfig := &rest.Config{
			Host:        config.APIServer.URL,
			BearerToken: config.APIServer.BootstrapToken,
			TLSClientConfig: rest.TLSClientConfig{
				CAData: []byte(config.APIServer.CA),
			},
		}

		// this can be removed after migration to gardener-node-agent has happened:
		if _, err := os.Stat(downloader.PathCredentialsToken); err == nil {
			restConfig = &rest.Config{
				Host:            config.APIServer.URL,
				BearerTokenFile: downloader.PathCredentialsToken,
				TLSClientConfig: rest.TLSClientConfig{
					CAData: []byte(config.APIServer.CA),
				},
			}
		}

		c, err := client.New(restConfig, client.Options{})
		if err != nil {
			return fmt.Errorf("unable to create runtime client %w", err)
		}

		tokenSecret := &corev1.Secret{}
		err = c.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: nodeagentv1alpha1.NodeAgentTokenSecretName}, tokenSecret)
		if err != nil {
			return fmt.Errorf("unable to fetch token %w", err)
		}

		log.Info("token fetched from api server")
		err = os.WriteFile(nodeagentv1alpha1.NodeAgentTokenFilePath, tokenSecret.Data[nodeagentv1alpha1.NodeAgentTokenSecretKey], 0600)
		if err != nil {
			return fmt.Errorf("unable to write token %w", err)
		}

		log.Info("token written to disk")
	}

	err = writeKubeconfigBootstrap(config)
	if err != nil {
		return err
	}

	log.Info("token found, creating restconfig")
	restConfig := &rest.Config{
		Host:            config.APIServer.URL,
		BearerTokenFile: nodeagentv1alpha1.NodeAgentTokenFilePath,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: []byte(config.APIServer.CA),
		},
	}

	log.Info("Setting up manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Logger: log,
		// TODO: refine cache selector to allow only access to needed secrets instead
		Namespace:               metav1.NamespaceSystem,
		Scheme:                  kubernetes.ShootScheme,
		GracefulShutdownTimeout: pointer.Duration(5 * time.Second),
		LeaderElection:          false,
		Controller: controllerconfigv1alpha1.ControllerConfigurationSpec{
			RecoverPanic: pointer.Bool(true),
		},
	})
	if err != nil {
		return err
	}

	log.Info("Adding controllers to manager")
	if err := controller.AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding controllers to manager: %w", err)
	}

	log.Info("Starting manager")
	return mgr.Start(ctx)
}

func initLogging(opts *options) (logr.Logger, error) {
	log, err := logger.NewZapLogger(logger.DebugLevel, logger.FormatJSON)
	if err != nil {
		return logr.Logger{}, fmt.Errorf("error instantiating zap logger: %w", err)
	}
	return log, nil
}

func writeKubeconfigBootstrap(config *nodeagentv1alpha1.NodeAgentConfiguration) error {
	if _, err := os.Stat(kubelet.PathKubeconfigBootstrap); err == nil {
		return nil
	}

	kubeconfig := &clientcmdv1.Config{
		Kind:       "Config",
		APIVersion: "v1",
		Clusters: []clientcmdv1.NamedCluster{
			{
				Name: "default",
				Cluster: clientcmdv1.Cluster{
					Server:                   config.APIServer.URL,
					CertificateAuthorityData: []byte(config.APIServer.CA),
				},
			},
		},
		CurrentContext: "kubelet-bootstrap@default",
		Contexts: []clientcmdv1.NamedContext{
			{
				Name: "kubelet-bootstrap@default",
				Context: clientcmdv1.Context{
					Cluster:  "default",
					AuthInfo: "kubelet-bootstrap",
				},
			},
		},
		AuthInfos: []clientcmdv1.NamedAuthInfo{
			{
				Name: "kubelet-bootstrap",
				AuthInfo: clientcmdv1.AuthInfo{
					Token:                config.APIServer.BootstrapToken,
					ImpersonateUserExtra: make(map[string][]string),
				},
			},
		},
	}

	raw, err := runtime.Encode(configlatest.Codec, kubeconfig)
	if err != nil {
		return fmt.Errorf("unable to encode kubeconfig: %w", err)
	}

	dir := filepath.Dir(kubelet.PathKubeconfigBootstrap)

	err = os.MkdirAll(dir, fs.ModeDir)
	if err != nil {
		return fmt.Errorf("unable to create kubelet kubeconfig directory %w", err)
	}

	return os.WriteFile(kubelet.PathKubeconfigBootstrap, raw, 0600)
}
