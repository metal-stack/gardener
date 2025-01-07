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
			version.Lifecycle = append(version.Lifecycle, core.LifecycleStage{
				Classification: *version.Classification,
			})
		}

		if version.ExpirationDate != nil {
			if version.Classification == nil {
				version.Lifecycle = append(version.Lifecycle, core.LifecycleStage{
					Classification: core.ClassificationSupported,
				})
			}

			version.Lifecycle = append(version.Lifecycle, core.LifecycleStage{
				Classification: core.ClassificationExpired,
				StartTime:      version.ExpirationDate,
			})
		}
	}

	if len(version.Lifecycle) == 0 {
		// when there is no classification lifecycle defined then default to supported
		version.Lifecycle = append(version.Lifecycle, core.LifecycleStage{
			Classification: core.ClassificationSupported,
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

func VersionIsSupported(version core.ExpirableVersion) bool {
	return CurrentLifecycleClassification(version) == core.ClassificationSupported
}

func SupportedLifecycleClassification(version core.ExpirableVersion) *core.LifecycleStage {
	for _, stage := range version.Lifecycle {
		if stage.Classification == core.ClassificationSupported {
			return &stage
		}
	}
	return nil
}
