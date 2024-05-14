// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/registry"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
)

// Interface represents an OCI compatible regisry.
type Interface interface {
	Pull(ctx context.Context, oci *gardencorev1.OCIRepository) ([]byte, error)
}

type HelmRegistry struct {
}

func NewHelmRegistry() (*HelmRegistry, error) {
	return &HelmRegistry{}, nil
}

const (
	localRegistry        = "localhost:5001"
	inKubernetesRegistry = "garden.local.gardener.cloud:5001"
)

func (r *HelmRegistry) Pull(_ context.Context, oci *gardencorev1.OCIRepository) ([]byte, error) {
	ref := buildRef(oci)
	insecure := false
	// rewrite registry URL for gardener-local setup
	if strings.Contains(ref, localRegistry) {
		insecure = true
		ref = strings.Replace(ref, localRegistry, inKubernetesRegistry, 1)
	}
	client, err := newClient(insecure)
	if err != nil {
		return nil, err
	}
	// TODO: use oras directly so we can leverage the memory store
	res, err := client.Pull(strings.TrimPrefix(ref, "oci://"))
	if err != nil {
		return nil, err
	}
	return res.Chart.Data, nil
}

func buildRef(oci *gardencorev1.OCIRepository) string {
	if oci.URL != "" {
		return oci.URL
	}
	if oci.Digest != "" {
		return oci.Repository + "@" + oci.Digest
	}
	if oci.Tag != "" {
		return oci.Repository + ":" + oci.Tag
	}
	// should not be reachable
	return oci.Repository
}

func newClient(insecure bool) (*registry.Client, error) {
	opts := []registry.ClientOption{
		registry.ClientOptHTTPClient(&http.Client{
			Transport: &http.Transport{
				// From https://github.com/google/go-containerregistry/blob/31786c6cbb82d6ec4fb8eb79cd9387905130534e/pkg/v1/remote/options.go#L87
				DisableCompression: true,
				DialContext: (&net.Dialer{
					// By default we wrap the transport in retries, so reduce the
					// default dial timeout to 5s to avoid 5x 30s of connection
					// timeouts when doing the "ping" on certain http registries.
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				Proxy:                 http.ProxyFromEnvironment,
			},
		}),
	}
	if insecure {
		opts = append(opts, registry.ClientOptPlainHTTP())
	}
	return registry.NewClient(opts...)
}
