// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package care_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/botanist/matchers"
	. "github.com/gardener/gardener/pkg/operation/care"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("WebhookRemediation", func() {
	var (
		ctx = context.TODO()

		fakeClient              client.Client
		fakeKubernetesInterface kubernetes.Interface
		shootClientInit         func() (kubernetes.Interface, bool, error)

		shoot *gardencorev1beta1.Shoot
		op    *operation.Operation

		remediator *WebhookRemediation
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
		fakeKubernetesInterface = kubernetesfake.NewClientSetBuilder().WithClient(fakeClient).Build()
		shootClientInit = func() (kubernetes.Interface, bool, error) {
			return fakeKubernetesInterface, true, nil
		}

		shoot = &gardencorev1beta1.Shoot{}
		op = &operation.Operation{
			Logger: logr.Discard(),
			Shoot:  &shootpkg.Shoot{},
		}
		op.Shoot.SetInfo(shoot)

		remediator = NewWebhookRemediation(op, shootClientInit)
	})

	Describe("#Remediate", func() {
		var (
			ignore = admissionregistrationv1.Ignore
			fail   = admissionregistrationv1.Fail

			validatingWebhookConfiguration *admissionregistrationv1.ValidatingWebhookConfiguration
			mutatingWebhookConfiguration   *admissionregistrationv1.MutatingWebhookConfiguration
		)

		BeforeEach(func() {
			validatingWebhookConfiguration = &admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "validating",
				},
			}
			mutatingWebhookConfiguration = &admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mutating",
				},
			}
		})

		It("should succeed when there are no webhooks", func() {
			Expect(remediator.Remediate(ctx)).To(Succeed())
		})

		It("should succeed when there are only excluded webhooks", func() {
			validatingWebhookConfiguration.Webhooks = []admissionregistrationv1.ValidatingWebhook{{
				Name:           "some-webhook.example.com",
				TimeoutSeconds: pointer.Int32(30),
			}}
			mutatingWebhookConfiguration.Webhooks = []admissionregistrationv1.MutatingWebhook{{
				Name:           "some-webhook.example.com",
				TimeoutSeconds: pointer.Int32(30),
			}}

			metav1.SetMetaDataLabel(&validatingWebhookConfiguration.ObjectMeta, "remediation.webhook.shoot.gardener.cloud/exclude", "true")
			metav1.SetMetaDataLabel(&mutatingWebhookConfiguration.ObjectMeta, "remediation.webhook.shoot.gardener.cloud/exclude", "true")

			Expect(fakeClient.Create(ctx, validatingWebhookConfiguration)).To(Succeed())
			Expect(fakeClient.Create(ctx, mutatingWebhookConfiguration)).To(Succeed())

			Expect(remediator.Remediate(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(validatingWebhookConfiguration), validatingWebhookConfiguration)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mutatingWebhookConfiguration), mutatingWebhookConfiguration)).To(Succeed())

			Expect(validatingWebhookConfiguration.Annotations).NotTo(HaveKey("gardener.cloud/warning"))
			Expect(mutatingWebhookConfiguration.Annotations).NotTo(HaveKey("gardener.cloud/warning"))
		})

		It("should succeed when there are only Gardener-managed webhooks", func() {
			validatingWebhookConfiguration.Webhooks = []admissionregistrationv1.ValidatingWebhook{{
				Name:           "some-webhook.example.com",
				TimeoutSeconds: pointer.Int32(30),
			}}
			mutatingWebhookConfiguration.Webhooks = []admissionregistrationv1.MutatingWebhook{{
				Name:           "some-webhook.example.com",
				TimeoutSeconds: pointer.Int32(30),
			}}

			metav1.SetMetaDataLabel(&validatingWebhookConfiguration.ObjectMeta, "resources.gardener.cloud/managed-by", "gardener")
			metav1.SetMetaDataLabel(&mutatingWebhookConfiguration.ObjectMeta, "resources.gardener.cloud/managed-by", "gardener")

			Expect(fakeClient.Create(ctx, validatingWebhookConfiguration)).To(Succeed())
			Expect(fakeClient.Create(ctx, mutatingWebhookConfiguration)).To(Succeed())

			Expect(remediator.Remediate(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(validatingWebhookConfiguration), validatingWebhookConfiguration)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mutatingWebhookConfiguration), mutatingWebhookConfiguration)).To(Succeed())

			Expect(validatingWebhookConfiguration.Annotations).NotTo(HaveKey("gardener.cloud/warning"))
			Expect(mutatingWebhookConfiguration.Annotations).NotTo(HaveKey("gardener.cloud/warning"))
		})

		Context("remediate offensive webhooks", func() {
			Context("validating", func() {
				It("timeoutSeconds", func() {
					validatingWebhookConfiguration.Webhooks = []admissionregistrationv1.ValidatingWebhook{{
						Name:           "some-webhook.example.com",
						TimeoutSeconds: pointer.Int32(30),
					}}
					Expect(fakeClient.Create(ctx, validatingWebhookConfiguration)).To(Succeed())

					Expect(remediator.Remediate(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(validatingWebhookConfiguration), validatingWebhookConfiguration)).To(Succeed())
					Expect(validatingWebhookConfiguration.Annotations).To(HaveKey("gardener.cloud/warning"))
					Expect(validatingWebhookConfiguration.Webhooks[0].TimeoutSeconds).To(Equal(pointer.Int32(15)))
				})

				It("timeoutSeconds when failurePolicy=Ignore", func() {
					validatingWebhookConfiguration.Webhooks = []admissionregistrationv1.ValidatingWebhook{{
						Name:           "some-webhook.example.com",
						TimeoutSeconds: pointer.Int32(30),
						FailurePolicy:  &ignore,
					}}
					Expect(fakeClient.Create(ctx, validatingWebhookConfiguration)).To(Succeed())

					Expect(remediator.Remediate(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(validatingWebhookConfiguration), validatingWebhookConfiguration)).To(Succeed())
					Expect(validatingWebhookConfiguration.Annotations).To(HaveKey("gardener.cloud/warning"))
					Expect(validatingWebhookConfiguration.Webhooks[0].TimeoutSeconds).To(Equal(pointer.Int32(15)))
				})

				It("failurePolicy", func() {
					defer test.WithVar(&matchers.WebhookConstraintMatchers, []matchers.WebhookConstraintMatcher{
						{GVR: corev1.SchemeGroupVersion.WithResource("foobars")},
						{GVR: corev1.SchemeGroupVersion.WithResource("barfoos"), ObjectLabels: map[string]string{"foo": "bar"}},
					})()

					validatingWebhookConfiguration.Webhooks = []admissionregistrationv1.ValidatingWebhook{{
						Name:          "some-webhook.example.com",
						FailurePolicy: &fail,
						Rules: []admissionregistrationv1.RuleWithOperations{{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"foobars", "barfoos"},
							},
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						}},
					}}
					Expect(fakeClient.Create(ctx, validatingWebhookConfiguration)).To(Succeed())

					Expect(remediator.Remediate(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(validatingWebhookConfiguration), validatingWebhookConfiguration)).To(Succeed())
					Expect(validatingWebhookConfiguration.Annotations).To(HaveKey("gardener.cloud/warning"))
					Expect(validatingWebhookConfiguration.Webhooks[0].FailurePolicy).To(Equal(&ignore))
				})

				It("namespaceSelector", func() {
					validatingWebhookConfiguration.Webhooks = []admissionregistrationv1.ValidatingWebhook{{
						Name:          "some-webhook.example.com",
						FailurePolicy: &fail,
						Rules: []admissionregistrationv1.RuleWithOperations{{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						}},
					}}
					Expect(fakeClient.Create(ctx, validatingWebhookConfiguration)).To(Succeed())

					Expect(remediator.Remediate(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(validatingWebhookConfiguration), validatingWebhookConfiguration)).To(Succeed())
					Expect(validatingWebhookConfiguration.Annotations).To(HaveKey("gardener.cloud/warning"))
					Expect(validatingWebhookConfiguration.Webhooks[0].FailurePolicy).To(Equal(&fail))
					Expect(validatingWebhookConfiguration.Webhooks[0].NamespaceSelector).NotTo(BeNil())
					Expect(validatingWebhookConfiguration.Webhooks[0].NamespaceSelector.MatchExpressions).To(ConsistOf(
						metav1.LabelSelectorRequirement{Key: "shoot.gardener.cloud/no-cleanup", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"true"}},
						metav1.LabelSelectorRequirement{Key: "gardener.cloud/purpose", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"kube-system"}},
						metav1.LabelSelectorRequirement{Key: "kubernetes.io/metadata.name", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"kube-system"}},
					))
				})

				It("objectSelector", func() {
					defer test.WithVar(&matchers.WebhookConstraintMatchers, []matchers.WebhookConstraintMatcher{
						{GVR: corev1.SchemeGroupVersion.WithResource("foobars"), ObjectLabels: map[string]string{"foo": "bar"}},
					})()

					validatingWebhookConfiguration.Webhooks = []admissionregistrationv1.ValidatingWebhook{{
						Name:          "some-webhook.example.com",
						FailurePolicy: &fail,
						Rules: []admissionregistrationv1.RuleWithOperations{{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"foobars"},
							},
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						}},
					}}
					Expect(fakeClient.Create(ctx, validatingWebhookConfiguration)).To(Succeed())

					Expect(remediator.Remediate(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(validatingWebhookConfiguration), validatingWebhookConfiguration)).To(Succeed())
					Expect(validatingWebhookConfiguration.Annotations).To(HaveKey("gardener.cloud/warning"))
					Expect(validatingWebhookConfiguration.Webhooks[0].FailurePolicy).To(Equal(&fail))
					Expect(validatingWebhookConfiguration.Webhooks[0].ObjectSelector).To(Equal(&metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: "foo", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"bar"}},
						},
					}))
				})

				It("both selectors (multiple matching rules)", func() {
					defer test.WithVar(&matchers.WebhookConstraintMatchers, []matchers.WebhookConstraintMatcher{
						{GVR: corev1.SchemeGroupVersion.WithResource("foobars"), ObjectLabels: map[string]string{"foo": "bar"}},
						{GVR: corev1.SchemeGroupVersion.WithResource("barfoos"), NamespaceLabels: map[string]string{"bar": "foo"}},
						{GVR: corev1.SchemeGroupVersion.WithResource("foobazs"), NamespaceLabels: map[string]string{"foo": "baz"}, ObjectLabels: map[string]string{"foo": "baz"}},
						{GVR: corev1.SchemeGroupVersion.WithResource("foobaz2s"), NamespaceLabels: map[string]string{"foo": "baz"}, ObjectLabels: map[string]string{"foo": "baz"}},
					})()

					validatingWebhookConfiguration.Webhooks = []admissionregistrationv1.ValidatingWebhook{{
						Name:          "some-webhook.example.com",
						FailurePolicy: &fail,
						Rules: []admissionregistrationv1.RuleWithOperations{
							{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"foobars", "barfoos", "foobazs"},
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							},
							{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"foobaz2s"},
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							},
						},
					}}
					Expect(fakeClient.Create(ctx, validatingWebhookConfiguration)).To(Succeed())

					Expect(remediator.Remediate(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(validatingWebhookConfiguration), validatingWebhookConfiguration)).To(Succeed())
					Expect(validatingWebhookConfiguration.Annotations).To(HaveKey("gardener.cloud/warning"))
					Expect(validatingWebhookConfiguration.Webhooks[0].FailurePolicy).To(Equal(&fail))
					Expect(validatingWebhookConfiguration.Webhooks[0].ObjectSelector).To(Equal(&metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: "foo", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"bar"}},
							{Key: "foo", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"baz"}},
						},
					}))
					Expect(validatingWebhookConfiguration.Webhooks[0].NamespaceSelector).To(Equal(&metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: "bar", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"foo"}},
							{Key: "foo", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"baz"}},
						},
					}))
				})
			})

			Context("mutating", func() {
				It("timeoutSeconds", func() {
					mutatingWebhookConfiguration.Webhooks = []admissionregistrationv1.MutatingWebhook{{
						Name:           "some-webhook.example.com",
						TimeoutSeconds: pointer.Int32(30),
					}}
					Expect(fakeClient.Create(ctx, mutatingWebhookConfiguration)).To(Succeed())

					Expect(remediator.Remediate(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mutatingWebhookConfiguration), mutatingWebhookConfiguration)).To(Succeed())
					Expect(mutatingWebhookConfiguration.Annotations).To(HaveKey("gardener.cloud/warning"))
					Expect(mutatingWebhookConfiguration.Webhooks[0].TimeoutSeconds).To(Equal(pointer.Int32(15)))
				})

				It("timeoutSeconds when failurePolicy=Ignore", func() {
					mutatingWebhookConfiguration.Webhooks = []admissionregistrationv1.MutatingWebhook{
						{
							Name:           "some-webhook1.example.com",
							TimeoutSeconds: pointer.Int32(30),
							FailurePolicy:  &ignore,
						},
					}
					Expect(fakeClient.Create(ctx, mutatingWebhookConfiguration)).To(Succeed())

					Expect(remediator.Remediate(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mutatingWebhookConfiguration), mutatingWebhookConfiguration)).To(Succeed())
					Expect(mutatingWebhookConfiguration.Annotations).To(HaveKey("gardener.cloud/warning"))
					Expect(mutatingWebhookConfiguration.Webhooks[0].TimeoutSeconds).To(Equal(pointer.Int32(15)))
				})

				It("timeoutSeconds when failurePolicy=Ignore for lease resource", func() {
					mutatingWebhookConfiguration.Webhooks = []admissionregistrationv1.MutatingWebhook{
						{
							Name:           "some-webhook1.example.com",
							TimeoutSeconds: pointer.Int32(10),
							FailurePolicy:  &ignore,
							ObjectSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": "test",
								},
							},
							Rules: []admissionregistrationv1.RuleWithOperations{
								{
									Rule: admissionregistrationv1.Rule{
										APIGroups:   []string{"coordination.k8s.io"},
										APIVersions: []string{"v1", "v1beta1"},
										Resources:   []string{"leases"},
									},
									Operations: []admissionregistrationv1.OperationType{
										"UPDATE",
									},
								},
							},
						},
						{
							Name:           "some-webhook2.example.com",
							TimeoutSeconds: pointer.Int32(10),
							FailurePolicy:  &ignore,
							Rules: []admissionregistrationv1.RuleWithOperations{
								{
									Rule: admissionregistrationv1.Rule{
										APIGroups:   []string{"coordination.k8s.io"},
										APIVersions: []string{"v1", "v1beta1"},
										Resources:   []string{"leases"},
									},
									Operations: []admissionregistrationv1.OperationType{
										"UPDATE",
									},
								},
							},
						},
						{
							Name:           "some-webhook3.example.com",
							TimeoutSeconds: pointer.Int32(1),
							FailurePolicy:  &ignore,
							Rules: []admissionregistrationv1.RuleWithOperations{
								{
									Rule: admissionregistrationv1.Rule{
										APIGroups:   []string{"coordination.k8s.io"},
										APIVersions: []string{"v1", "v1beta1"},
										Resources:   []string{"leases"},
									},
									Operations: []admissionregistrationv1.OperationType{
										"UPDATE",
									},
								},
							},
						},
					}
					Expect(fakeClient.Create(ctx, mutatingWebhookConfiguration)).To(Succeed())

					Expect(remediator.Remediate(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mutatingWebhookConfiguration), mutatingWebhookConfiguration)).To(Succeed())
					Expect(mutatingWebhookConfiguration.Annotations).To(HaveKey("gardener.cloud/warning"))
					Expect(mutatingWebhookConfiguration.Webhooks[0].TimeoutSeconds).To(Equal(pointer.Int32(10)))
					Expect(mutatingWebhookConfiguration.Webhooks[1].TimeoutSeconds).To(Equal(pointer.Int32(3)))
					Expect(mutatingWebhookConfiguration.Webhooks[2].TimeoutSeconds).To(Equal(pointer.Int32(1)))
				})

				It("failurePolicy", func() {
					defer test.WithVar(&matchers.WebhookConstraintMatchers, []matchers.WebhookConstraintMatcher{
						{GVR: corev1.SchemeGroupVersion.WithResource("foobars")},
						{GVR: corev1.SchemeGroupVersion.WithResource("barfoos"), ObjectLabels: map[string]string{"foo": "bar"}},
					})()

					mutatingWebhookConfiguration.Webhooks = []admissionregistrationv1.MutatingWebhook{{
						Name:          "some-webhook.example.com",
						FailurePolicy: &fail,
						Rules: []admissionregistrationv1.RuleWithOperations{{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"foobars", "barfoos"},
							},
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						}},
					}}
					Expect(fakeClient.Create(ctx, mutatingWebhookConfiguration)).To(Succeed())

					Expect(remediator.Remediate(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mutatingWebhookConfiguration), mutatingWebhookConfiguration)).To(Succeed())
					Expect(mutatingWebhookConfiguration.Annotations).To(HaveKey("gardener.cloud/warning"))
					Expect(mutatingWebhookConfiguration.Webhooks[0].FailurePolicy).To(Equal(&ignore))
				})

				It("namespaceSelector", func() {
					mutatingWebhookConfiguration.Webhooks = []admissionregistrationv1.MutatingWebhook{{
						Name:          "some-webhook.example.com",
						FailurePolicy: &fail,
						Rules: []admissionregistrationv1.RuleWithOperations{{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						}},
					}}
					Expect(fakeClient.Create(ctx, mutatingWebhookConfiguration)).To(Succeed())

					Expect(remediator.Remediate(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mutatingWebhookConfiguration), mutatingWebhookConfiguration)).To(Succeed())
					Expect(mutatingWebhookConfiguration.Annotations).To(HaveKey("gardener.cloud/warning"))
					Expect(mutatingWebhookConfiguration.Webhooks[0].FailurePolicy).To(Equal(&fail))
					Expect(mutatingWebhookConfiguration.Webhooks[0].NamespaceSelector).NotTo(BeNil())
					Expect(mutatingWebhookConfiguration.Webhooks[0].NamespaceSelector.MatchExpressions).To(ConsistOf(
						metav1.LabelSelectorRequirement{Key: "shoot.gardener.cloud/no-cleanup", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"true"}},
						metav1.LabelSelectorRequirement{Key: "gardener.cloud/purpose", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"kube-system"}},
						metav1.LabelSelectorRequirement{Key: "kubernetes.io/metadata.name", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"kube-system"}},
					))
				})

				It("objectSelector", func() {
					defer test.WithVar(&matchers.WebhookConstraintMatchers, []matchers.WebhookConstraintMatcher{
						{GVR: corev1.SchemeGroupVersion.WithResource("foobars"), ObjectLabels: map[string]string{"foo": "bar"}},
					})()

					mutatingWebhookConfiguration.Webhooks = []admissionregistrationv1.MutatingWebhook{{
						Name:          "some-webhook.example.com",
						FailurePolicy: &fail,
						Rules: []admissionregistrationv1.RuleWithOperations{{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"foobars"},
							},
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						}},
					}}
					Expect(fakeClient.Create(ctx, mutatingWebhookConfiguration)).To(Succeed())

					Expect(remediator.Remediate(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mutatingWebhookConfiguration), mutatingWebhookConfiguration)).To(Succeed())
					Expect(mutatingWebhookConfiguration.Annotations).To(HaveKey("gardener.cloud/warning"))
					Expect(mutatingWebhookConfiguration.Webhooks[0].FailurePolicy).To(Equal(&fail))
					Expect(mutatingWebhookConfiguration.Webhooks[0].ObjectSelector).To(Equal(&metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: "foo", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"bar"}},
						},
					}))
				})

				It("both selectors (multiple matching rules)", func() {
					defer test.WithVar(&matchers.WebhookConstraintMatchers, []matchers.WebhookConstraintMatcher{
						{GVR: corev1.SchemeGroupVersion.WithResource("foobars"), ObjectLabels: map[string]string{"foo": "bar"}},
						{GVR: corev1.SchemeGroupVersion.WithResource("barfoos"), NamespaceLabels: map[string]string{"bar": "foo"}},
						{GVR: corev1.SchemeGroupVersion.WithResource("foobazs"), NamespaceLabels: map[string]string{"foo": "baz"}, ObjectLabels: map[string]string{"foo": "baz"}},
						{GVR: corev1.SchemeGroupVersion.WithResource("foobaz2s"), NamespaceLabels: map[string]string{"foo": "baz"}, ObjectLabels: map[string]string{"foo": "baz"}},
					})()

					mutatingWebhookConfiguration.Webhooks = []admissionregistrationv1.MutatingWebhook{{
						Name:          "some-webhook.example.com",
						FailurePolicy: &fail,
						Rules: []admissionregistrationv1.RuleWithOperations{
							{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"foobars", "barfoos", "foobazs"},
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							},
							{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"foobaz2s"},
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							},
						},
					}}
					Expect(fakeClient.Create(ctx, mutatingWebhookConfiguration)).To(Succeed())

					Expect(remediator.Remediate(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mutatingWebhookConfiguration), mutatingWebhookConfiguration)).To(Succeed())
					Expect(mutatingWebhookConfiguration.Annotations).To(HaveKey("gardener.cloud/warning"))
					Expect(mutatingWebhookConfiguration.Webhooks[0].FailurePolicy).To(Equal(&fail))
					Expect(mutatingWebhookConfiguration.Webhooks[0].ObjectSelector).To(Equal(&metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: "foo", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"bar"}},
							{Key: "foo", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"baz"}},
						},
					}))
					Expect(mutatingWebhookConfiguration.Webhooks[0].NamespaceSelector).To(Equal(&metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: "bar", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"foo"}},
							{Key: "foo", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"baz"}},
						},
					}))
				})
			})
		})
	})
})
