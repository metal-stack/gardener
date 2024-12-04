package helper_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

var _ = Describe("CloudProfile Helper", func() {
	var (
		now = time.Now()
	)

	Context("calculate the current lifecycle classification", func() {
		It("only version is given", func() {
			classification := helper.CurrentLifecycleClassification(gardencorev1beta1.ExpirableVersion{
				Version: "1.28.0",
			})
			Expect(classification).To(Equal(gardencorev1beta1.ClassificationSupported))
		})

		It("unavailable classification due to scheduled lifecycle start in the future", func() {
			classification := helper.CurrentLifecycleClassification(gardencorev1beta1.ExpirableVersion{
				Version: "1.28.0",
				Lifecycle: []gardencorev1beta1.ClassificationLifecycle{
					{
						Classification: gardencorev1beta1.ClassificationSupported,
						StartTime:      ptr.To(metav1.NewTime(now.Add(3 * time.Hour))),
					},
				},
			})
			Expect(classification).To(Equal(gardencorev1beta1.ClassificationUnavailable))
		})

		It("version is in preview stage", func() {
			classification := helper.CurrentLifecycleClassification(gardencorev1beta1.ExpirableVersion{
				Version: "1.28.0",
				Lifecycle: []gardencorev1beta1.ClassificationLifecycle{
					{
						Classification: gardencorev1beta1.ClassificationPreview,
						StartTime:      ptr.To(metav1.NewTime(now.Add(-1 * time.Hour))),
					},
					{
						Classification: gardencorev1beta1.ClassificationSupported,
						StartTime:      ptr.To(metav1.NewTime(now.Add(3 * time.Hour))),
					},
				},
			})
			Expect(classification).To(Equal(gardencorev1beta1.ClassificationPreview))
		})

		It("full version lifecycle with version currently in supported stage", func() {
			classification := helper.CurrentLifecycleClassification(gardencorev1beta1.ExpirableVersion{
				Version: "1.28.0",
				Lifecycle: []gardencorev1beta1.ClassificationLifecycle{
					{
						Classification: gardencorev1beta1.ClassificationPreview,
						StartTime:      ptr.To(metav1.NewTime(now.Add(-3 * time.Hour))),
					},
					{
						Classification: gardencorev1beta1.ClassificationSupported,
						StartTime:      ptr.To(metav1.NewTime(now.Add(-1 * time.Hour))),
					},
					{
						Classification: gardencorev1beta1.ClassificationDeprecated,
						StartTime:      ptr.To(metav1.NewTime(now.Add(5 * time.Hour))),
					},
					{
						Classification: gardencorev1beta1.ClassificationExpired,
						StartTime:      ptr.To(metav1.NewTime(now.Add(8 * time.Hour))),
					},
				},
			})
			Expect(classification).To(Equal(gardencorev1beta1.ClassificationSupported))
		})

		It("version is expired", func() {
			classification := helper.CurrentLifecycleClassification(gardencorev1beta1.ExpirableVersion{
				Version: "1.28.0",
				Lifecycle: []gardencorev1beta1.ClassificationLifecycle{
					{
						Classification: gardencorev1beta1.ClassificationSupported,
						StartTime:      ptr.To(metav1.NewTime(now.Add(-4 * time.Hour))),
					},
					{
						Classification: gardencorev1beta1.ClassificationDeprecated,
						StartTime:      ptr.To(metav1.NewTime(now.Add(-3 * time.Hour))),
					},
					{
						Classification: gardencorev1beta1.ClassificationExpired,
						StartTime:      ptr.To(metav1.NewTime(now.Add(-1 * time.Hour))),
					},
				},
			})
			Expect(classification).To(Equal(gardencorev1beta1.ClassificationExpired))
		})

		It("first lifecycle start time field is optional", func() {
			classification := helper.CurrentLifecycleClassification(gardencorev1beta1.ExpirableVersion{
				Version: "1.28.5",
				Lifecycle: []gardencorev1beta1.ClassificationLifecycle{
					{
						Classification: gardencorev1beta1.ClassificationPreview,
					},
					{
						Classification: gardencorev1beta1.ClassificationSupported,
						StartTime:      ptr.To(metav1.NewTime(now.Add(3 * time.Hour))),
					},
					{
						Classification: gardencorev1beta1.ClassificationDeprecated,
						StartTime:      ptr.To(metav1.NewTime(now.Add(4 * time.Hour))),
					},
					{
						Classification: gardencorev1beta1.ClassificationExpired,
						StartTime:      ptr.To(metav1.NewTime(now.Add(5 * time.Hour))),
					},
				},
			})
			Expect(classification).To(Equal(gardencorev1beta1.ClassificationPreview))
		})

		It("determining supported for deprecated classification field", func() {
			classification := helper.CurrentLifecycleClassification(gardencorev1beta1.ExpirableVersion{
				Classification: ptr.To(gardencorev1beta1.ClassificationSupported),
				Version:        "1.28.0",
			})
			Expect(classification).To(Equal(gardencorev1beta1.ClassificationSupported))
		})

		It("determining expired for deprecated expiration date field", func() {
			classification := helper.CurrentLifecycleClassification(gardencorev1beta1.ExpirableVersion{
				ExpirationDate: ptr.To(metav1.NewTime(now.Add(-1 * time.Hour))),
				Version:        "1.28.0",
			})
			Expect(classification).To(Equal(gardencorev1beta1.ClassificationExpired))
		})

		It("determining preview for deprecated classification and expiration date field", func() {
			classification := helper.CurrentLifecycleClassification(gardencorev1beta1.ExpirableVersion{
				Classification: ptr.To(gardencorev1beta1.ClassificationPreview),
				Version:        "1.28.0",
				ExpirationDate: ptr.To(metav1.NewTime(now.Add(3 * time.Hour))),
			})
			Expect(classification).To(Equal(gardencorev1beta1.ClassificationPreview))
		})

	})
})
