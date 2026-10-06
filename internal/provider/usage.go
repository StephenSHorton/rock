package provider

import (
	"context"
	"strings"
	"time"
)

// SubscriptionUsage is how much of a subscription's included allowance is
// used in the current period. Providers fill only what their backend
// reports; Rock never derives a limit it was not given.
type SubscriptionUsage struct {
	// Percent used, 0-100 (clamped).
	Percent float64
	// Label names the allowance: "Weekly limit", "Monthly limit", or
	// "Usage" when the period is unknown.
	Label string
	// ResetsAt is when the current period ends (zero when unknown).
	ResetsAt time.Time
	// Plan is the subscription tier name when reported (e.g. "SuperGrok Heavy").
	Plan string
}

// UsageReporter is an optional Provider method. ok is false when the
// provider has nothing to report right now (no live connection, no data).
type UsageReporter interface {
	SubscriptionUsage(ctx context.Context) (u SubscriptionUsage, ok bool, err error)
}

// UsageLabel maps a period type (e.g. USAGE_PERIOD_TYPE_WEEKLY) to its label.
func UsageLabel(periodType string) string {
	t := strings.ToUpper(periodType)
	switch {
	case strings.Contains(t, "WEEK"):
		return "Weekly limit"
	case strings.Contains(t, "MONTH"):
		return "Monthly limit"
	}
	return "Usage"
}
