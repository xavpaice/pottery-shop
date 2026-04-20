package metrics

import (
	"context"
	"log"
	"sync/atomic"
	"time"
)

// FeatureChecker polls a license field and exposes a thread-safe Enabled() method.
// It reacts to license changes without requiring a restart.
type FeatureChecker struct {
	sdkServiceName string
	fieldName      string
	fallback       bool
	interval       time.Duration
	enabled        atomic.Bool
}

func NewFeatureChecker(sdkServiceName, fieldName string, fallback bool, interval time.Duration) *FeatureChecker {
	fc := &FeatureChecker{
		sdkServiceName: sdkServiceName,
		fieldName:      fieldName,
		fallback:       fallback,
		interval:       interval,
	}
	fc.enabled.Store(fallback)
	return fc
}

func (fc *FeatureChecker) Enabled() bool {
	return fc.enabled.Load()
}

func (fc *FeatureChecker) Run(ctx context.Context) {
	// Check immediately on startup after a short delay for the SDK
	time.Sleep(10 * time.Second)
	fc.check()

	ticker := time.NewTicker(fc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fc.check()
		}
	}
}

func (fc *FeatureChecker) check() {
	val := CheckLicenseFieldBool(fc.sdkServiceName, fc.fieldName, fc.fallback)
	prev := fc.enabled.Swap(val)
	if prev != val {
		log.Printf("license: feature %s changed: %v -> %v", fc.fieldName, prev, val)
	}
}
