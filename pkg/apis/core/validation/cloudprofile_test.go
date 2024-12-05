// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
)

var (
	metadata = metav1.ObjectMeta{
		Name: "profile",
	}
	machineType = core.MachineType{
		Name:         "machine-type-1",
		CPU:          resource.MustParse("2"),
		GPU:          resource.MustParse("0"),
		Memory:       resource.MustParse("100Gi"),
		Architecture: ptr.To("amd64"),
	}
	machineTypesConstraint = []core.MachineType{
		machineType,
	}
	volumeTypesConstraint = []core.VolumeType{
		{
			Name:  "volume-type-1",
			Class: "super-premium",
		},
	}

	negativeQuantity = resource.MustParse("-1")
	validQuantity    = resource.MustParse("100Gi")

	invalidMachineType = core.MachineType{
		Name:   "",
		CPU:    negativeQuantity,
		GPU:    negativeQuantity,
		Memory: resource.MustParse("-100Gi"),
		Storage: &core.MachineTypeStorage{
			MinSize: &negativeQuantity,
		},
		Architecture: ptr.To("amd64"),
	}
	invalidMachineType2 = core.MachineType{
		Name:   "negative-storage-size",
		CPU:    resource.MustParse("2"),
		GPU:    resource.MustParse("0"),
		Memory: resource.MustParse("100Gi"),
		Storage: &core.MachineTypeStorage{
			StorageSize: &negativeQuantity,
		},
		Architecture: ptr.To("amd64"),
	}
	invalidMachineType3 = core.MachineType{
		Name:   "min-size-and-storage-size",
		CPU:    resource.MustParse("2"),
		GPU:    resource.MustParse("0"),
		Memory: resource.MustParse("100Gi"),
		Storage: &core.MachineTypeStorage{
			MinSize:     &validQuantity,
			StorageSize: &validQuantity,
		},
		Architecture: ptr.To("arm64"),
	}
	invalidMachineType4 = core.MachineType{
		Name:         "empty-storage-config",
		CPU:          resource.MustParse("2"),
		GPU:          resource.MustParse("0"),
		Memory:       resource.MustParse("100Gi"),
		Storage:      &core.MachineTypeStorage{},
		Architecture: ptr.To("foo"),
	}
	invalidMachineTypes = []core.MachineType{
		invalidMachineType,
		invalidMachineType2,
		invalidMachineType3,
		invalidMachineType4,
	}
	invalidVolumeTypes = []core.VolumeType{
		{
			Name:    "",
			Class:   "",
			MinSize: &negativeQuantity,
		},
	}

	regionName = "region1"
	zoneName   = "zone1"

	supportedClassification  = core.ClassificationSupported
	previewClassification    = core.ClassificationPreview
	deprecatedClassification = core.ClassificationDeprecated
	expiredClassification    = core.ClassificationExpired

	updateStrategyMajor = core.MachineImageUpdateStrategy("major")
)

var _ = Describe("CloudProfile Validation Tests ", func() {
	Describe("#ValidateCloudProfile", func() {
		It("should forbid empty CloudProfile resources", func() {
			cloudProfile := &core.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       core.CloudProfileSpec{},
			}

			errorList := ValidateCloudProfile(cloudProfile)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.type"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.kubernetes.versions"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.versions"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.machineImages"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.machineTypes"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.regions"),
				}))))
		})

		Context("tests for unknown cloud profiles", func() {
			var (
				cloudProfile *core.CloudProfile

				duplicatedKubernetes = core.KubernetesSettings{
					Versions: []core.ExpirableVersion{{Version: "1.11.4"}, {Version: "1.11.4", Classification: &previewClassification}},
				}
				duplicatedRegions = []core.Region{
					{
						Name: regionName,
						Zones: []core.AvailabilityZone{
							{Name: zoneName},
						},
					},
					{
						Name: regionName,
						Zones: []core.AvailabilityZone{
							{Name: zoneName},
						},
					},
				}
				duplicatedZones = []core.Region{
					{
						Name: regionName,
						Zones: []core.AvailabilityZone{
							{Name: zoneName},
							{Name: zoneName},
						},
					},
				}
				duplicatedMachineImage = []core.MachineImage{
					{
						Name: "ubuntu",
						Versions: []core.MachineImageVersion{
							{
								ExpirableVersion: core.ExpirableVersion{
									Version: "3.4.6",
								},
								CRI:           []core.CRI{{Name: "containerd"}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: core.ExpirableVersion{
									Version: "3.4.6",
									Lifecycle: []core.LifecycleStage{
										{Classification: previewClassification},
									},
								},
								CRI:           []core.CRI{{Name: "containerd"}},
								Architectures: []string{"amd64"},
							},
						},
						UpdateStrategy: &updateStrategyMajor,
					},
				}
			)

			BeforeEach(func() {
				cloudProfile = &core.CloudProfile{
					ObjectMeta: metadata,
					Spec: core.CloudProfileSpec{
						Type: "unknown",
						SeedSelector: &core.SeedSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"foo": "bar"},
							},
						},
						Kubernetes: core.KubernetesSettings{
							Versions: []core.ExpirableVersion{{
								Version: "1.11.4",
							}},
						},
						MachineImages: []core.MachineImage{
							{
								Name: "some-machineimage",
								Versions: []core.MachineImageVersion{
									{
										ExpirableVersion: core.ExpirableVersion{
											Version: "1.2.3",
										},
										CRI:           []core.CRI{{Name: "containerd"}},
										Architectures: []string{"amd64"},
									},
								},
								UpdateStrategy: &updateStrategyMajor,
							},
						},
						Regions: []core.Region{
							{
								Name: regionName,
								Zones: []core.AvailabilityZone{
									{Name: zoneName},
								},
							},
						},
						MachineTypes: machineTypesConstraint,
						VolumeTypes:  volumeTypesConstraint,
					},
				}
			})

			It("should not return any errors", func() {
				errorList := ValidateCloudProfile(cloudProfile)

				Expect(errorList).To(BeEmpty())
			})

			DescribeTable("CloudProfile metadata",
				func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
					cloudProfile.ObjectMeta = objectMeta

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(matcher)
				},

				Entry("should forbid CloudProfile with empty metadata",
					metav1.ObjectMeta{},
					ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("metadata.name"),
					}))),
				),
				Entry("should forbid CloudProfile with empty name",
					metav1.ObjectMeta{Name: ""},
					ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("metadata.name"),
					}))),
				),
				Entry("should allow CloudProfile with '.' in the name",
					metav1.ObjectMeta{Name: "profile.test"},
					BeEmpty(),
				),
				Entry("should forbid CloudProfile with '_' in the name (not a DNS-1123 subdomain)",
					metav1.ObjectMeta{Name: "profile_test"},
					ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("metadata.name"),
					}))),
				),
			)

			It("should forbid not specifying a type", func() {
				cloudProfile.Spec.Type = ""

				errorList := ValidateCloudProfile(cloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.type"),
				}))))
			})

			It("should forbid ca bundles with unsupported format", func() {
				cloudProfile.Spec.CABundle = ptr.To("unsupported")

				errorList := ValidateCloudProfile(cloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.caBundle"),
				}))))
			})

			Context("kubernetes version constraints", func() {
				It("should enforce that at least one version has been defined", func() {
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.kubernetes.versions"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.versions"),
					}))))
				})

				It("should forbid versions of a not allowed pattern", func() {
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{{Version: "1.11"}}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.versions[0]"),
					}))))
				})

				It("should forbid (deprecated) expiration date on latest kubernetes version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{
						{
							Version:        "1.1.0",
							Classification: &supportedClassification,
						},
						{
							Version:        "1.2.0",
							Classification: &deprecatedClassification,
							ExpirationDate: expirationDate,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.versions[].expirationDate"),
					}))))
				})

				It("should forbid expired lifecycle stage on latest kubernetes version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{
						{
							Version: "1.1.0",
							Lifecycle: []core.LifecycleStage{
								{Classification: supportedClassification},
							},
						},
						{
							Version: "1.2.0",
							Lifecycle: []core.LifecycleStage{
								{Classification: deprecatedClassification},
								{Classification: expiredClassification, StartTime: expirationDate},
							},
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.versions[].lifecycle"),
					}))))
				})

				It("should forbid specifying lifecycle and classification at the same time", func() {
					deprecatedDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{
						{
							Version:        "1.1.0",
							Classification: &supportedClassification,
							Lifecycle: []core.LifecycleStage{
								{Classification: deprecatedClassification, StartTime: deprecatedDate},
							},
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.kubernetes.versions[0]"),
					}))))
				})

				It("should allow multiple directly following lifecycle stages with same start time", func() {
					supportedDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}

					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{
						{
							Version: "1.1.0",
							Lifecycle: []core.LifecycleStage{
								{Classification: supportedClassification, StartTime: supportedDate},
								{Classification: deprecatedClassification, StartTime: supportedDate},
							},
						},
						{
							Version: "1.2.0",
							Lifecycle: []core.LifecycleStage{
								{Classification: supportedClassification},
							},
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(BeEmpty())
				})

				It("should forbid multiple lifecycle stages with same classification", func() {
					supportedDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{
						{
							Version: "1.1.0",
							Lifecycle: []core.LifecycleStage{
								{Classification: supportedClassification},
								{Classification: supportedClassification, StartTime: supportedDate},
							},
						},
						{
							Version: "1.2.0",
							Lifecycle: []core.LifecycleStage{
								{Classification: supportedClassification},
							},
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.kubernetes.versions[0].lifecycle"),
						"Detail": Equal("duplicate classification stage in lifecycle"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.kubernetes.versions[0].lifecycle[1]"),
						"Detail": Equal("lifecycle classifications not in order, must be preview -> supported -> deprecated -> expired"),
					}))))
				})

				It("should forbid unordered lifecycle stages", func() {
					now := time.Now()

					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{
						{
							Version: "1.1.0",
							Lifecycle: []core.LifecycleStage{
								{Classification: previewClassification},
								{Classification: deprecatedClassification, StartTime: ptr.To(metav1.NewTime(now.Add(1 * time.Hour)))},
								{Classification: supportedClassification, StartTime: ptr.To(metav1.NewTime(now.Add(3 * time.Hour)))},
							},
						},
						{
							Version: "1.2.0",
							Lifecycle: []core.LifecycleStage{
								{Classification: supportedClassification},
							},
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.kubernetes.versions[0].lifecycle[2]"),
						"Detail": Equal("lifecycle classifications not in order, must be preview -> supported -> deprecated -> expired"),
					}))))
				})

				It("should forbid missing start time for subsequent lifecycle stages", func() {
					previewDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					deprecatedDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 3)}
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{
						{
							Version: "1.1.0",
							Lifecycle: []core.LifecycleStage{
								{Classification: previewClassification, StartTime: previewDate},
								{Classification: supportedClassification},
								{Classification: deprecatedClassification, StartTime: deprecatedDate},
							},
						},
						{
							Version: "1.2.0",
							Lifecycle: []core.LifecycleStage{
								{Classification: supportedClassification},
							},
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.kubernetes.versions[0].lifecycle[1]"),
						"Detail": Equal("only the leading lifecycle elements can have the start time optional"),
					}))))
				})

				It("should forbid lifecycle start times that are not monotonically increasing", func() {
					now := time.Now()

					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{
						{
							Version: "1.1.0",
							Lifecycle: []core.LifecycleStage{
								{Classification: previewClassification, StartTime: ptr.To(metav1.NewTime(now.Add(0 * time.Hour)))},
								{Classification: supportedClassification, StartTime: ptr.To(metav1.NewTime(now.Add(2 * time.Hour)))},
								{Classification: deprecatedClassification, StartTime: ptr.To(metav1.NewTime(now.Add(1 * time.Hour)))},
							},
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.kubernetes.versions[0].lifecycle[2]"),
						"Detail": Equal("lifecycle start times must be monotonically increasing"),
					}))))
				})

				It("should allow multiple missing start times for leading lifecycle stages", func() {
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{
						{
							Version: "1.1.0",
							Lifecycle: []core.LifecycleStage{
								{Classification: previewClassification},
								{Classification: supportedClassification},
								{Classification: deprecatedClassification},
							},
						},
						{
							Version: "1.2.0",
							Lifecycle: []core.LifecycleStage{
								{Classification: supportedClassification},
							},
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(BeEmpty())
				})

				It("should allow missing start time for first lifecycle stage", func() {
					supportedDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					deprecatedDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 2)}
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{
						{
							Version: "1.1.0",
							Lifecycle: []core.LifecycleStage{
								{Classification: previewClassification},
								{Classification: supportedClassification, StartTime: supportedDate},
								{Classification: deprecatedClassification, StartTime: deprecatedDate},
							},
						},
						{
							Version: "1.2.0",
							Lifecycle: []core.LifecycleStage{
								{Classification: supportedClassification},
							},
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(BeEmpty())
				})

				It("should forbid duplicated kubernetes versions", func() {
					cloudProfile.Spec.Kubernetes = duplicatedKubernetes

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.kubernetes.versions[%d].version", len(duplicatedKubernetes.Versions)-1)),
						}))))
				})

				It("should forbid invalid classification for kubernetes versions", func() {
					classification := core.VersionClassification("dummy")
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{
						{
							Version:        "1.1.0",
							Classification: &classification,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":     Equal(field.ErrorTypeNotSupported),
						"Field":    Equal("spec.kubernetes.versions[0].classification"),
						"BadValue": Equal(classification),
						"Detail":   Equal(`supported values: "deprecated", "expired", "preview", "supported", "unavailable"`),
					}))))
				})

				It("only allow one supported version per minor version", func() {
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{
						{
							Version:        "1.1.0",
							Classification: &supportedClassification,
						},
						{
							Version:        "1.1.1",
							Classification: &supportedClassification,
						},
					}
					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.kubernetes.versions[1]"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.kubernetes.versions[0]"),
					}))))
				})
			})

			Context("machine image validation", func() {
				It("should forbid specifying lifecycle and classification at the same time", func() {
					deprecatedDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "ubuntu",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "3.4.6",
										Classification: &supportedClassification,
										Lifecycle: []core.LifecycleStage{
											{Classification: deprecatedClassification, StartTime: deprecatedDate},
										},
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.machineImages[0].versions[0]"),
						"Detail": Equal("cannot specify `classification` or `expirationDate` in combination with `lifecycle`"),
					}))))
				})

				It("should allow multiple lifecycle stages with same start time", func() {
					supportedDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "ubuntu",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "3.4.6",
										Lifecycle: []core.LifecycleStage{
											{Classification: supportedClassification, StartTime: supportedDate},
										},
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "4.4.6",
										Lifecycle: []core.LifecycleStage{
											{Classification: supportedClassification, StartTime: supportedDate},
											{Classification: deprecatedClassification, StartTime: supportedDate},
										},
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(BeEmpty())
				})

				It("should forbid multiple lifecycle stages with same classification", func() {
					supportedDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}

					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "ubuntu",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "3.4.6",
										Lifecycle: []core.LifecycleStage{
											{Classification: supportedClassification},
										},
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "4.4.6",
										Lifecycle: []core.LifecycleStage{
											{Classification: supportedClassification},
											{Classification: supportedClassification, StartTime: supportedDate},
										},
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.machineImages[0].versions[1].lifecycle"),
						"Detail": Equal("duplicate classification stage in lifecycle"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.machineImages[0].versions[1].lifecycle[1]"),
						"Detail": Equal("lifecycle classifications not in order, must be preview -> supported -> deprecated -> expired"),
					}))))
				})

				It("should forbid unordered lifecycle stages", func() {
					now := time.Now()

					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "ubuntu",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "3.4.6",
										Lifecycle: []core.LifecycleStage{
											{Classification: previewClassification},
											{Classification: deprecatedClassification, StartTime: ptr.To(metav1.NewTime(now.Add(1 * time.Hour)))},
											{Classification: supportedClassification, StartTime: ptr.To(metav1.NewTime(now.Add(2 * time.Hour)))},
										},
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "4.4.6",
										Lifecycle: []core.LifecycleStage{
											{Classification: supportedClassification},
										},
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.machineImages[0].versions[0].lifecycle[2]"),
						"Detail": Equal("lifecycle classifications not in order, must be preview -> supported -> deprecated -> expired"),
					}))))
				})

				It("should forbid missing start time for subsequent lifecycle stages", func() {
					supportedDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}

					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "ubuntu",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "3.4.6",
										Lifecycle: []core.LifecycleStage{
											{Classification: previewClassification},
											{Classification: supportedClassification, StartTime: supportedDate},
											{Classification: deprecatedClassification},
										},
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "4.4.6",
										Lifecycle: []core.LifecycleStage{
											{Classification: supportedClassification},
										},
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.machineImages[0].versions[0].lifecycle[2]"),
						"Detail": Equal("only the leading lifecycle elements can have the start time optional"),
					}))))
				})

				It("should forbid duplicated machineImage versions", func() {
					cloudProfile.Spec.MachineImages = duplicatedMachineImage

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.machineImages[0].versions[%d]", len(duplicatedMachineImage[0].Versions)-1)),
						}))))
				})

				It("should forbid invalid classification for machineImage versions", func() {
					invalidClassification := core.VersionClassification("dummy")

					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "ubuntu",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "3.4.6",
										Lifecycle: []core.LifecycleStage{
											{Classification: invalidClassification},
										},
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":     Equal(field.ErrorTypeNotSupported),
						"Field":    Equal("spec.machineImages[0].versions[0].lifecycle[0].classification"),
						"BadValue": Equal(invalidClassification),
						"Detail":   Equal(`supported values: "deprecated", "expired", "preview", "supported", "unavailable"`),
					}))))
				})

				It("should forbid an empty list of machine images", func() {
					cloudProfile.Spec.MachineImages = []core.MachineImage{}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.machineImages"),
					}))))
				})

				It("should forbid duplicate names in list of machine images", func() {
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "3.4.6",
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
						{
							Name: "some-machineimage",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "3.4.5",
										Classification: &previewClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.machineImages[1]"),
					}))))
				})

				It("should forbid machine images with no version", func() {
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{Name: "some-machineimage"},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.machineImages[0].versions"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineImages"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.machineImages[0].updateStrategy"),
					}))))
				})

				It("should forbid machine images with an invalid machine image update strategy", func() {
					updateStrategy := core.MachineImageUpdateStrategy("dummy")
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "3.4.6",
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategy,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("spec.machineImages[0].updateStrategy"),
					}))))
				})

				It("should allow machine images with a valid machine image update strategy", func() {
					updateStrategy := core.UpdateStrategyMinor
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "3.4.6",
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategy,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(BeEmpty())
				})

				It("should forbid nonSemVer machine image versions", func() {
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "0.1.2",
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
						{
							Name: "xy",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "a.b.c",
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineImages"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineImages[1].versions[0].version"),
					}))))
				})

				It("should allow expiration date on latest machine image version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "0.1.2",
										ExpirationDate: expirationDate,
										Classification: &previewClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "0.1.1",
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
						{
							Name: "xy",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "0.1.1",
										ExpirationDate: expirationDate,
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(BeEmpty())
				})

				It("should forbid invalid classification for machine image versions", func() {
					classification := core.VersionClassification("dummy")
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "0.1.2",
										Classification: &classification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":     Equal(field.ErrorTypeNotSupported),
						"Field":    Equal("spec.machineImages[0].versions[0].classification"),
						"BadValue": Equal(classification),
					}))))
				})

				It("should allow valid CPU architecture for machine image versions", func() {
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "0.1.2",
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "0.1.3",
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "0.1.4",
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"arm64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(BeEmpty())
				})

				It("should forbid invalid CPU architecture for machine image versions", func() {
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "0.1.2",
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"foo"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("spec.machineImages[0].versions[0].architecture"),
					}))))
				})

				It("should allow valid kubeletVersionConstraint for machine image versions", func() {
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "0.1.2",
									},
									CRI:                      []core.CRI{{Name: "containerd"}},
									KubeletVersionConstraint: ptr.To("< 1.26"),
									Architectures:            []string{"amd64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "0.1.3",
									},
									CRI:                      []core.CRI{{Name: "containerd"}},
									KubeletVersionConstraint: ptr.To(">= 1.26"),
									Architectures:            []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(BeEmpty())
				})

				It("should forbid invalid kubeletVersionConstraint for machine image versions", func() {
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "0.1.2",
									},
									CRI:                      []core.CRI{{Name: "containerd"}},
									KubeletVersionConstraint: ptr.To(""),
									Architectures:            []string{"amd64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "0.1.3",
									},
									CRI:                      []core.CRI{{Name: "containerd"}},
									KubeletVersionConstraint: ptr.To("invalid-version"),
									Architectures:            []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.machineImages[0].versions[0].kubeletVersionConstraint"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.machineImages[0].versions[1].kubeletVersionConstraint"),
						})),
					))
				})
			})

			It("should forbid if no cri is present", func() {
				cloudProfile.Spec.MachineImages[0].Versions[0].CRI = nil

				errorList := ValidateCloudProfile(cloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.machineImages[0].versions[0].cri"),
				}))))
			})

			It("should forbid non-supported container runtime interface names", func() {
				cloudProfile.Spec.MachineImages = []core.MachineImage{
					{
						Name: "invalid-cri-name",
						Versions: []core.MachineImageVersion{
							{
								ExpirableVersion: core.ExpirableVersion{
									Version: "0.1.2",
								},
								CRI: []core.CRI{
									{
										Name: "invalid-cri-name",
									},
									{
										Name: "docker",
									},
								},
								Architectures: []string{"amd64"},
							},
						},
						UpdateStrategy: &updateStrategyMajor,
					},
					{
						Name: "valid-cri-name",
						Versions: []core.MachineImageVersion{
							{
								ExpirableVersion: core.ExpirableVersion{
									Version: "0.1.2",
								},
								CRI: []core.CRI{
									{
										Name: core.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64"},
							},
						},
						UpdateStrategy: &updateStrategyMajor,
					},
				}

				errorList := ValidateCloudProfile(cloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeNotSupported),
					"Field":  Equal("spec.machineImages[0].versions[0].cri[0].name"),
					"Detail": Equal("supported values: \"containerd\""),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeNotSupported),
					"Field":  Equal("spec.machineImages[0].versions[0].cri[1].name"),
					"Detail": Equal("supported values: \"containerd\""),
				})),
				))
			})

			It("should forbid duplicated container runtime interface names", func() {
				cloudProfile.Spec.MachineImages[0].Versions[0].CRI = []core.CRI{
					{
						Name: core.CRINameContainerD,
					},
					{
						Name: core.CRINameContainerD,
					},
				}

				errorList := ValidateCloudProfile(cloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.machineImages[0].versions[0].cri[1]"),
				}))))
			})

			It("should forbid duplicated container runtime names", func() {
				cloudProfile.Spec.MachineImages[0].Versions[0].CRI = []core.CRI{
					{
						Name: core.CRINameContainerD,
						ContainerRuntimes: []core.ContainerRuntime{
							{
								Type: "cr1",
							},
							{
								Type: "cr1",
							},
						},
					},
				}

				errorList := ValidateCloudProfile(cloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.machineImages[0].versions[0].cri[0].containerRuntimes[1].type"),
				}))))
			})

			Context("machine types validation", func() {
				It("should enforce that at least one machine type has been defined", func() {
					cloudProfile.Spec.MachineTypes = []core.MachineType{}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.machineTypes"),
					}))))
				})

				It("should enforce uniqueness of machine type names", func() {
					cloudProfile.Spec.MachineTypes = []core.MachineType{
						cloudProfile.Spec.MachineTypes[0],
						cloudProfile.Spec.MachineTypes[0],
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.machineTypes[1].name"),
					}))))
				})

				It("should forbid machine types with unsupported property values", func() {
					cloudProfile.Spec.MachineTypes = invalidMachineTypes

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.machineTypes[0].name"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineTypes[0].cpu"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineTypes[0].gpu"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineTypes[0].memory"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineTypes[0].storage.minSize"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineTypes[1].storage.size"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineTypes[2].storage"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineTypes[3].storage"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("spec.machineTypes[3].architecture"),
					})),
					))
				})

				It("should allow machine types with valid values", func() {
					cloudProfile.Spec.MachineTypes = machineTypesConstraint

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(BeEmpty())
				})
			})

			Context("regions validation", func() {
				It("should forbid regions with unsupported name values", func() {
					cloudProfile.Spec.Regions = []core.Region{
						{
							Name:  "",
							Zones: []core.AvailabilityZone{{Name: ""}},
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.regions[0].name"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.regions[0].zones[0].name"),
					}))))
				})

				It("should forbid duplicated region names", func() {
					cloudProfile.Spec.Regions = duplicatedRegions

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.regions[%d].name", len(duplicatedRegions)-1)),
						}))))
				})

				It("should forbid duplicated zone names", func() {
					cloudProfile.Spec.Regions = duplicatedZones

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.regions[0].zones[%d].name", len(duplicatedZones[0].Zones)-1)),
						}))))
				})

				It("should forbid invalid label specifications", func() {
					cloudProfile.Spec.Regions[0].Labels = map[string]string{
						"this-is-not-allowed": "?*&!@",
						"toolongtoolongtoolongtoolongtoolongtoolongtoolongtoolongtoolongtoolongtoolong": "toolongtoolongtoolongtoolongtoolongtoolongtoolongtoolongtoolongtoolongtoolong",
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.regions[0].labels"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.regions[0].labels"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.regions[0].labels"),
						})),
					))
				})
			})

			Context("volume types validation", func() {
				It("should enforce uniqueness of volume type names", func() {
					cloudProfile.Spec.VolumeTypes = []core.VolumeType{
						cloudProfile.Spec.VolumeTypes[0],
						cloudProfile.Spec.VolumeTypes[0],
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.volumeTypes[1].name"),
					}))))
				})

				It("should forbid volume types with unsupported property values", func() {
					cloudProfile.Spec.VolumeTypes = invalidVolumeTypes

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.volumeTypes[0].name"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.volumeTypes[0].class"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.volumeTypes[0].minSize"),
					})),
					))
				})
			})

			Context("bastion validation", func() {
				It("should forbid empty bastion", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.bastion"),
					}))))
				})

				It("should forbid unknown machineType", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineType: &core.BastionMachineType{Name: "unknown"},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.bastion.machineType.name"),
					}))))
				})

				It("should allow known machineType", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineType: &core.BastionMachineType{Name: machineType.Name},
					}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(BeEmpty())
				})

				It("should forbid unknown machineImage", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineImage: &core.BastionMachineImage{Name: "unknown"},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.bastion.machineImage.name"),
					}))))
				})

				It("should forbid preview image", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineImage: &core.BastionMachineImage{Name: "some-machineimage"},
					}
					cloudProfile.Spec.MachineImages[0].Versions[0].Classification = &previewClassification
					cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{"amd64"}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.bastion.machineImage.name"),
					}))))
				})

				It("should forbid no arch images", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineImage: &core.BastionMachineImage{Name: "some-machineimage"},
					}
					cloudProfile.Spec.MachineImages[0].Versions[0].Classification = &supportedClassification
					cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.bastion.machineImage.name"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.machineImages[0].versions[0].architectures"),
					}))))
				})

				It("should allow images with supported classification and architecture specification", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineImage: &core.BastionMachineImage{Name: "some-machineimage"},
					}
					cloudProfile.Spec.MachineImages[0].Versions[0].Classification = &supportedClassification
					cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{"amd64"}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(BeEmpty())
				})

				It("should allow matching arch of machineType and machineImage", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineType: &core.BastionMachineType{Name: machineType.Name},
						MachineImage: &core.BastionMachineImage{
							Name:    "some-machineimage",
							Version: ptr.To("1.2.3"),
						},
					}
					cloudProfile.Spec.MachineImages[0].Versions[0].Classification = &supportedClassification
					cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{*machineType.Architecture}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(BeEmpty())
				})

				It("should forbid different arch of machineType and machineImage", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineType: &core.BastionMachineType{Name: machineType.Name},
						MachineImage: &core.BastionMachineImage{
							Name:    "some-machineimage",
							Version: ptr.To("1.2.3"),
						},
					}
					cloudProfile.Spec.MachineImages[0].Versions[0].Classification = &supportedClassification
					cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{"arm64"}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.bastion.machineImage.version"),
					}))))
				})

				It("should allow any image architecture if machineType is nil", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineType: nil,
						MachineImage: &core.BastionMachineImage{
							Name:    "some-machineimage",
							Version: ptr.To("1.2.3"),
						},
					}
					cloudProfile.Spec.MachineImages[0].Versions[0].Classification = &supportedClassification
					// architectures must be one of arm64 or amd64
					cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{"arm64"}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(BeEmpty())
				})
			})

			It("should forbid unsupported seed selectors", func() {
				cloudProfile.Spec.SeedSelector.MatchLabels["foo"] = "no/slash/allowed"

				errorList := ValidateCloudProfile(cloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.seedSelector.matchLabels"),
				}))))
			})
		})
	})

	var (
		cloudProfileNew *core.CloudProfile
		cloudProfileOld *core.CloudProfile
		dateInThePast   = &metav1.Time{Time: time.Now().AddDate(-5, 0, 0)}
	)

	Describe("#ValidateCloudProfileSpecUpdate", func() {
		BeforeEach(func() {
			cloudProfileNew = &core.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "dummy",
					Name:            "dummy",
				},
				Spec: core.CloudProfileSpec{
					Type: "aws",
					MachineImages: []core.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "1.2.3",
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					},
					Kubernetes: core.KubernetesSettings{
						Versions: []core.ExpirableVersion{
							{
								Version:        "1.17.2",
								Classification: &deprecatedClassification,
							},
						},
					},
					Regions: []core.Region{
						{
							Name: regionName,
							Zones: []core.AvailabilityZone{
								{Name: zoneName},
							},
						},
					},
					MachineTypes: machineTypesConstraint,
					VolumeTypes:  volumeTypesConstraint,
				},
			}
			cloudProfileOld = cloudProfileNew.DeepCopy()
		})

		Context("Removed Kubernetes versions", func() {
			It("deleting version - should not return any errors", func() {
				versions := []core.ExpirableVersion{
					{Version: "1.17.2", Classification: &deprecatedClassification},
					{Version: "1.17.1", Classification: &deprecatedClassification, ExpirationDate: dateInThePast},
					{Version: "1.17.0", Classification: &deprecatedClassification, ExpirationDate: dateInThePast},
				}
				cloudProfileNew.Spec.Kubernetes.Versions = versions[0:1]
				cloudProfileOld.Spec.Kubernetes.Versions = versions
				errorList := ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)

				Expect(errorList).To(BeEmpty())
			})
		})

		Context("Removed MachineImage versions", func() {
			It("deleting version - should not return any errors", func() {
				versions := []core.MachineImageVersion{
					{
						ExpirableVersion: core.ExpirableVersion{
							Version:        "2135.6.2",
							Classification: &deprecatedClassification,
						},
						CRI:           []core.CRI{{Name: "containerd"}},
						Architectures: []string{"amd64"},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version:        "2135.6.1",
							Classification: &deprecatedClassification,
							ExpirationDate: dateInThePast,
						},
						CRI:           []core.CRI{{Name: "containerd"}},
						Architectures: []string{"amd64"},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version:        "2135.6.0",
							Classification: &deprecatedClassification,
							ExpirationDate: dateInThePast,
						},
						CRI:           []core.CRI{{Name: "containerd"}},
						Architectures: []string{"amd64"},
					},
				}
				cloudProfileNew.Spec.MachineImages[0].Versions = versions[0:1]
				cloudProfileOld.Spec.MachineImages[0].Versions = versions
				errorList := ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)

				Expect(errorList).To(BeEmpty())
			})
		})

		Context("Update version from supported to unavailable", func() {
			It("should prevent a kubernetes version update to unavailable when it was already active", func() {
				now := time.Now()

				cloudProfileNew.Spec.Kubernetes.Versions = []core.ExpirableVersion{
					{
						Version: "1.17.2",
						Lifecycle: []core.LifecycleStage{
							{
								Classification: core.ClassificationSupported,
								StartTime:      ptr.To(metav1.NewTime(now.Add(1 * time.Hour))),
							},
						},
					},
				}
				cloudProfileOld.Spec.Kubernetes.Versions = []core.ExpirableVersion{
					{
						Version: "1.17.2",
						Lifecycle: []core.LifecycleStage{
							{
								Classification: core.ClassificationSupported,
							},
						},
					},
				}
				errorList := ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.kubernetes.versions[0]"),
					"Detail": Equal("a version cannot be turned into unavailable if it already was already moved into a later lifecycle stage"),
				}))))
			})

			It("should prevent a machine image update to unavailable when it was already active", func() {
				cloudProfileNew.Spec.MachineImages = []core.MachineImage{
					{
						Name:           "ubuntu",
						UpdateStrategy: &updateStrategyMajor,
						Versions: []core.MachineImageVersion{
							{
								CRI:           []core.CRI{{Name: "containerd"}},
								Architectures: []string{"amd64"},
								ExpirableVersion: core.ExpirableVersion{
									Version: "2135.6.0",
									Lifecycle: []core.LifecycleStage{
										{
											Classification: core.ClassificationUnavailable,
										},
									},
								},
							},
						},
					},
				}
				cloudProfileOld.Spec.MachineImages = []core.MachineImage{
					{
						Name:           "ubuntu",
						UpdateStrategy: &updateStrategyMajor,
						Versions: []core.MachineImageVersion{
							{
								CRI:           []core.CRI{{Name: "containerd"}},
								Architectures: []string{"amd64"},
								ExpirableVersion: core.ExpirableVersion{
									Version: "2135.6.0",
									Lifecycle: []core.LifecycleStage{
										{
											Classification: core.ClassificationPreview,
										},
									},
								},
							},
						},
					},
				}

				errorList := ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.machineImages[0].versions[0]"),
					"Detail": Equal("a version cannot be turned into unavailable if it already was already moved into a later lifecycle stage"),
				}))))
			})
		})
	})
})
