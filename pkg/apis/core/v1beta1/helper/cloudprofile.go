package helper

import (
	"time"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

func CurrentLifecycleClassification(version v1beta1.ExpirableVersion) v1beta1.VersionClassification {
	var (
		currentClassification = v1beta1.ClassificationUnavailable
		currentTime           = time.Now()
	)

	if version.Classification != nil || version.ExpirationDate != nil {
		// old cloud profile definition, convert to lifecycle
		// this can be removed as soon as we remove the old classification and expiration date fields

		if version.Classification != nil {
			version.Lifecycle = append(version.Lifecycle, v1beta1.LifecycleStage{
				Classification: *version.Classification,
			})
		}

		if version.ExpirationDate != nil {
			if version.Classification == nil {
				version.Lifecycle = append(version.Lifecycle, v1beta1.LifecycleStage{
					Classification: v1beta1.ClassificationSupported,
				})
			}

			version.Lifecycle = append(version.Lifecycle, v1beta1.LifecycleStage{
				Classification: v1beta1.ClassificationExpired,
				StartTime:      version.ExpirationDate,
			})
		}
	}

	if len(version.Lifecycle) == 0 {
		// when there is no classification lifecycle defined then default to supported
		version.Lifecycle = append(version.Lifecycle, v1beta1.LifecycleStage{
			Classification: v1beta1.ClassificationSupported,
		})
	}

	for _, stage := range version.Lifecycle {
		startTime := time.Time{}
		if stage.StartTime != nil {
			startTime = stage.StartTime.Time
		}

		if startTime.Before(currentTime) {
			currentClassification = stage.Classification
		}
	}

	return currentClassification
}

func VersionIsExpired(version v1beta1.ExpirableVersion) bool {
	return CurrentLifecycleClassification(version) == v1beta1.ClassificationExpired
}

func VersionIsActive(version v1beta1.ExpirableVersion) bool {
	curr := CurrentLifecycleClassification(version)
	return curr != v1beta1.ClassificationExpired && curr != v1beta1.ClassificationUnavailable
}

func VersionIsSupported(version v1beta1.ExpirableVersion) bool {
	return CurrentLifecycleClassification(version) == v1beta1.ClassificationSupported
}

func VersionIsPreview(version v1beta1.ExpirableVersion) bool {
	return CurrentLifecycleClassification(version) == v1beta1.ClassificationPreview
}

func DurationUntilNextVersionLifecycleStage(cloudProfile *v1beta1.CloudProfileSpec) time.Duration {
	var (
		next time.Time
		now  = time.Now()
	)

	for _, version := range cloudProfile.Kubernetes.Versions {
		for _, stage := range version.Lifecycle {
			if stage.StartTime == nil {
				continue
			}
			time := stage.StartTime.Time
			if now.Before(time) && next.IsZero() || next.After(time) {
				next = time
			}
		}
	}

	for _, image := range cloudProfile.MachineImages {
		for _, version := range image.Versions {

			for _, stage := range version.Lifecycle {
				if stage.StartTime == nil {
					continue
				}
				time := stage.StartTime.Time
				if now.Before(time) && next.After(time) {
					next = time
				}
			}
		}
	}

	if next.IsZero() {
		return 0
	}
	return next.Sub(now)
}
