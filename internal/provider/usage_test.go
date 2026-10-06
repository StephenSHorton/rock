package provider

import "testing"

func TestUsageLabel(t *testing.T) {
	for in, want := range map[string]string{
		"USAGE_PERIOD_TYPE_WEEKLY":      "Weekly limit",
		"USAGE_PERIOD_TYPE_MONTHLY":     "Monthly limit",
		"weekly":                        "Weekly limit",
		"USAGE_PERIOD_TYPE_UNSPECIFIED": "Usage",
		"":                              "Usage",
	} {
		if got := UsageLabel(in); got != want {
			t.Fatalf("%q: %q want %q", in, got, want)
		}
	}
}
