// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operator_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Extensions", func() {
	Describe("#Extensions", func() {
		It("should return a map of well-known extensions", func() {
			Expect(Extensions()["provider-local"].Deployment).NotTo(BeNil())
		})
	})

	Describe("#ExtensionSpecFor", func() {
		It("should return an extension spec when the name is known", func() {
			spec, ok := ExtensionSpecFor("provider-local")
			Expect(ok).To(BeTrue())
			Expect(spec.Deployment).NotTo(BeNil())
		})

		It("should return an empty extension spec when the name is known", func() {
			spec, ok := ExtensionSpecFor("some-foo-unknown-extension")
			Expect(ok).To(BeFalse())
			Expect(spec).To(Equal(operatorv1alpha1.ExtensionSpec{}))
		})
	})

	Describe("#MergeExtensionSpecs", func() {
		It("should correctly merge the extension spec", func() {
			spec := operatorv1alpha1.ExtensionSpec{
				Deployment: &operatorv1alpha1.Deployment{
					Admission: &operatorv1alpha1.DeploymentSpec{Helm: &operatorv1alpha1.Helm{}},
					Extension: &operatorv1alpha1.ExtensionDeploymentSpec{
						Policy:      ptr.To(gardencorev1beta1.ControllerDeploymentPolicyAlwaysExceptNoShoots),
						Annotations: map[string]string{"another": "annotation"},
						DeploymentSpec: operatorv1alpha1.DeploymentSpec{
							Helm: &operatorv1alpha1.Helm{
								Values: &runtime.RawExtension{Raw: []byte(`{
  "foo": "bar",
  "image": "overwritten"
}`)},
							},
						},
					},
				},
			}

			result, err := MergeExtensionSpecs("provider-local", spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Resources).To(Equal([]gardencorev1beta1.ControllerResource{
				{Kind: "BackupBucket", Type: "local"},
				{Kind: "BackupEntry", Type: "local"},
				{Kind: "DNSRecord", Type: "local"},
				{Kind: "ControlPlane", Type: "local"},
				{Kind: "Infrastructure", Type: "local"},
				{Kind: "OperatingSystemConfig", Type: "local"},
				{Kind: "Worker", Type: "local"},
				{Kind: "Extension", Type: "local-ext-seed", WorkerlessSupported: ptr.To(true), Lifecycle: &gardencorev1beta1.ControllerResourceLifecycle{Reconcile: ptr.To(gardencorev1beta1.BeforeKubeAPIServer), Delete: ptr.To(gardencorev1beta1.AfterKubeAPIServer), Migrate: ptr.To(gardencorev1beta1.AfterKubeAPIServer)}},
				{Kind: "Extension", Type: "local-ext-shoot", WorkerlessSupported: ptr.To(true)},
				{Kind: "Extension", Type: "local-ext-shoot-after-worker", Lifecycle: &gardencorev1beta1.ControllerResourceLifecycle{Reconcile: ptr.To(gardencorev1beta1.AfterWorker)}},
			}))
			Expect(result.Deployment).To(DeepDerivativeEqual(&operatorv1alpha1.Deployment{
				Admission: &operatorv1alpha1.DeploymentSpec{Helm: &operatorv1alpha1.Helm{}},
				Extension: &operatorv1alpha1.ExtensionDeploymentSpec{
					Policy: ptr.To(gardencorev1beta1.ControllerDeploymentPolicyAlwaysExceptNoShoots),
					Annotations: map[string]string{
						"another": "annotation",
						"security.gardener.cloud/pod-security-enforce": "privileged",
					},
					DeploymentSpec: operatorv1alpha1.DeploymentSpec{
						Helm: &operatorv1alpha1.Helm{
							Values: &runtime.RawExtension{Raw: []byte(`{"foo":"bar","image":"overwritten","replicaCount":1}`)},
						},
					},
				},
			}))
		})
	})
})
