/*
Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

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

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/authentication/user"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"

	authenticationapi "github.com/gardener/gardener/pkg/apis/authentication"
	authenticationv1alpha1 "github.com/gardener/gardener/pkg/apis/authentication/v1alpha1"
	authenticationvalidation "github.com/gardener/gardener/pkg/apis/authentication/validation"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// AdminKubeconfigREST implements a RESTStorage for shoots/adminkubeconfig.
type AdminKubeconfigREST struct {
	secretLister         kubecorev1listers.SecretLister
	internalSecretLister gardencorelisters.InternalSecretLister
	shootStateStorage    getter
	shootStorage         getter
	maxExpirationSeconds int64
}

var (
	_ = rest.NamedCreater(&AdminKubeconfigREST{})
	_ = rest.GroupVersionKindProvider(&AdminKubeconfigREST{})

	gvk = schema.GroupVersionKind{
		Group:   authenticationv1alpha1.SchemeGroupVersion.Group,
		Version: authenticationv1alpha1.SchemeGroupVersion.Version,
		Kind:    "AdminKubeconfigRequest",
	}
)

// New returns and instance of AdminKubeconfigRequest
func (r *AdminKubeconfigREST) New() runtime.Object {
	return &authenticationapi.AdminKubeconfigRequest{}
}

// Destroy cleans up its resources on shutdown.
func (r *AdminKubeconfigREST) Destroy() {
	// Given that underlying store is shared with REST,
	// we don't destroy it here explicitly.
}

// Create returns an AdminKubeconfigRequest with kubeconfig based on
// - shoot's advertised addresses
// - shoot's certificate authority
// - user making the request
func (r *AdminKubeconfigREST) Create(ctx context.Context, name string, obj runtime.Object, createValidation rest.ValidateObjectFunc, _ *metav1.CreateOptions) (runtime.Object, error) {
	if createValidation != nil {
		if err := createValidation(ctx, obj.DeepCopyObject()); err != nil {
			return nil, err
		}
	}

	out := obj.(*authenticationapi.AdminKubeconfigRequest)
	if errs := authenticationvalidation.ValidateAdminKubeconfigRequest(out); len(errs) != 0 {
		return nil, apierrors.NewInvalid(gvk.GroupKind(), "", errs)
	}

	userInfo, ok := genericapirequest.UserFrom(ctx)
	if !ok {
		return nil, apierrors.NewBadRequest("no user in context")
	}

	shootObj, err := r.shootStorage.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	shoot, ok := shootObj.(*core.Shoot)
	if !ok {
		return nil, apierrors.NewInternalError(fmt.Errorf("cannot convert to *core.Shoot object - got type %T", shootObj))
	}

	if len(shoot.Status.AdvertisedAddresses) == 0 {
		fieldErr := field.Invalid(field.NewPath("status", "status"), shoot.Status.AdvertisedAddresses, "no kube-apiserver advertised addresses in Shoot .status.advertisedAddresses")
		return nil, apierrors.NewInvalid(gvk.GroupKind(), shoot.Name, field.ErrorList{fieldErr})
	}

	var (
		clusterCABundle      []byte
		clientCACertificate  *secrets.Certificate
		fallbackToShootState = false
	)

	caClientSecret, err := r.internalSecretLister.InternalSecrets(shoot.Namespace).Get(gardenerutils.ComputeShootProjectSecretName(shoot.Name, gardenerutils.ShootProjectSecretSuffixCAClient))
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, apierrors.NewInternalError(fmt.Errorf("could not find client CA secret: %w", err))
		}
		fallbackToShootState = true
	} else {
		clientCACertificate, err = secrets.LoadCertificate("", caClientSecret.Data[secrets.DataKeyPrivateKeyCA], caClientSecret.Data[secrets.DataKeyCertificateCA])
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("could not load client CA certificate from secret: %w", err))
		}

		caClusterSecret, err := r.secretLister.Secrets(shoot.Namespace).Get(gardenerutils.ComputeShootProjectSecretName(shoot.Name, gardenerutils.ShootProjectSecretSuffixCACluster))
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("could not find cluster CA bundle: %w", err))
		}
		clusterCABundle = caClusterSecret.Data[secrets.DataKeyCertificateCA]

		if len(clusterCABundle) == 0 {
			return nil, apierrors.NewInternalError(fmt.Errorf("could not load cluster CA bundle from secret"))
		}
	}

	// TODO(timebertt): drop this fallback after v1.74 has been released.
	if fallbackToShootState {
		shootStateObj, err := r.shootStateStorage.Get(ctx, name, &metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		shootState := &gardencorev1beta1.ShootState{}
		if err := kubernetes.GardenScheme.Convert(shootStateObj, shootState, nil); err != nil {
			return nil, err
		}

		resourceDataList := v1beta1helper.GardenerResourceDataList(shootState.Spec.Gardener)

		clusterCABundle, err = getClusterCABundle(resourceDataList)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("could not find cluster CA bundle: %w", err))
		}

		clientCACertificate, err = getClientCACertificate(resourceDataList)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("could not find client CA certificate: %w", err))
		}
	}

	if r.maxExpirationSeconds > 0 && out.Spec.ExpirationSeconds > r.maxExpirationSeconds {
		out.Spec.ExpirationSeconds = r.maxExpirationSeconds
	}

	var (
		validity = time.Duration(out.Spec.ExpirationSeconds) * time.Second
		authName = fmt.Sprintf("%s--%s", shoot.Namespace, shoot.Name)
		cpsc     = secrets.ControlPlaneSecretConfig{
			Name: authName,
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName:   userInfo.GetName(),
				Organization: []string{user.SystemPrivilegedGroup},
				CertType:     secrets.ClientCert,
				Validity:     &validity,
				SigningCA:    clientCACertificate,
			},
		}
	)

	for _, address := range shoot.Status.AdvertisedAddresses {
		u, err := url.Parse(address.URL)
		if err != nil {
			return nil, err
		}

		cpsc.KubeConfigRequests = append(cpsc.KubeConfigRequests, secrets.KubeConfigRequest{
			ClusterName:   fmt.Sprintf("%s-%s", authName, address.Name),
			APIServerHost: u.Host,
			CAData:        clusterCABundle,
		})
	}

	cp, err := cpsc.Generate()
	if err != nil {
		return nil, err
	}
	controlPlaneSecret := cp.(*secrets.ControlPlane)

	out.Status.Kubeconfig = controlPlaneSecret.Kubeconfig
	out.Status.ExpirationTimestamp = metav1.Time{Time: controlPlaneSecret.Certificate.Certificate.NotAfter}

	return out, nil
}

// GroupVersionKind returns authentication.gardener.cloud/v1alpha1 for AdminKubeconfigRequest.
func (r *AdminKubeconfigREST) GroupVersionKind(schema.GroupVersion) schema.GroupVersionKind {
	return gvk
}

type getter interface {
	Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error)
}

var (
	managedBySecretsManagerReq = utils.MustNewRequirement(secretsmanager.LabelKeyManagedBy, selection.Equals, secretsmanager.LabelValueSecretsManager)
	identityGardenletReq       = utils.MustNewRequirement(secretsmanager.LabelKeyManagerIdentity, selection.Equals, v1beta1constants.SecretManagerIdentityGardenlet)
	nameCAClientReq            = utils.MustNewRequirement(secretsmanager.LabelKeyName, selection.Equals, v1beta1constants.SecretNameCAClient)
	nameCAClusterReq           = utils.MustNewRequirement(secretsmanager.LabelKeyName, selection.Equals, v1beta1constants.SecretNameCACluster)
	caCertificateSelector      = labels.NewSelector().Add(managedBySecretsManagerReq).Add(identityGardenletReq)
)

func findNewestCACertificate(results []*gardencorev1beta1.GardenerResourceData) (*gardencorev1beta1.GardenerResourceData, error) {
	if len(results) == 1 {
		return results[0], nil
	}

	var (
		newestIssuedAtTime int64
		result             *gardencorev1beta1.GardenerResourceData
	)

	for _, data := range results {
		issuedAtTime, ok := data.Labels[secretsmanager.LabelKeyIssuedAtTime]
		if !ok {
			continue
		}

		issuedAtUnix, err := strconv.ParseInt(issuedAtTime, 10, 64)
		if err != nil {
			return nil, err
		}

		if issuedAtUnix > newestIssuedAtTime {
			newestIssuedAtTime = issuedAtUnix
			result = data.DeepCopy()
		}
	}

	return result, nil
}

func getClusterCABundle(resourceDataList v1beta1helper.GardenerResourceDataList) ([]byte, error) {
	var (
		allCAs   = resourceDataList.Select(caCertificateSelector.Add(nameCAClusterReq))
		caBundle []byte
		err      error
	)

	// Sort CAs descending based on their issued-at-time label. This ensures that the current CA is always the first so
	// that the order in the bundle is the same as in the <shoot-name>.ca-cluster secret.
	sort.Slice(allCAs, func(i, j int) bool {
		issuedAtUnix1, err1 := parseIssuedAtUnix(allCAs[i])
		if err1 != nil {
			err = err1
		}

		issuedAtUnix2, err2 := parseIssuedAtUnix(allCAs[j])
		if err2 != nil {
			err = err2
		}

		return issuedAtUnix1 > issuedAtUnix2
	})
	if err != nil {
		return nil, err
	}

	for _, data := range allCAs {
		cert, _, err := getCADataRaw(data)
		if err != nil {
			return nil, fmt.Errorf("could not fetch raw CA data for %q", data.Name)
		}
		caBundle = append(caBundle, cert...)
	}

	if len(caBundle) == 0 {
		return nil, fmt.Errorf("cluster certificate authority not yet provisioned")
	}

	return caBundle, nil
}

func getClientCACertificate(resourceDataList v1beta1helper.GardenerResourceDataList) (*secrets.Certificate, error) {
	var (
		ca  *gardencorev1beta1.GardenerResourceData
		err error
	)

	if caCerts := resourceDataList.Select(caCertificateSelector.Add(nameCAClientReq)); len(caCerts) > 0 {
		ca, err = findNewestCACertificate(caCerts)
		if err != nil {
			return nil, fmt.Errorf("could not find newest CA certificate for %s: %w", nameCAClientReq, err)
		}
	}

	if ca == nil {
		return nil, fmt.Errorf("client certificate authority not yet provisioned")
	}

	cert, key, err := getCADataRaw(ca)
	if err != nil {
		return nil, fmt.Errorf("could not fetch raw client CA data for %q", ca.Name)
	}

	return secrets.LoadCertificate("", key, cert)
}

func getCADataRaw(resourceData *gardencorev1beta1.GardenerResourceData) (certificate []byte, privateKey []byte, err error) {
	data := make(map[string][]byte)
	if err = json.Unmarshal(resourceData.Data.Raw, &data); err != nil {
		return nil, nil, err
	}

	certificate, privateKey = data[secrets.DataKeyCertificateCA], data[secrets.DataKeyPrivateKeyCA]
	return
}

func parseIssuedAtUnix(caSecret *gardencorev1beta1.GardenerResourceData) (int64, error) {
	issuedAtTime, ok1 := caSecret.Labels[secretsmanager.LabelKeyIssuedAtTime]
	if !ok1 {
		return -1, fmt.Errorf("ca %q does not have %s label", caSecret.Name, secretsmanager.LabelKeyIssuedAtTime)
	}

	issuedAtUnix, err := strconv.ParseInt(issuedAtTime, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("could not parse %s label for ca %q: %w", secretsmanager.LabelKeyIssuedAtTime, caSecret.Name, err)
	}

	return issuedAtUnix, nil
}
