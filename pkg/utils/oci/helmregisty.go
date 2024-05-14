// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"context"
	"net"
	"net/http"
	"time"

	"helm.sh/helm/v3/pkg/registry"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
)

// Interface represents an OCI compatible regisry.
type Interface interface {
	Pull(ctx context.Context, oci *gardencorev1.OCIRepository) ([]byte, error)
}

type HelmRegistry struct {
	client *registry.Client
}

func NewHelmRegistry() (*HelmRegistry, error) {
	client, err := registry.NewClient(registry.ClientOptHTTPClient(&http.Client{
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
	}))
	if err != nil {
		return nil, err
	}
	return &HelmRegistry{
		client: client,
	}, nil
}

func (r *HelmRegistry) Pull(_ context.Context, oci *gardencorev1.OCIRepository) ([]byte, error) {
	ref := buildRef(oci)
	// TODO: use oras directly so we can leverage the memory store
	res, err := r.client.Pull(ref)
	if err != nil {
		return nil, err
	}
	return res.Chart.Data, nil
}

func buildRef(oci *gardencorev1.OCIRepository) string {
	if oci.Tag != "" {
		return oci.URL + ":" + oci.Tag
	}
	if oci.Digest != "" {
		return oci.URL + "@" + oci.Digest
	}
	// should not be reachable
	return oci.URL
}
