// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seedrestriction

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/admissioncontroller/seedidentity"
	admissionwebhook "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	seedmanagementv1alpha1helper "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var (
	// Only take v1beta1 for the core.gardener.cloud API group because the Authorize function only checks the resource
	// group and the resource (but it ignores the version).
	backupBucketResource              = gardencorev1beta1.Resource("backupbuckets")
	backupEntryResource               = gardencorev1beta1.Resource("backupentries")
	bastionResource                   = operationsv1alpha1.Resource("bastions")
	certificateSigningRequestResource = certificatesv1.Resource("certificatesigningrequests")
	clusterRoleBindingResource        = rbacv1.Resource("clusterrolebindings")
	internalSecretResource            = gardencorev1beta1.Resource("internalsecrets")
	leaseResource                     = coordinationv1.Resource("leases")
	secretResource                    = corev1.Resource("secrets")
	seedResource                      = gardencorev1beta1.Resource("seeds")
	serviceAccountResource            = corev1.Resource("serviceaccounts")
	shootStateResource                = gardencorev1beta1.Resource("shootstates")
)

// Handler restricts requests made by gardenlets.
type Handler struct {
	Logger  logr.Logger
	Client  client.Reader
	decoder *admission.Decoder
}

// InjectDecoder injects the decoder.
func (h *Handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

// Handle restricts requests made by gardenlets.
func (h *Handler) Handle(ctx context.Context, request admission.Request) admission.Response {
	seedName, isSeed := seedidentity.FromAuthenticationV1UserInfo(request.UserInfo)
	if !isSeed {
		return admissionwebhook.Allowed("")
	}

	requestResource := schema.GroupResource{Group: request.Resource.Group, Resource: request.Resource.Resource}
	switch requestResource {
	case backupBucketResource:
		return h.admitBackupBucket(ctx, seedName, request)
	case backupEntryResource:
		return h.admitBackupEntry(ctx, seedName, request)
	case bastionResource:
		return h.admitBastion(seedName, request)
	case certificateSigningRequestResource:
		return h.admitCertificateSigningRequest(seedName, request)
	case clusterRoleBindingResource:
		return h.admitClusterRoleBinding(ctx, seedName, request)
	case internalSecretResource:
		return h.admitInternalSecret(ctx, seedName, request)
	case leaseResource:
		return h.admitLease(seedName, request)
	case secretResource:
		return h.admitSecret(ctx, seedName, request)
	case seedResource:
		return h.admitSeed(ctx, seedName, request)
	case serviceAccountResource:
		return h.admitServiceAccount(ctx, seedName, request)
	case shootStateResource:
		return h.admitShootState(ctx, seedName, request)
	}

	return admissionwebhook.Allowed("")
}

func (h *Handler) admitBackupBucket(ctx context.Context, seedName string, request admission.Request) admission.Response {
	switch request.Operation {
	case admissionv1.Create:
		// If a gardenlet tries to create a BackupBucket then the request may only be allowed if the used `.spec.seedName`
		// is equal to the gardenlet's seed.
		backupBucket := &gardencorev1beta1.BackupBucket{}
		if err := h.decoder.Decode(request, backupBucket); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		return h.admit(seedName, backupBucket.Spec.SeedName)

	case admissionv1.Delete:
		// If a gardenlet tries to delete a BackupBucket then it may only be allowed if the name is equal to the UID of
		// the gardenlet's seed.
		seed := &gardencorev1beta1.Seed{}
		if err := h.Client.Get(ctx, kubernetesutils.Key(seedName), seed); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if string(seed.UID) != request.Name {
			return admission.Errored(http.StatusForbidden, fmt.Errorf("cannot delete unrelated BackupBucket"))
		}
		return admission.Allowed("")
	}

	return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
}

func (h *Handler) admitBackupEntry(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	backupEntry := &gardencorev1beta1.BackupEntry{}
	if err := h.decoder.Decode(request, backupEntry); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if resp := h.admit(seedName, backupEntry.Spec.SeedName); !resp.Allowed {
		return resp
	}

	if strings.HasPrefix(backupEntry.Name, v1beta1constants.BackupSourcePrefix) {
		return h.admitSourceBackupEntry(ctx, backupEntry)
	}

	backupBucket := &gardencorev1beta1.BackupBucket{}
	if err := h.Client.Get(ctx, kubernetesutils.Key(backupEntry.Spec.BucketName), backupBucket); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return h.admit(seedName, backupBucket.Spec.SeedName)
}

func (h *Handler) admitSourceBackupEntry(ctx context.Context, backupEntry *gardencorev1beta1.BackupEntry) admission.Response {
	// The source BackupEntry is created during the restore phase of control plane migration
	// so allow creations only if the shoot that owns the BackupEntry is currently being restored.
	shootName := gardenerutils.GetShootNameFromOwnerReferences(backupEntry)
	shoot := &gardencorev1beta1.Shoot{}
	if err := h.Client.Get(ctx, kubernetesutils.Key(backupEntry.Namespace, shootName), shoot); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if shoot.Status.LastOperation == nil || shoot.Status.LastOperation.Type != gardencorev1beta1.LastOperationTypeRestore ||
		shoot.Status.LastOperation.State != gardencorev1beta1.LastOperationStateProcessing {
		return admission.Errored(http.StatusForbidden, fmt.Errorf("creation of source BackupEntry is only allowed during shoot Restore operation (shoot: %s)", shootName))
	}

	// When the source BackupEntry is created it's spec is the same as that of the shoot's original BackupEntry.
	// The original BackupEntry is modified after the source BackupEntry has been deployed and successfully reconciled.
	shootBackupEntryName := strings.TrimPrefix(backupEntry.Name, fmt.Sprintf("%s-", v1beta1constants.BackupSourcePrefix))
	shootBackupEntry := &gardencorev1beta1.BackupEntry{}
	if err := h.Client.Get(ctx, kubernetesutils.Key(backupEntry.Namespace, shootBackupEntryName), shootBackupEntry); err != nil {
		if apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusForbidden, fmt.Errorf("could not find original BackupEntry %s: %w", shootBackupEntryName, err))
		}
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if !apiequality.Semantic.DeepEqual(backupEntry.Spec, shootBackupEntry.Spec) {
		return admission.Errored(http.StatusForbidden, fmt.Errorf("specification of source BackupEntry must equal specification of original BackupEntry %s", shootBackupEntryName))
	}

	return admission.Allowed("")
}

func (h *Handler) admitBastion(seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	bastion := &operationsv1alpha1.Bastion{}
	if err := h.decoder.Decode(request, bastion); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	return h.admit(seedName, bastion.Spec.SeedName)
}

func (h *Handler) admitCertificateSigningRequest(seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	csr := &certificatesv1.CertificateSigningRequest{}
	if err := h.decoder.Decode(request, csr); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	x509cr, err := utils.DecodeCertificateRequest(csr.Spec.Request)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if ok, reason := gardenerutils.IsSeedClientCert(x509cr, csr.Spec.Usages); !ok {
		return admission.Errored(http.StatusForbidden, fmt.Errorf("can only create CSRs for seed clusters: %s", reason))
	}

	seedNameInCSR, _ := seedidentity.FromCertificateSigningRequest(x509cr)
	return h.admit(seedName, &seedNameInCSR)
}

func (h *Handler) admitClusterRoleBinding(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// Allow gardenlet to create cluster role bindings referencing service accounts which can be used to bootstrap other
	// gardenlets deployed as part of the ManagedSeed reconciliation.
	if strings.HasPrefix(request.Name, gardenletbootstraputil.ClusterRoleBindingNamePrefix) {
		clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
		if err := h.decoder.Decode(request, clusterRoleBinding); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if clusterRoleBinding.RoleRef.APIGroup != rbacv1.GroupName ||
			clusterRoleBinding.RoleRef.Kind != "ClusterRole" ||
			clusterRoleBinding.RoleRef.Name != gardenletbootstraputil.GardenerSeedBootstrapper {

			return admission.Errored(http.StatusForbidden, fmt.Errorf("can only bindings referring to the bootstrapper role"))
		}

		managedSeedNamespace, managedSeedName := gardenletbootstraputil.ManagedSeedInfoFromClusterRoleBindingName(request.Name)
		return h.allowIfManagedSeedIsNotYetBootstrapped(ctx, seedName, managedSeedNamespace, managedSeedName)
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *Handler) admitInternalSecret(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// Check if the internal secret is related to a Shoot assigned to the seed the gardenlet is responsible for.
	if shootName, ok := gardenerutils.IsShootProjectInternalSecret(request.Name); ok {
		shoot := &gardencorev1beta1.Shoot{}
		if err := h.Client.Get(ctx, kubernetesutils.Key(request.Namespace, shootName), shoot); err != nil {
			if apierrors.IsNotFound(err) {
				return admission.Errored(http.StatusForbidden, err)
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return h.admit(seedName, shoot.Spec.SeedName)
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *Handler) admitLease(seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// This allows the gardenlet to create a Lease for leader election (if the garden cluster is a seed as well).
	if request.Name == "gardenlet-leader-election" {
		return admission.Allowed("")
	}

	// Each gardenlet creates a Lease with the name of its own seed in the `gardener-system-seed-lease` namespace.
	if request.Namespace == gardencorev1beta1.GardenerSeedLeaseNamespace {
		return h.admit(seedName, &request.Name)
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *Handler) admitSecret(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// Check if the secret is related to a BackupBucket assigned to the seed the gardenlet is responsible for.
	if strings.HasPrefix(request.Name, v1beta1constants.SecretPrefixGeneratedBackupBucket) {
		backupBucket := &gardencorev1beta1.BackupBucket{}
		if err := h.Client.Get(ctx, kubernetesutils.Key(strings.TrimPrefix(request.Name, v1beta1constants.SecretPrefixGeneratedBackupBucket)), backupBucket); err != nil {
			if apierrors.IsNotFound(err) {
				return admission.Errored(http.StatusForbidden, err)
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return h.admit(seedName, backupBucket.Spec.SeedName)
	}

	// Check if the secret is related to a Shoot assigned to the seed the gardenlet is responsible for.
	if shootName, ok := gardenerutils.IsShootProjectSecret(request.Name); ok {
		shoot := &gardencorev1beta1.Shoot{}
		if err := h.Client.Get(ctx, kubernetesutils.Key(request.Namespace, shootName), shoot); err != nil {
			if apierrors.IsNotFound(err) {
				return admission.Errored(http.StatusForbidden, err)
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return h.admit(seedName, shoot.Spec.SeedName)
	}

	// Check if the secret is a bootstrap token for a ManagedSeed.
	if strings.HasPrefix(request.Name, bootstraptokenapi.BootstrapTokenSecretPrefix) && request.Namespace == metav1.NamespaceSystem {
		secret := &corev1.Secret{}
		if err := h.decoder.Decode(request, secret); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if secret.Type != corev1.SecretTypeBootstrapToken {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("unexpected secret type: %q", secret.Type))
		}
		if string(secret.Data[bootstraptokenapi.BootstrapTokenUsageAuthentication]) != "true" {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("%q must be set to 'true'", bootstraptokenapi.BootstrapTokenUsageAuthentication))
		}
		if string(secret.Data[bootstraptokenapi.BootstrapTokenUsageSigningKey]) != "true" {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("%q must be set to 'true'", bootstraptokenapi.BootstrapTokenUsageSigningKey))
		}
		if _, ok := secret.Data[bootstraptokenapi.BootstrapTokenExtraGroupsKey]; ok {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("%q must not be set", bootstraptokenapi.BootstrapTokenExtraGroupsKey))
		}

		managedSeedNamespace, managedSeedName := gardenletbootstraputil.MetadataFromDescription(
			string(secret.Data[bootstraptokenapi.BootstrapTokenDescriptionKey]),
			gardenletbootstraputil.KindManagedSeed,
		)

		return h.allowIfManagedSeedIsNotYetBootstrapped(ctx, seedName, managedSeedNamespace, managedSeedName)
	}

	// Check if the secret is related to a ManagedSeed assigned to the seed the gardenlet is responsible for.
	managedSeedList := &seedmanagementv1alpha1.ManagedSeedList{}
	if err := h.Client.List(ctx, managedSeedList); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	for _, managedSeed := range managedSeedList.Items {
		shoot := &gardencorev1beta1.Shoot{}
		if err := h.Client.Get(ctx, kubernetesutils.Key(managedSeed.Namespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if !h.admit(seedName, shoot.Spec.SeedName).Allowed {
			continue
		}

		seedTemplate, _, err := seedmanagementv1alpha1helper.ExtractSeedTemplateAndGardenletConfig(&managedSeed)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if seedTemplate.Spec.SecretRef != nil &&
			seedTemplate.Spec.SecretRef.Namespace == request.Namespace &&
			seedTemplate.Spec.SecretRef.Name == request.Name {
			return admission.Allowed("")
		}

		if seedTemplate.Spec.Backup != nil &&
			seedTemplate.Spec.Backup.SecretRef.Namespace == request.Namespace &&
			seedTemplate.Spec.Backup.SecretRef.Name == request.Name {
			return admission.Allowed("")
		}
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *Handler) admitSeed(ctx context.Context, seedName string, request admission.Request) admission.Response {
	response := h.admit(seedName, &request.Name)
	if request.Operation == admissionv1.Delete && !response.Allowed {
		// If the deletion request is not allowed, then it might be submitted by the "parent gardenlet".
		// This is the gardenlet/seed which is responsible for the `managedseed` in question.
		managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
		if err := h.Client.Get(ctx, kubernetesutils.Key(v1beta1constants.GardenNamespace, request.Name), managedSeed); err != nil {
			if apierrors.IsNotFound(err) {
				return response
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}

		// If a gardenlet tries to delete a Seed belonging to a ManagedSeed then the request may only be considered
		// further if the `.spec.deletionTimestamp` is set (gardenlets themselves are not allowed to delete ManagedSeeds,
		// so it's safe to only continue if somebody else has set this deletion timestamp).
		if managedSeed.DeletionTimestamp == nil {
			return admission.Errored(http.StatusForbidden, fmt.Errorf("object can only be deleted if corresponding ManagedSeed has a deletion timestamp"))
		}

		// If for whatever reason the `.spec.shoot` is nil then we exit early.
		if managedSeed.Spec.Shoot == nil {
			return response
		}

		// Check if the `.spec.seedName` of the Shoot referenced in the `.spec.shoot.name` field of the ManagedSeed matches
		// the seed name of the requesting gardenlet.
		shoot := &gardencorev1beta1.Shoot{}
		if err := h.Client.Get(ctx, kubernetesutils.Key(managedSeed.Namespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return h.admit(seedName, shoot.Spec.SeedName)
	}

	return response
}

func (h *Handler) admitServiceAccount(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// Allow gardenlet to create service accounts which can be used to bootstrap other gardenlets deployed as part of
	// the ManagedSeed reconciliation.
	if strings.HasPrefix(request.Name, gardenletbootstraputil.ServiceAccountNamePrefix) {
		return h.allowIfManagedSeedIsNotYetBootstrapped(ctx, seedName, request.Namespace, strings.TrimPrefix(request.Name, gardenletbootstraputil.ServiceAccountNamePrefix))
	}

	// Allow all verbs for service accounts in gardenlets' seed-<name> namespaces.
	if request.Namespace == gardenerutils.ComputeGardenNamespace(seedName) {
		return admission.Allowed("")
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *Handler) admitShootState(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := h.Client.Get(ctx, kubernetesutils.Key(request.Namespace, request.Name), shoot); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return h.admit(seedName, shoot.Spec.SeedName, shoot.Status.SeedName)
}

func (h *Handler) admit(seedName string, seedNamesForObject ...*string) admission.Response {
	// Allow request if one of the seed names for the object matches the seed name of the requesting user.
	for _, seedNameForObject := range seedNamesForObject {
		if seedNameForObject != nil && *seedNameForObject == seedName {
			return admission.Allowed("")
		}
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *Handler) allowIfManagedSeedIsNotYetBootstrapped(ctx context.Context, seedName, managedSeedNamespace, managedSeedName string) admission.Response {
	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := h.Client.Get(ctx, kubernetesutils.Key(managedSeedNamespace, managedSeedName), managedSeed); err != nil {
		if apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusForbidden, err)
		}
		return admission.Errored(http.StatusInternalServerError, err)
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := h.Client.Get(ctx, kubernetesutils.Key(managedSeed.Namespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if response := h.admit(seedName, shoot.Spec.SeedName); !response.Allowed {
		return response
	}

	seed := &gardencorev1beta1.Seed{}
	if err := h.Client.Get(ctx, kubernetesutils.Key(managedSeedName), seed); err != nil {
		if !apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.Allowed("")
	} else if seed.Status.ClientCertificateExpirationTimestamp != nil && seed.Status.ClientCertificateExpirationTimestamp.UTC().Before(time.Now().UTC()) {
		return admission.Allowed("")
	} else if managedSeed.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRenewKubeconfig {
		return admission.Allowed("")
	}

	return admission.Errored(http.StatusBadRequest, fmt.Errorf("managed seed %s/%s is already bootstrapped", managedSeed.Namespace, managedSeed.Name))
}
