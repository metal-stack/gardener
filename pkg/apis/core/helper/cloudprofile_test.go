package helper_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
)

var _ = Describe("CloudProfile Helper", func() {
	var (
		now = time.Now()
	)

	Context("calculate the current lifecycle classification", func() {
		It("only version is given", func() {
			classification := helper.CurrentLifecycleClassification(core.ExpirableVersion{
				Version: "1.28.0",
			})
			Expect(classification).To(Equal(core.ClassificationSupported))
		})

		It("unavailable classification due to scheduled lifecycle start in the future", func() {
			classification := helper.CurrentLifecycleClassification(core.ExpirableVersion{
				Version: "1.28.0",
				Lifecycles: []core.LifecycleStage{
					{
						Classification: core.ClassificationSupported,
						StartTime:      ptr.To(metav1.NewTime(now.Add(3 * time.Hour))),
					},
				},
			})
			Expect(classification).To(Equal(core.ClassificationUnavailable))
		})

		It("version is in preview stage", func() {
			classification := helper.CurrentLifecycleClassification(core.ExpirableVersion{
				Version: "1.28.0",
				Lifecycles: []core.LifecycleStage{
					{
						Classification: core.ClassificationPreview,
						StartTime:      ptr.To(metav1.NewTime(now.Add(-1 * time.Hour))),
					},
					{
						Classification: core.ClassificationSupported,
						StartTime:      ptr.To(metav1.NewTime(now.Add(3 * time.Hour))),
					},
				},
			})
			Expect(classification).To(Equal(core.ClassificationPreview))
		})

		It("full version lifecycle with version currently in supported stage", func() {
			classification := helper.CurrentLifecycleClassification(core.ExpirableVersion{
				Version: "1.28.0",
				Lifecycles: []core.LifecycleStage{
					{
						Classification: core.ClassificationPreview,
						StartTime:      ptr.To(metav1.NewTime(now.Add(-3 * time.Hour))),
					},
					{
						Classification: core.ClassificationSupported,
						StartTime:      ptr.To(metav1.NewTime(now.Add(-1 * time.Hour))),
					},
					{
						Classification: core.ClassificationDeprecated,
						StartTime:      ptr.To(metav1.NewTime(now.Add(5 * time.Hour))),
					},
					{
						Classification: core.ClassificationExpired,
						StartTime:      ptr.To(metav1.NewTime(now.Add(8 * time.Hour))),
					},
				},
			})
			Expect(classification).To(Equal(core.ClassificationSupported))
		})

		It("version is expired", func() {
			classification := helper.CurrentLifecycleClassification(core.ExpirableVersion{
				Version: "1.28.0",
				Lifecycles: []core.LifecycleStage{
					{
						Classification: core.ClassificationSupported,
						StartTime:      ptr.To(metav1.NewTime(now.Add(-4 * time.Hour))),
					},
					{
						Classification: core.ClassificationDeprecated,
						StartTime:      ptr.To(metav1.NewTime(now.Add(-3 * time.Hour))),
					},
					{
						Classification: core.ClassificationExpired,
						StartTime:      ptr.To(metav1.NewTime(now.Add(-1 * time.Hour))),
					},
				},
			})
			Expect(classification).To(Equal(core.ClassificationExpired))
		})

		It("first lifecycle start time field is optional", func() {
			classification := helper.CurrentLifecycleClassification(core.ExpirableVersion{
				Version: "1.28.5",
				Lifecycles: []core.LifecycleStage{
					{
						Classification: core.ClassificationPreview,
					},
					{
						Classification: core.ClassificationSupported,
						StartTime:      ptr.To(metav1.NewTime(now.Add(3 * time.Hour))),
					},
					{
						Classification: core.ClassificationDeprecated,
						StartTime:      ptr.To(metav1.NewTime(now.Add(4 * time.Hour))),
					},
					{
						Classification: core.ClassificationExpired,
						StartTime:      ptr.To(metav1.NewTime(now.Add(5 * time.Hour))),
					},
				},
			})
			Expect(classification).To(Equal(core.ClassificationPreview))
		})

		It("determining supported for deprecated classification field", func() {
			classification := helper.CurrentLifecycleClassification(core.ExpirableVersion{
				Classification: ptr.To(core.ClassificationSupported),
				Version:        "1.28.0",
			})
			Expect(classification).To(Equal(core.ClassificationSupported))
		})

		It("determining expired for deprecated expiration date field", func() {
			classification := helper.CurrentLifecycleClassification(core.ExpirableVersion{
				ExpirationDate: ptr.To(metav1.NewTime(now.Add(-1 * time.Hour))),
				Version:        "1.28.0",
			})
			Expect(classification).To(Equal(core.ClassificationExpired))
		})

		It("determining preview for deprecated classification and expiration date field", func() {
			classification := helper.CurrentLifecycleClassification(core.ExpirableVersion{
				Classification: ptr.To(core.ClassificationPreview),
				Version:        "1.28.0",
				ExpirationDate: ptr.To(metav1.NewTime(now.Add(3 * time.Hour))),
			})
			Expect(classification).To(Equal(core.ClassificationPreview))
		})

	})
})
