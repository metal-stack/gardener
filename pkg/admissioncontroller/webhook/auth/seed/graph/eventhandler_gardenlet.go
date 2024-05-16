// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func (g *graph) setupGardenletWatch(ctx context.Context, informer cache.Informer) error {
	_, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if gardenlet, ok := obj.(*seedmanagementv1alpha1.Gardenlet); ok {
				g.handleGardenletCreateOrUpdate(ctx, gardenlet)
				return
			}
		},

		UpdateFunc: func(newObj, _ interface{}) {
			if gardenlet, ok := newObj.(*seedmanagementv1alpha1.Gardenlet); ok {
				g.handleGardenletCreateOrUpdate(ctx, gardenlet)
				return
			}
		},

		DeleteFunc: func(obj interface{}) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}

			if gardenlet, ok := obj.(*seedmanagementv1alpha1.Gardenlet); ok {
				g.handleGardenletDelete(gardenlet.Name, gardenlet.Namespace)
				return
			}
		},
	})
	return err
}

func (g *graph) handleGardenletCreateOrUpdate(ctx context.Context, gardenlet *seedmanagementv1alpha1.Gardenlet) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("Gardenlet", "Create").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	var (
		gardenletVertex = g.getOrCreateVertex(VertexTypeGardenlet, gardenlet.Namespace, gardenlet.Name)
		seedVertex      = g.getOrCreateVertex(VertexTypeSeed, "", gardenlet.Name)
	)

	g.addEdge(gardenletVertex, seedVertex)

	var allowBootstrap bool

	seed := &gardencorev1beta1.Seed{}
	if err := g.client.Get(ctx, kubernetesutils.Key(gardenlet.Name), seed); err != nil {
		if !apierrors.IsNotFound(err) {
			return
		}
	} else if seed.Status.ClientCertificateExpirationTimestamp != nil && seed.Status.ClientCertificateExpirationTimestamp.UTC().Before(time.Now().UTC()) {
		// Seed is registered but the client certificate expiration timestamp is expired.
		allowBootstrap = true
	} else if gardenlet.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRenewKubeconfig {
		allowBootstrap = true
	}

	if allowBootstrap {
		secretVertex := g.getOrCreateVertex(VertexTypeSecret, metav1.NamespaceSystem, bootstraptokenapi.BootstrapTokenSecretPrefix+gardenletbootstraputil.TokenID(gardenlet.ObjectMeta))
		g.addEdge(secretVertex, gardenletVertex)
	}
}

func (g *graph) handleGardenletDelete(name, namespace string) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("Gardenlet", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeGardenlet, namespace, name)
}
