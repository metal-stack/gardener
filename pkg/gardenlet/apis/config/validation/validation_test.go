// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	. "github.com/gardener/gardener/pkg/gardenlet/apis/config/validation"
)

var _ = Describe("GardenletConfiguration", func() {
	var (
		cfg *config.GardenletConfiguration

		deletionGracePeriodHours = 1
		concurrentSyncs          = 20
	)

	BeforeEach(func() {
		cfg = &config.GardenletConfiguration{
			Controllers: &config.GardenletControllerConfiguration{
				BackupEntry: &config.BackupEntryControllerConfiguration{
					DeletionGracePeriodHours:         &deletionGracePeriodHours,
					DeletionGracePeriodShootPurposes: []gardencore.ShootPurpose{gardencore.ShootPurposeDevelopment},
				},
				Bastion: &config.BastionControllerConfiguration{
					ConcurrentSyncs: &concurrentSyncs,
				},
				Shoot: &config.ShootControllerConfiguration{
					ConcurrentSyncs:      &concurrentSyncs,
					ProgressReportPeriod: &metav1.Duration{Duration: time.Hour},
					SyncPeriod:           &metav1.Duration{Duration: time.Hour},
					RetryDuration:        &metav1.Duration{Duration: time.Hour},
					DNSEntryTTLSeconds:   pointer.Int64(120),
				},
				ShootCare: &config.ShootCareControllerConfiguration{
					ConcurrentSyncs:                     &concurrentSyncs,
					SyncPeriod:                          &metav1.Duration{Duration: time.Hour},
					StaleExtensionHealthChecks:          &config.StaleExtensionHealthChecks{Threshold: &metav1.Duration{Duration: time.Hour}},
					ManagedResourceProgressingThreshold: &metav1.Duration{Duration: time.Hour},
					ConditionThresholds:                 []config.ConditionThreshold{{Duration: metav1.Duration{Duration: time.Hour}}},
				},
				ManagedSeed: &config.ManagedSeedControllerConfiguration{
					ConcurrentSyncs:  &concurrentSyncs,
					SyncPeriod:       &metav1.Duration{Duration: 1 * time.Hour},
					WaitSyncPeriod:   &metav1.Duration{Duration: 15 * time.Second},
					SyncJitterPeriod: &metav1.Duration{Duration: 5 * time.Minute},
				},
			},
			FeatureGates: map[string]bool{},
			SeedConfig: &config.SeedConfig{
				SeedTemplate: gardencore.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"foo": "bar",
						},
					},
					Spec: gardencore.SeedSpec{
						DNS: gardencore.SeedDNS{
							Provider: &gardencore.SeedDNSProvider{
								Type: "foo",
								SecretRef: corev1.SecretReference{
									Name:      "secret",
									Namespace: "namespace",
								},
							},
						},
						Ingress: &gardencore.Ingress{
							Domain: "ingress.test.example.com",
							Controller: gardencore.IngressController{
								Kind: "nginx",
							},
						},
						Networks: gardencore.SeedNetworks{
							Pods:     "100.96.0.0/11",
							Services: "100.64.0.0/13",
						},
						Provider: gardencore.SeedProvider{
							Type:   "foo",
							Region: "some-region",
						},
					},
				},
			},
			Resources: &config.ResourcesConfiguration{
				Capacity: corev1.ResourceList{
					"foo": resource.MustParse("42"),
					"bar": resource.MustParse("13"),
				},
				Reserved: corev1.ResourceList{
					"foo": resource.MustParse("7"),
				},
			},
		}
	})

	Describe("#ValidateGardenletConfiguration", func() {
		It("should allow valid configurations", func() {
			errorList := ValidateGardenletConfiguration(cfg, nil, false)

			Expect(errorList).To(BeEmpty())
		})

		Context("garden client connection", func() {
			Context("kubeconfig validity", func() {
				It("should allow when config is not set", func() {
					Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(BeEmpty())
				})

				It("should allow valid configurations", func() {
					cfg.GardenClientConnection = &config.GardenClientConnection{
						KubeconfigValidity: &config.KubeconfigValidity{
							Validity:                        &metav1.Duration{Duration: time.Hour},
							AutoRotationJitterPercentageMin: pointer.Int32(13),
							AutoRotationJitterPercentageMax: pointer.Int32(37),
						},
					}

					Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(BeEmpty())
				})

				It("should forbid validity less than 10m", func() {
					cfg.GardenClientConnection = &config.GardenClientConnection{
						KubeconfigValidity: &config.KubeconfigValidity{
							Validity: &metav1.Duration{Duration: time.Second},
						},
					}

					Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("gardenClientConnection.kubeconfigValidity.validity"),
						"Detail": ContainSubstring("must be at least 10m"),
					}))))
				})

				It("should forbid auto rotation jitter percentage min less than 1", func() {
					cfg.GardenClientConnection = &config.GardenClientConnection{
						KubeconfigValidity: &config.KubeconfigValidity{
							AutoRotationJitterPercentageMin: pointer.Int32(0),
						},
					}

					Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("gardenClientConnection.kubeconfigValidity.autoRotationJitterPercentageMin"),
						"Detail": ContainSubstring("must be at least 1"),
					}))))
				})

				It("should forbid auto rotation jitter percentage max more than 100", func() {
					cfg.GardenClientConnection = &config.GardenClientConnection{
						KubeconfigValidity: &config.KubeconfigValidity{
							AutoRotationJitterPercentageMax: pointer.Int32(101),
						},
					}

					Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("gardenClientConnection.kubeconfigValidity.autoRotationJitterPercentageMax"),
						"Detail": ContainSubstring("must be at most 100"),
					}))))
				})

				It("should forbid auto rotation jitter percentage min equal max", func() {
					cfg.GardenClientConnection = &config.GardenClientConnection{
						KubeconfigValidity: &config.KubeconfigValidity{
							AutoRotationJitterPercentageMin: pointer.Int32(13),
							AutoRotationJitterPercentageMax: pointer.Int32(13),
						},
					}

					Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("gardenClientConnection.kubeconfigValidity.autoRotationJitterPercentageMin"),
						"Detail": ContainSubstring("minimum percentage must be less than maximum percentage"),
					}))))
				})

				It("should forbid auto rotation jitter percentage min higher than max", func() {
					cfg.GardenClientConnection = &config.GardenClientConnection{
						KubeconfigValidity: &config.KubeconfigValidity{
							AutoRotationJitterPercentageMin: pointer.Int32(14),
							AutoRotationJitterPercentageMax: pointer.Int32(13),
						},
					}

					Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("gardenClientConnection.kubeconfigValidity.autoRotationJitterPercentageMin"),
						"Detail": ContainSubstring("minimum percentage must be less than maximum percentage"),
					}))))
				})
			})
		})

		Context("shoot controller", func() {
			It("should forbid invalid configuration", func() {
				invalidConcurrentSyncs := -1

				cfg.Controllers.Shoot.ConcurrentSyncs = &invalidConcurrentSyncs
				cfg.Controllers.Shoot.ProgressReportPeriod = &metav1.Duration{Duration: -1}
				cfg.Controllers.Shoot.SyncPeriod = &metav1.Duration{Duration: -1}
				cfg.Controllers.Shoot.RetryDuration = &metav1.Duration{Duration: -1}

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shoot.concurrentSyncs"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shoot.progressReporterPeriod"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shoot.syncPeriod"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shoot.retryDuration"),
					})),
				))
			})

			It("should forbid too low values for the DNS TTL", func() {
				cfg.Controllers.Shoot.DNSEntryTTLSeconds = pointer.Int64(-1)

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllers.shoot.dnsEntryTTLSeconds"),
				}))))
			})

			It("should forbid too high values for the DNS TTL", func() {
				cfg.Controllers.Shoot.DNSEntryTTLSeconds = pointer.Int64(601)

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllers.shoot.dnsEntryTTLSeconds"),
				}))))
			})
		})

		Context("shootCare controller", func() {
			It("should forbid invalid configuration", func() {
				invalidConcurrentSyncs := -1

				cfg.Controllers.ShootCare.ConcurrentSyncs = &invalidConcurrentSyncs
				cfg.Controllers.ShootCare.SyncPeriod = &metav1.Duration{Duration: -1}
				cfg.Controllers.ShootCare.StaleExtensionHealthChecks = &config.StaleExtensionHealthChecks{Threshold: &metav1.Duration{Duration: -1}}
				cfg.Controllers.ShootCare.ManagedResourceProgressingThreshold = &metav1.Duration{Duration: -1}
				cfg.Controllers.ShootCare.ConditionThresholds = []config.ConditionThreshold{{Duration: metav1.Duration{Duration: -1}}}

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shootCare.concurrentSyncs"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shootCare.syncPeriod"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shootCare.staleExtensionHealthChecks.threshold"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shootCare.managedResourceProgressingThreshold"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shootCare.conditionThresholds[0].duration"),
					})),
				))
			})
		})

		Context("managed seed controller", func() {
			It("should forbid invalid configuration", func() {
				invalidConcurrentSyncs := -1

				cfg.Controllers.ManagedSeed.ConcurrentSyncs = &invalidConcurrentSyncs
				cfg.Controllers.ManagedSeed.SyncPeriod = &metav1.Duration{Duration: -1}
				cfg.Controllers.ManagedSeed.WaitSyncPeriod = &metav1.Duration{Duration: -1}
				cfg.Controllers.ManagedSeed.SyncJitterPeriod = &metav1.Duration{Duration: -1}

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.managedSeed.concurrentSyncs"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.managedSeed.syncPeriod"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.managedSeed.waitSyncPeriod"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.managedSeed.syncJitterPeriod"),
					})),
				))
			})
		})

		Context("backup entry controller", func() {
			It("should forbid specifying purposes when not specifying hours", func() {
				cfg.Controllers.BackupEntry.DeletionGracePeriodHours = nil

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("controllers.backupEntry.deletionGracePeriodShootPurposes"),
					})),
				))
			})

			It("should allow valid purposes", func() {
				cfg.Controllers.BackupEntry.DeletionGracePeriodShootPurposes = []gardencore.ShootPurpose{
					gardencore.ShootPurposeEvaluation,
					gardencore.ShootPurposeTesting,
					gardencore.ShootPurposeDevelopment,
					gardencore.ShootPurposeInfrastructure,
					gardencore.ShootPurposeProduction,
				}

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(BeEmpty())
			})

			It("should forbid invalid purposes", func() {
				cfg.Controllers.BackupEntry.DeletionGracePeriodShootPurposes = []gardencore.ShootPurpose{"does-not-exist"}

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("controllers.backupEntry.deletionGracePeriodShootPurposes[0]"),
					})),
				))
			})
		})

		Context("bastion controller", func() {
			It("should forbid invalid configuration", func() {
				invalidConcurrentSyncs := -1
				cfg.Controllers.Bastion.ConcurrentSyncs = &invalidConcurrentSyncs

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.bastion.concurrentSyncs"),
					})),
				))
			})
		})

		Context("network policy controller", func() {
			BeforeEach(func() {
				cfg.Controllers.NetworkPolicy = &config.NetworkPolicyControllerConfiguration{}
			})

			It("should return errors because concurrent syncs are < 0", func() {
				cfg.Controllers.NetworkPolicy.ConcurrentSyncs = pointer.Int(-1)

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.networkPolicy.concurrentSyncs"),
					})),
				))
			})

			It("should return errors because some label selector is invalid", func() {
				cfg.Controllers.NetworkPolicy.AdditionalNamespaceSelectors = append(cfg.Controllers.NetworkPolicy.AdditionalNamespaceSelectors,
					metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
					metav1.LabelSelector{MatchLabels: map[string]string{"foo": "no/slash/allowed"}},
				)

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.networkPolicy.additionalNamespaceSelectors[1].matchLabels"),
					})),
				))
			})
		})

		Context("seed config", func() {
			It("should require a seedConfig", func() {
				cfg.SeedConfig = nil

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("seedConfig"),
				}))))
			})
		})

		Context("seed template", func() {
			It("should forbid invalid fields in seed template", func() {
				cfg.SeedConfig.Spec.Networks.Nodes = pointer.String("")

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("seedConfig.spec.networks.nodes"),
					})),
				))
			})
		})

		Context("resources", func() {
			It("should forbid reserved greater than capacity", func() {
				cfg.Resources = &config.ResourcesConfiguration{
					Capacity: corev1.ResourceList{
						"foo": resource.MustParse("42"),
					},
					Reserved: corev1.ResourceList{
						"foo": resource.MustParse("43"),
					},
				}

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("resources.reserved.foo"),
				}))))
			})

			It("should forbid reserved without capacity", func() {
				cfg.Resources = &config.ResourcesConfiguration{
					Reserved: corev1.ResourceList{
						"foo": resource.MustParse("42"),
					},
				}

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("resources.reserved.foo"),
				}))))
			})
		})

		Context("sni", func() {
			BeforeEach(func() {
				cfg.SNI = &config.SNI{Ingress: &config.SNIIngress{}}
			})

			It("should pass as sni config contains a valid external service ip", func() {
				cfg.SNI.Ingress.ServiceExternalIP = pointer.String("1.1.1.1")

				errorList := ValidateGardenletConfiguration(cfg, nil, false)
				Expect(errorList).To(BeEmpty())
			})

			It("should forbid as sni config contains an empty external service ip", func() {
				cfg.SNI.Ingress.ServiceExternalIP = pointer.String("")

				errorList := ValidateGardenletConfiguration(cfg, nil, false)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("sni.ingress.serviceExternalIP"),
				}))))
			})

			It("should forbid as sni config contains an invalid external service ip", func() {
				cfg.SNI.Ingress.ServiceExternalIP = pointer.String("a.b.c.d")

				errorList := ValidateGardenletConfiguration(cfg, nil, false)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("sni.ingress.serviceExternalIP"),
				}))))
			})
		})

		Context("exposureClassHandlers", func() {
			BeforeEach(func() {
				cfg.ExposureClassHandlers = []config.ExposureClassHandler{
					{
						Name: "test",
						LoadBalancerService: config.LoadBalancerServiceConfig{
							Annotations: map[string]string{"test": "foo"},
						},
						SNI: &config.SNI{Ingress: &config.SNIIngress{}},
					},
				}
			})

			It("should pass valid exposureClassHandler", func() {
				errorList := ValidateGardenletConfiguration(cfg, nil, false)
				Expect(errorList).To(BeEmpty())
			})

			It("should fail as exposureClassHandler name is no DNS1123 label with zero length", func() {
				cfg.ExposureClassHandlers[0].Name = ""

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("exposureClassHandlers[0].name"),
				}))))
			})

			It("should fail as exposureClassHandler name is no DNS1123 label", func() {
				cfg.ExposureClassHandlers[0].Name = "TE:ST"

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("exposureClassHandlers[0].name"),
				}))))
			})

			Context("serviceExternalIP", func() {
				It("should allow to use an external service ip as loadbalancer ip is valid", func() {
					cfg.ExposureClassHandlers[0].SNI.Ingress.ServiceExternalIP = pointer.String("1.1.1.1")

					errorList := ValidateGardenletConfiguration(cfg, nil, false)

					Expect(errorList).To(BeEmpty())
				})

				It("should allow to use an external service ip", func() {
					cfg.ExposureClassHandlers[0].SNI.Ingress.ServiceExternalIP = pointer.String("1.1.1.1")

					errorList := ValidateGardenletConfiguration(cfg, nil, false)

					Expect(errorList).To(BeEmpty())
				})

				It("should forbid to use an empty external service ip", func() {
					cfg.ExposureClassHandlers[0].SNI.Ingress.ServiceExternalIP = pointer.String("")

					errorList := ValidateGardenletConfiguration(cfg, nil, false)
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("exposureClassHandlers[0].sni.ingress.serviceExternalIP"),
					}))))
				})

				It("should forbid to use an invalid external service ip", func() {
					cfg.ExposureClassHandlers[0].SNI.Ingress.ServiceExternalIP = pointer.String("a.b.c.d")

					errorList := ValidateGardenletConfiguration(cfg, nil, false)
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("exposureClassHandlers[0].sni.ingress.serviceExternalIP"),
					}))))
				})
			})
		})

		Context("nodeToleration", func() {
			It("should pass with unset toleration options", func() {
				cfg.NodeToleration = nil

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(BeEmpty())
			})

			It("should pass with unset toleration seconds", func() {
				cfg.NodeToleration = &config.NodeToleration{
					DefaultNotReadyTolerationSeconds:    nil,
					DefaultUnreachableTolerationSeconds: nil,
				}

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(BeEmpty())
			})

			It("should pass with valid toleration options", func() {
				cfg.NodeToleration = &config.NodeToleration{
					DefaultNotReadyTolerationSeconds:    pointer.Int64(60),
					DefaultUnreachableTolerationSeconds: pointer.Int64(120),
				}

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(BeEmpty())
			})

			It("should fail with invalid toleration options", func() {
				cfg.NodeToleration = &config.NodeToleration{
					DefaultNotReadyTolerationSeconds:    pointer.Int64(-1),
					DefaultUnreachableTolerationSeconds: pointer.Int64(-2),
				}

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("nodeToleration.defaultNotReadyTolerationSeconds"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("nodeToleration.defaultUnreachableTolerationSeconds"),
					}))),
				)
			})
		})
	})

	Describe("#ValidateGardenletConfigurationUpdate", func() {
		It("should allow valid configuration updates", func() {
			errorList := ValidateGardenletConfigurationUpdate(cfg, cfg, nil)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid changes to immutable fields in seed template", func() {
			newCfg := cfg.DeepCopy()
			newCfg.SeedConfig.Spec.Networks.Pods = "100.97.0.0/11"

			errorList := ValidateGardenletConfigurationUpdate(newCfg, cfg, nil)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("seedConfig.spec.networks.pods"),
					"Detail": Equal("field is immutable"),
				})),
			))
		})
	})
})
