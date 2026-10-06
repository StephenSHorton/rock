package grokcli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/StephenSHorton/rock/internal/provider"
)

// BillingMethod is grok's ACP extension request for the signed-in user's
// Grok Build credit usage. The leading underscore is required: grok 1.0.41
// answers "x.ai/billing" with Method not found. It needs no session; it
// works right after initialize.
const BillingMethod = "_x.ai/billing"

// BillingTimeout bounds one billing query. It never blocks a turn.
var BillingTimeout = 5 * time.Second

// SubscriptionUsage asks the live grok child for credit usage. It never
// spawns a child: with none running it reports ok=false.
func (p *Provider) SubscriptionUsage(ctx context.Context) (provider.SubscriptionUsage, bool, error) {
	p.mu.Lock()
	var c *client
	if p.sess != nil && p.sess.client.alive() {
		c = p.sess.client
	}
	p.mu.Unlock()
	if c == nil {
		return provider.SubscriptionUsage{}, false, nil
	}
	ctx, cancel := withTimeout(ctx, BillingTimeout)
	defer cancel()
	raw, err := c.request(ctx, BillingMethod, map[string]any{})
	if err != nil {
		return provider.SubscriptionUsage{}, false, err
	}
	u, ok := ParseBilling(raw)
	return u, ok, nil
}

// ParseBilling reads a _x.ai/billing result. It accepts the result bare
// ({"config":{...},"subscription_tier":...}), still wrapped in a JSON-RPC
// envelope ({"result":{...}}), or the config object alone, in camelCase or
// snake_case. ok is false when no usage percent can be found.
func ParseBilling(raw json.RawMessage) (provider.SubscriptionUsage, bool) {
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return provider.SubscriptionUsage{}, false
	}
	for range 3 {
		inner, ok := pick(top, "result")
		if !ok {
			break
		}
		var next map[string]json.RawMessage
		if json.Unmarshal(inner, &next) != nil {
			return provider.SubscriptionUsage{}, false
		}
		top = next
	}
	cfg := top
	if rawCfg, ok := pick(top, "config"); ok {
		cfg = nil
		if json.Unmarshal(rawCfg, &cfg) != nil || cfg == nil {
			return provider.SubscriptionUsage{}, false
		}
	}

	var u provider.SubscriptionUsage
	u.Plan = str(top, "subscription_tier", "subscriptionTier")
	if u.Plan == "" {
		u.Plan = str(cfg, "subscription_tier", "subscriptionTier")
	}

	pct, havePct := num(cfg, "creditUsagePercent", "credit_usage_percent")
	if !havePct {
		// Older GrokBuildBillingConfig shape: used / monthlyLimit in cents.
		limit, okL := cents(cfg, "monthlyLimit", "monthly_limit")
		used, okU := cents(cfg, "used")
		if !okL || !okU || limit <= 0 {
			return provider.SubscriptionUsage{}, false
		}
		pct = used / limit * 100
	}
	if math.IsNaN(pct) || math.IsInf(pct, 0) {
		return provider.SubscriptionUsage{}, false
	}
	u.Percent = math.Max(0, math.Min(100, pct))

	periodType, end := "", ""
	if rawP, ok := pick(cfg, "currentPeriod", "current_period"); ok {
		var period map[string]json.RawMessage
		if json.Unmarshal(rawP, &period) == nil {
			periodType = str(period, "type", "periodType", "period_type")
			end = str(period, "end")
		}
	}
	if end == "" {
		end = str(cfg, "billingPeriodEnd", "billing_period_end")
	}
	u.Label = provider.UsageLabel(periodType)
	if end != "" {
		if t, err := time.Parse(time.RFC3339Nano, end); err == nil {
			u.ResetsAt = t
		}
	}
	return u, true
}

func pick(m map[string]json.RawMessage, keys ...string) (json.RawMessage, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok && len(v) > 0 && string(v) != "null" {
			return v, true
		}
	}
	return nil, false
}

func str(m map[string]json.RawMessage, keys ...string) string {
	v, ok := pick(m, keys...)
	if !ok {
		return ""
	}
	var s string
	if json.Unmarshal(v, &s) != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

// num reads a JSON number (or a numeric string, as proto JSON sometimes
// sends) from the first present key.
func num(m map[string]json.RawMessage, keys ...string) (float64, bool) {
	v, ok := pick(m, keys...)
	if !ok {
		return 0, false
	}
	var f float64
	if json.Unmarshal(v, &f) == nil {
		return f, true
	}
	var s string
	if json.Unmarshal(v, &s) == nil {
		if _, err := fmt.Sscan(strings.TrimSpace(s), &f); err == nil {
			return f, true
		}
	}
	return 0, false
}

// cents reads a {"val": n} Cent object. proto3 JSON drops zero values, so
// {} is 0.
func cents(m map[string]json.RawMessage, keys ...string) (float64, bool) {
	v, ok := pick(m, keys...)
	if !ok {
		return 0, false
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(v, &obj) != nil {
		return 0, false
	}
	if f, ok := num(obj, "val"); ok {
		return f, true
	}
	return 0, true
}
