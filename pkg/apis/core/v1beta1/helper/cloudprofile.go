package helper

import (
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"time"
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
			version.Lifecycle = append(version.Lifecycle, v1beta1.ClassificationLifecycle{
				Classification: *version.Classification,
			})
		}

		if version.ExpirationDate != nil {
			if version.Classification == nil {
				version.Lifecycle = append(version.Lifecycle, v1beta1.ClassificationLifecycle{
					Classification: v1beta1.ClassificationSupported,
				})
			}

			version.Lifecycle = append(version.Lifecycle, v1beta1.ClassificationLifecycle{
				Classification: v1beta1.ClassificationExpired,
				StartTime:      version.ExpirationDate,
			})
		}
	}

	if len(version.Lifecycle) == 0 {
		// when there is no classification lifecycle defined then default to supported
		version.Lifecycle = append(version.Lifecycle, v1beta1.ClassificationLifecycle{
			Classification: v1beta1.ClassificationSupported,
		})
	}

	for _, l := range version.Lifecycle {
		startTime := time.Time{}
		if l.StartTime != nil {
			startTime = l.StartTime.Time
		}

		if startTime.Before(currentTime) {
			currentClassification = l.Classification
		}
	}

	return currentClassification
}

func VersionIsExpired(version v1beta1.ExpirableVersion) bool {
	return CurrentLifecycleClassification(version) == v1beta1.ClassificationExpired
}
