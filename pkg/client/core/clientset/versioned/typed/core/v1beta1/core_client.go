/*
Copyright (c) SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package v1beta1

import (
	"net/http"

	v1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/core/clientset/versioned/scheme"
	rest "k8s.io/client-go/rest"
)

type CoreV1beta1Interface interface {
	RESTClient() rest.Interface
	BackupBucketsGetter
	BackupEntriesGetter
	CloudProfilesGetter
	ControllerDeploymentsGetter
	ControllerInstallationsGetter
	ControllerRegistrationsGetter
	ExposureClassesGetter
	ProjectsGetter
	QuotasGetter
	SecretBindingsGetter
	SeedsGetter
	ShootsGetter
	ShootStatesGetter
}

// CoreV1beta1Client is used to interact with features provided by the core.gardener.cloud group.
type CoreV1beta1Client struct {
	restClient rest.Interface
}

func (c *CoreV1beta1Client) BackupBuckets() BackupBucketInterface {
	return newBackupBuckets(c)
}

func (c *CoreV1beta1Client) BackupEntries(namespace string) BackupEntryInterface {
	return newBackupEntries(c, namespace)
}

func (c *CoreV1beta1Client) CloudProfiles() CloudProfileInterface {
	return newCloudProfiles(c)
}

func (c *CoreV1beta1Client) ControllerDeployments() ControllerDeploymentInterface {
	return newControllerDeployments(c)
}

func (c *CoreV1beta1Client) ControllerInstallations() ControllerInstallationInterface {
	return newControllerInstallations(c)
}

func (c *CoreV1beta1Client) ControllerRegistrations() ControllerRegistrationInterface {
	return newControllerRegistrations(c)
}

func (c *CoreV1beta1Client) ExposureClasses() ExposureClassInterface {
	return newExposureClasses(c)
}

func (c *CoreV1beta1Client) Projects() ProjectInterface {
	return newProjects(c)
}

func (c *CoreV1beta1Client) Quotas(namespace string) QuotaInterface {
	return newQuotas(c, namespace)
}

func (c *CoreV1beta1Client) SecretBindings(namespace string) SecretBindingInterface {
	return newSecretBindings(c, namespace)
}

func (c *CoreV1beta1Client) Seeds() SeedInterface {
	return newSeeds(c)
}

func (c *CoreV1beta1Client) Shoots(namespace string) ShootInterface {
	return newShoots(c, namespace)
}

func (c *CoreV1beta1Client) ShootStates(namespace string) ShootStateInterface {
	return newShootStates(c, namespace)
}

// NewForConfig creates a new CoreV1beta1Client for the given config.
// NewForConfig is equivalent to NewForConfigAndClient(c, httpClient),
// where httpClient was generated with rest.HTTPClientFor(c).
func NewForConfig(c *rest.Config) (*CoreV1beta1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	httpClient, err := rest.HTTPClientFor(&config)
	if err != nil {
		return nil, err
	}
	return NewForConfigAndClient(&config, httpClient)
}

// NewForConfigAndClient creates a new CoreV1beta1Client for the given config and http client.
// Note the http client provided takes precedence over the configured transport values.
func NewForConfigAndClient(c *rest.Config, h *http.Client) (*CoreV1beta1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientForConfigAndClient(&config, h)
	if err != nil {
		return nil, err
	}
	return &CoreV1beta1Client{client}, nil
}

// NewForConfigOrDie creates a new CoreV1beta1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *CoreV1beta1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new CoreV1beta1Client for the given RESTClient.
func New(c rest.Interface) *CoreV1beta1Client {
	return &CoreV1beta1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1beta1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *CoreV1beta1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
