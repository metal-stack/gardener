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

package botanist_test

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/extensions/dnsrecord"
	mockdnsrecord "github.com/gardener/gardener/pkg/component/extensions/dnsrecord/mock"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/operation/shoot"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

const (
	shootNamespace = "bar"

	externalDomain   = "foo.bar.external.example.com"
	externalProvider = "external-provider"
	externalZone     = "external-zone"

	internalDomain   = "foo.bar.internal.example.com"
	internalProvider = "internal-provider"
	internalZone     = "internal-zone"

	address       = "1.2.3.4"
	ttl     int64 = 300
)

var _ = Describe("dnsrecord", func() {
	var (
		shootName     string
		seedNamespace string
		ctrl          *gomock.Controller

		scheme *runtime.Scheme
		c      client.Client

		externalDNSRecord *mockdnsrecord.MockInterface
		internalDNSRecord *mockdnsrecord.MockInterface

		b *Botanist

		ctx                 = context.TODO()
		now                 = time.Now()
		testErr             = fmt.Errorf("test")
		seedClusterIdentity = "seed1"

		cleanup func()
	)

	BeforeEach(func() {
		shootName = "foo"
		seedNamespace = "shoot--foo--bar"
		ctrl = gomock.NewController(GinkgoT())

		scheme = runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(corev1.AddToScheme(scheme)).NotTo(HaveOccurred())
		c = fake.NewClientBuilder().WithScheme(scheme).Build()

		externalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)
		internalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)

		cleanup = test.WithVar(&dnsrecord.TimeNow, func() time.Time { return now })
	})

	JustBeforeEach(func() {
		b = &Botanist{
			Operation: &operation.Operation{
				Config: &config.GardenletConfiguration{
					Controllers: &config.GardenletControllerConfiguration{
						Shoot: &config.ShootControllerConfiguration{
							DNSEntryTTLSeconds: pointer.Int64(ttl),
						},
					},
				},
				Shoot: &shoot.Shoot{
					SeedNamespace:         seedNamespace,
					ExternalClusterDomain: pointer.String(externalDomain),
					ExternalDomain: &gardenerutils.Domain{
						Domain:   externalDomain,
						Provider: externalProvider,
						Zone:     externalZone,
						SecretData: map[string][]byte{
							"external-foo": []byte("external-bar"),
						},
					},
					InternalClusterDomain: internalDomain,
					Components: &shoot.Components{
						Extensions: &shoot.Extensions{
							ExternalDNSRecord: externalDNSRecord,
							InternalDNSRecord: internalDNSRecord,
						},
					},
				},
				Garden: &garden.Garden{
					InternalDomain: &gardenerutils.Domain{
						Domain:   internalDomain,
						Provider: internalProvider,
						Zone:     internalZone,
						SecretData: map[string][]byte{
							"internal-foo": []byte("internal-bar"),
						},
					},
				},
				Seed:   &seed.Seed{},
				Logger: logr.Discard(),
			},
		}
		b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: shootNamespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				DNS: &gardencorev1beta1.DNS{
					Domain: pointer.String(externalDomain),
				},
			},
		})

		b.Seed.SetInfo(&gardencorev1beta1.Seed{
			Status: gardencorev1beta1.SeedStatus{
				ClusterIdentity: &seedClusterIdentity,
			},
		})

		renderer := chartrenderer.NewWithServerVersion(&version.Info{})
		chartApplier := kubernetes.NewChartApplier(renderer, kubernetes.NewApplier(c, meta.NewDefaultRESTMapper([]schema.GroupVersion{})))
		Expect(chartApplier).NotTo(BeNil(), "should return chart applier")
		b.SeedClientSet = kubernetesfake.NewClientSetBuilder().
			WithClient(c).
			WithChartApplier(chartApplier).
			Build()
	})

	AfterEach(func() {
		cleanup()
		ctrl.Finish()
	})

	Describe("#DefaultExternalDNSRecord", func() {
		It("should create a component with correct values", func() {
			r := b.DefaultExternalDNSRecord()
			r.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			r.SetValues([]string{address})

			actual := r.GetValues()
			Expect(actual).To(DeepEqual(&dnsrecord.Values{
				Name:       b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordExternalName,
				SecretName: DNSRecordSecretPrefix + "-" + b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordExternalName,
				Namespace:  seedNamespace,
				TTL:        pointer.Int64(ttl),
				Type:       externalProvider,
				Zone:       pointer.String(externalZone),
				SecretData: map[string][]byte{
					"external-foo": []byte("external-bar"),
				},
				DNSName:           "api." + externalDomain,
				RecordType:        extensionsv1alpha1.DNSRecordTypeA,
				Values:            []string{address},
				AnnotateOperation: false,
			}))
		})

		DescribeTable("should set AnnotateOperation value to true",
			func(mutateShootFn func()) {
				mutateShootFn()

				c := b.DefaultExternalDNSRecord()

				Expect(c.GetValues().AnnotateOperation).To(BeTrue())
			},

			Entry("task annotation present", func() {
				shoot := b.Shoot.GetInfo()
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "shoot.gardener.cloud/tasks", "deployDNSRecordExternal")
				b.Shoot.SetInfo(shoot)
			}),
			Entry("restore phase", func() {
				shoot := b.Shoot.GetInfo()
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeRestore}
				b.Shoot.SetInfo(shoot)
			}),
		)

		It("should create a component that creates the DNSRecord and its secret on Deploy", func() {
			shoot := b.Shoot.GetInfo()
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "shoot.gardener.cloud/tasks", "deployDNSRecordExternal")
			b.Shoot.SetInfo(shoot)

			r := b.DefaultExternalDNSRecord()
			r.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			r.SetValues([]string{address})

			Expect(r.Deploy(ctx)).ToNot(HaveOccurred())

			dnsRecord := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, types.NamespacedName{Name: shootName + "-" + v1beta1constants.DNSRecordExternalName, Namespace: seedNamespace}, dnsRecord)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsRecord).To(DeepDerivativeEqual(&extensionsv1alpha1.DNSRecord{
				TypeMeta: metav1.TypeMeta{
					Kind:       "DNSRecord",
					APIVersion: "extensions.gardener.cloud/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            shootName + "-" + v1beta1constants.DNSRecordExternalName,
					Namespace:       seedNamespace,
					ResourceVersion: "1",
					Annotations: map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
					},
				},
				Spec: extensionsv1alpha1.DNSRecordSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						Type: externalProvider,
					},
					SecretRef: corev1.SecretReference{
						Name:      DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordExternalName,
						Namespace: seedNamespace,
					},
					Zone:       pointer.String(externalZone),
					Name:       "api." + externalDomain,
					RecordType: extensionsv1alpha1.DNSRecordTypeA,
					Values:     []string{address},
					TTL:        pointer.Int64(ttl),
				},
			}))

			secret := &corev1.Secret{}
			err = c.Get(ctx, types.NamespacedName{Name: DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordExternalName, Namespace: seedNamespace}, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(secret).To(DeepDerivativeEqual(&corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordExternalName,
					Namespace:       seedNamespace,
					ResourceVersion: "1",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"external-foo": []byte("external-bar"),
				},
			}))
		})
	})

	Describe("#DefaultInternalDNSRecord", func() {
		It("should create a component with correct values", func() {
			c := b.DefaultInternalDNSRecord()
			c.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			c.SetValues([]string{address})

			actual := c.GetValues()
			Expect(actual).To(DeepEqual(&dnsrecord.Values{
				Name:       b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordInternalName,
				SecretName: DNSRecordSecretPrefix + "-" + b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordInternalName,
				Namespace:  seedNamespace,
				TTL:        pointer.Int64(ttl),
				Type:       internalProvider,
				Zone:       pointer.String(internalZone),
				SecretData: map[string][]byte{
					"internal-foo": []byte("internal-bar"),
				},
				DNSName:           "api." + internalDomain,
				RecordType:        extensionsv1alpha1.DNSRecordTypeA,
				Values:            []string{address},
				AnnotateOperation: false,
			}))
		})

		DescribeTable("should set AnnotateOperation value to true",
			func(mutateShootFn func()) {
				mutateShootFn()

				c := b.DefaultInternalDNSRecord()

				Expect(c.GetValues().AnnotateOperation).To(BeTrue())
			},

			Entry("shoot deletion", func() {
				shoot := b.Shoot.GetInfo()
				shoot.DeletionTimestamp = &metav1.Time{}
				b.Shoot.SetInfo(shoot)
			}),
			Entry("task annotation present", func() {
				shoot := b.Shoot.GetInfo()
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "shoot.gardener.cloud/tasks", "deployDNSRecordInternal")
				b.Shoot.SetInfo(shoot)
			}),
			Entry("restore phase", func() {
				shoot := b.Shoot.GetInfo()
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeRestore}
				b.Shoot.SetInfo(shoot)
			}),
		)

		It("should create a component that creates the DNSRecord and its secret on Deploy", func() {
			shoot := b.Shoot.GetInfo()
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "shoot.gardener.cloud/tasks", "deployDNSRecordInternal")
			b.Shoot.SetInfo(shoot)

			r := b.DefaultInternalDNSRecord()
			r.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			r.SetValues([]string{address})

			Expect(r.Deploy(ctx)).ToNot(HaveOccurred())

			dnsRecord := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, types.NamespacedName{Name: shootName + "-" + v1beta1constants.DNSRecordInternalName, Namespace: seedNamespace}, dnsRecord)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsRecord).To(DeepDerivativeEqual(&extensionsv1alpha1.DNSRecord{
				TypeMeta: metav1.TypeMeta{
					Kind:       "DNSRecord",
					APIVersion: "extensions.gardener.cloud/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            shootName + "-" + v1beta1constants.DNSRecordInternalName,
					Namespace:       seedNamespace,
					ResourceVersion: "1",
					Annotations: map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
					},
				},
				Spec: extensionsv1alpha1.DNSRecordSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						Type: internalProvider,
					},
					SecretRef: corev1.SecretReference{
						Name:      DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordInternalName,
						Namespace: seedNamespace,
					},
					Zone:       pointer.String(internalZone),
					Name:       "api." + internalDomain,
					RecordType: extensionsv1alpha1.DNSRecordTypeA,
					Values:     []string{address},
					TTL:        pointer.Int64(ttl),
				},
			}))

			secret := &corev1.Secret{}
			err = c.Get(ctx, types.NamespacedName{Name: DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordInternalName, Namespace: seedNamespace}, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(secret).To(DeepDerivativeEqual(&corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordInternalName,
					Namespace:       seedNamespace,
					ResourceVersion: "1",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"internal-foo": []byte("internal-bar"),
				},
			}))
		})
	})

	Describe("#DeployOrDestroyExternalDNSRecord", func() {
		Context("deploy", func() {
			It("should call Deploy and Wait and succeed if they succeeded", func() {
				externalDNSRecord.EXPECT().Deploy(ctx)
				externalDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Deploy and fail if it failed", func() {
				externalDNSRecord.EXPECT().Deploy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})

		Context("restore", func() {
			var shootState = &gardencorev1beta1.ShootState{}

			JustBeforeEach(func() {
				b.Shoot.SetShootState(shootState)
				b.Shoot.GetInfo().Status = gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						Type: gardencorev1beta1.LastOperationTypeRestore,
					},
				}
			})

			It("should call Restore and Wait and succeed if they succeeded", func() {
				externalDNSRecord.EXPECT().Restore(ctx, shootState)
				externalDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Restore and fail if it failed", func() {
				externalDNSRecord.EXPECT().Restore(ctx, shootState).Return(testErr)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})

		Context("destroy (Shoot DNS is set to nil)", func() {
			JustBeforeEach(func() {
				b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						DNS: nil,
					},
				})
			})

			It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
				externalDNSRecord.EXPECT().Destroy(ctx)
				externalDNSRecord.EXPECT().WaitCleanup(ctx)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Destroy and fail if it failed", func() {
				externalDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})
	})

	Describe("#DeployOrDestroyInternalDNSRecord", func() {
		Context("deploy", func() {
			It("should call Deploy and Wait and succeed if they succeeded", func() {
				internalDNSRecord.EXPECT().Deploy(ctx)
				internalDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Deploy and fail if it failed", func() {
				internalDNSRecord.EXPECT().Deploy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})

		Context("restore", func() {
			var shootState = &gardencorev1beta1.ShootState{}

			JustBeforeEach(func() {
				b.Shoot.SetShootState(shootState)
				b.Shoot.GetInfo().Status = gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						Type: gardencorev1beta1.LastOperationTypeRestore,
					},
				}
			})

			It("should call Restore and Wait and succeed if they succeeded", func() {
				internalDNSRecord.EXPECT().Restore(ctx, shootState)
				internalDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Restore and fail if it failed", func() {
				internalDNSRecord.EXPECT().Restore(ctx, shootState).Return(testErr)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})

		Context("destroy (Garden InternalDomain is set to nil)", func() {
			JustBeforeEach(func() {
				b.Garden.InternalDomain = nil
			})

			It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
				internalDNSRecord.EXPECT().Destroy(ctx)
				internalDNSRecord.EXPECT().WaitCleanup(ctx)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Destroy and fail if it failed", func() {
				internalDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})
	})

	Describe("#DestroyExternalDNSRecord", func() {
		It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
			externalDNSRecord.EXPECT().Destroy(ctx)
			externalDNSRecord.EXPECT().WaitCleanup(ctx)
			Expect(b.DestroyExternalDNSRecord(ctx)).To(Succeed())
		})

		It("should call Destroy and fail if it failed", func() {
			externalDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
			Expect(b.DestroyExternalDNSRecord(ctx)).To(MatchError(testErr))
		})
	})

	Describe("#DestroyInternalDNSRecord", func() {
		It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
			internalDNSRecord.EXPECT().Destroy(ctx)
			internalDNSRecord.EXPECT().WaitCleanup(ctx)
			Expect(b.DestroyInternalDNSRecord(ctx)).To(Succeed())
		})

		It("should call Destroy and fail if it failed", func() {
			internalDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
			Expect(b.DestroyInternalDNSRecord(ctx)).To(MatchError(testErr))
		})
	})

	Describe("#MigrateExternalDNSRecord", func() {
		It("should call Migrate and WaitMigrate and succeed if they succeeded", func() {
			externalDNSRecord.EXPECT().Migrate(ctx)
			externalDNSRecord.EXPECT().WaitMigrate(ctx)
			Expect(b.MigrateExternalDNSRecord(ctx)).To(Succeed())
		})

		It("should call Migrate and fail if it failed", func() {
			externalDNSRecord.EXPECT().Migrate(ctx).Return(testErr)
			Expect(b.MigrateExternalDNSRecord(ctx)).To(MatchError(testErr))
		})
	})

	Describe("#MigrateInternalDNSRecord", func() {
		It("should call Migrate and WaitMigrate and succeed if they succeeded", func() {
			internalDNSRecord.EXPECT().Migrate(ctx)
			internalDNSRecord.EXPECT().WaitMigrate(ctx)
			Expect(b.MigrateInternalDNSRecord(ctx)).To(Succeed())
		})

		It("should call Migrate and fail if it failed", func() {
			internalDNSRecord.EXPECT().Migrate(ctx).Return(testErr)
			Expect(b.MigrateInternalDNSRecord(ctx)).To(MatchError(testErr))
		})
	})
})
