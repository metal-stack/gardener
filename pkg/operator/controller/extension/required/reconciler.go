// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package required

import (
	"context"
	"fmt"
	"sync"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/operator"
	"github.com/gardener/gardener/pkg/operator/apis/config"
)

// Reconciler reconciles ControllerInstallations. It checks whether they are still required by using the
// <KindToRequiredTypes> map.
type Reconciler struct {
	RuntimeClientSet kubernetes.Interface
	RuntimeVersion   *semver.Version
	GardenClientMap  clientmap.ClientMap
	Config           config.OperatorConfiguration
	Clock            clock.Clock
	SeedName         string

	Lock                *sync.RWMutex
	KindToRequiredTypes map[string]sets.Set[string]
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	extension := &operatorv1alpha1.Extension{}
	if err := r.RuntimeClientSet.Client().Get(ctx, request.NamespacedName, extension); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}
	mergedSpec, err := operator.MergeExtensionSpecs(extension.Name, extension.Spec)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error merging extension spec for name %s: %w", extension.Name, err)
	}
	extension.Spec = mergedSpec

	var (
		allKindsCalculated = true
		required           *bool
		requiredKindTypes  = sets.New[string]()
		message            string
	)

	r.Lock.RLock()
	for _, resource := range extension.Spec.Resources {
		requiredTypes, ok := r.KindToRequiredTypes[resource.Kind]
		if !ok {
			allKindsCalculated = false
			continue
		}

		if requiredTypes.Has(resource.Type) {
			required = ptr.To(true)
			requiredKindTypes.Insert(fmt.Sprintf("%s/%s", resource.Kind, resource.Type))
		}
	}
	r.Lock.RUnlock()

	if required == nil {
		if !allKindsCalculated {
			// if required wasn't set yet then but not all kinds were calculated then the it's not possible to
			// decide yet whether it's required or not
			return reconcile.Result{}, nil
		}

		// if required wasn't set yet then but all kinds were calculated then the installation is no longer required
		required = ptr.To(false)
		message = "no extension objects exist having the kind/type combinations the controller is responsible for"
	} else if *required {
		message = fmt.Sprintf("extension objects still exist: %+v", requiredKindTypes.UnsortedList())
	}

	if err := updateExtensionRequiredCondition(ctx, r.RuntimeClientSet.Client(), r.Clock, extension, *required, message); err != nil {
		log.Error(err, "Failed updating required condition for extension", "extension", extension, "required", *required)
		return reconcile.Result{}, err
	}
	log.Info("Updated required condition for extension", "extension", extension, "required", *required)

	return reconcile.Result{}, nil
}

func updateExtensionRequiredCondition(ctx context.Context, c client.StatusClient, clock clock.Clock, extension *operatorv1alpha1.Extension, required bool, message string) error {
	var (
		conditionRequired = v1beta1helper.GetOrInitConditionWithClock(clock, extension.Status.Conditions, ExtensionRequired)

		status = gardencorev1beta1.ConditionTrue
		reason = "ExtensionObjectsExist"
	)

	if !required {
		status = gardencorev1beta1.ConditionFalse
		reason = "NoExtensionObjects"
	}

	patch := client.MergeFrom(extension.DeepCopy())
	extension.Status.Conditions = v1beta1helper.MergeConditions(
		extension.Status.Conditions,
		v1beta1helper.UpdatedConditionWithClock(clock, conditionRequired, status, reason, message),
	)

	return c.Status().Patch(ctx, extension, patch)
}
