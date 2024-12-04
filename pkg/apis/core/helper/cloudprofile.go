package helper

import (
	"time"

	"github.com/gardener/gardener/pkg/apis/core"
)

func CurrentLifecycleClassification(version core.ExpirableVersion) core.VersionClassification {
	var (
		currentClassification = core.ClassificationUnavailable
		currentTime           = time.Now()
	)

	if version.Classification != nil || version.ExpirationDate != nil {
		// old cloud profile definition, convert to lifecycle
		// this can be removed as soon as we remove the old classification and expiration date fields

		if version.Classification != nil {
			version.Lifecycle = append(version.Lifecycle, core.ClassificationLifecycle{
				Classification: *version.Classification,
			})
		}

		if version.ExpirationDate != nil {
			if version.Classification == nil {
				version.Lifecycle = append(version.Lifecycle, core.ClassificationLifecycle{
					Classification: core.ClassificationSupported,
				})
			}

			version.Lifecycle = append(version.Lifecycle, core.ClassificationLifecycle{
				Classification: core.ClassificationExpired,
				StartTime:      version.ExpirationDate,
			})
		}
	}

	if len(version.Lifecycle) == 0 {
		// when there is no classification lifecycle defined then default to supported
		version.Lifecycle = append(version.Lifecycle, core.ClassificationLifecycle{
			Classification: core.ClassificationSupported,
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
