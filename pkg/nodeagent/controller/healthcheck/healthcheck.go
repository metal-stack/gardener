package healthcheck

import (
	"context"
	"time"
)

const maxFailureDuration = 60 * time.Second

// HealthChecker can be implemented to run a healthcheck against a node component
// and repair if possible, otherwise fail and report bach
type HealthChecker interface {
	// Name returns the name of the healthchecker
	Name() string
	// Check executes the healthcheck
	Check(ctx context.Context) error
}
