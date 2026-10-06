package grokcli

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/StephenSHorton/rock/internal/provider"
)

// realBilling is grok 1.0.41's _x.ai/billing response captured on
// Stephen's PC (full JSON-RPC envelope; billingPeriod* filled in).
func realBilling(t *testing.T) json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile("testdata/billing_1.0.41.json")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func resultOf(t *testing.T, envelope json.RawMessage) json.RawMessage {
	t.Helper()
	var env struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(envelope, &env); err != nil || env.Result == nil {
		t.Fatalf("envelope: %v", err)
	}
	return env.Result
}

func TestParseBillingRealResponse(t *testing.T) {
	end := time.Date(2026, 10, 8, 17, 47, 3, 8589000, time.UTC)
	for name, raw := range map[string]json.RawMessage{
		"wrapped (JSON-RPC envelope)": realBilling(t),
		"bare result":                 resultOf(t, realBilling(t)),
	} {
		u, ok := ParseBilling(raw)
		if !ok {
			t.Fatalf("%s: not ok", name)
		}
		if u.Percent != 96 || u.Label != "Weekly limit" || u.Plan != "SuperGrok Heavy" || !u.ResetsAt.Equal(end) {
			t.Fatalf("%s: %+v", name, u)
		}
	}
}

func TestParseBillingTolerance(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		ok    bool
		pct   float64
		label string
		plan  string
		reset bool
	}{
		{"config only", `{"creditUsagePercent":80,"currentPeriod":{"type":"USAGE_PERIOD_TYPE_MONTHLY","end":"2026-11-01T00:00:00Z"}}`, true, 80, "Monthly limit", "", true},
		{"snake case", `{"config":{"credit_usage_percent":"77.5","current_period":{"type":"USAGE_PERIOD_TYPE_WEEKLY"}},"subscriptionTier":"SuperGrok"}`, true, 77.5, "Weekly limit", "SuperGrok", false},
		{"no period", `{"config":{"creditUsagePercent":12}}`, true, 12, "Usage", "", false},
		{"legacy cents", `{"config":{"monthlyLimit":{"val":5000},"used":{"val":4600},"billingPeriodEnd":"2026-10-31T00:00:00Z"}}`, true, 92, "Usage", "", true},
		{"legacy zero used ({})", `{"config":{"monthlyLimit":{"val":5000},"used":{}}}`, true, 0, "Usage", "", false},
		{"over 100 clamps", `{"config":{"creditUsagePercent":130}}`, true, 100, "Usage", "", false},
		{"bad end ignored", `{"config":{"creditUsagePercent":90,"currentPeriod":{"end":"soon"}}}`, true, 90, "Usage", "", false},
		{"no usage", `{"config":{"onDemandCap":{"val":0}},"subscription_tier":"SuperGrok Heavy"}`, false, 0, "", "", false},
		{"null config", `{"config":null}`, false, 0, "", "", false},
		{"empty", `{}`, false, 0, "", "", false},
		{"not json", `nope`, false, 0, "", "", false},
		{"array", `[1,2]`, false, 0, "", "", false},
	}
	for _, c := range cases {
		u, ok := ParseBilling(json.RawMessage(c.raw))
		if ok != c.ok {
			t.Fatalf("%s: ok=%v want %v (%+v)", c.name, ok, c.ok, u)
		}
		if !ok {
			continue
		}
		if u.Percent != c.pct || u.Label != c.label || u.Plan != c.plan || u.ResetsAt.IsZero() == c.reset {
			t.Fatalf("%s: %+v", c.name, u)
		}
	}
}

func TestSubscriptionUsageNeverSpawnsAChild(t *testing.T) {
	fake := &FakeScript{Reply: "pong", Billing: resultOf(t, realBilling(t))}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake)}
	defer p.Close()
	_, ok, err := p.SubscriptionUsage(context.Background())
	if ok || err != nil || p.StartCount() != 0 || fake.BillingCalls() != 0 {
		t.Fatalf("no child yet: ok=%v err=%v starts=%d calls=%d", ok, err, p.StartCount(), fake.BillingCalls())
	}
}

func TestSubscriptionUsageAsksTheLiveChild(t *testing.T) {
	fake := &FakeScript{Reply: "pong", Billing: resultOf(t, realBilling(t))}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake)}
	defer p.Close()
	if _, _, err := p.Models(context.Background()); err != nil {
		t.Fatal(err)
	}
	u, ok, err := p.SubscriptionUsage(context.Background())
	if err != nil || !ok || u.Percent != 96 || u.Label != "Weekly limit" {
		t.Fatalf("usage %+v ok=%v err=%v", u, ok, err)
	}
	if fake.BillingCalls() != 1 || p.StartCount() != 1 {
		t.Fatalf("calls=%d starts=%d", fake.BillingCalls(), p.StartCount())
	}
}

func TestSubscriptionUsageErrorsAreReturnedNotFatal(t *testing.T) {
	fake := &FakeScript{Reply: "pong"} // Billing nil: Method not found
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake)}
	defer p.Close()
	if _, _, err := p.Models(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, ok, err := p.SubscriptionUsage(context.Background())
	if ok || err == nil || !strings.Contains(strings.ToLower(err.Error()), "method not found") {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	// The child is still usable.
	msg, err := p.Complete(context.Background(), "", []provider.Message{{Role: provider.RoleUser, Content: "ping"}}, nil)
	if err != nil || msg.Content != "pong" {
		t.Fatalf("after billing error: %+v %v", msg, err)
	}
}

func TestSubscriptionUsageTimesOut(t *testing.T) {
	old := BillingTimeout
	BillingTimeout = 100 * time.Millisecond
	defer func() { BillingTimeout = old }()
	fake := &FakeScript{Reply: "pong", Billing: resultOf(t, realBilling(t)), BillingDelay: 2 * time.Second}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake)}
	defer p.Close()
	if _, _, err := p.Models(context.Background()); err != nil {
		t.Fatal(err)
	}
	at := time.Now()
	_, ok, err := p.SubscriptionUsage(context.Background())
	if ok || err == nil || time.Since(at) > time.Second {
		t.Fatalf("ok=%v err=%v after %s", ok, err, time.Since(at))
	}
}

// A billing query running beside a streaming prompt must not steal the
// prompt's activity signals (its idle timer) or its reply.
func TestSubscriptionUsageDuringAPrompt(t *testing.T) {
	fake := &FakeScript{
		Reply:       "a fairly long streamed reply for the idle timer",
		ReplyChunks: 8,
		ChunkDelay:  60 * time.Millisecond,
		Billing:     resultOf(t, realBilling(t)),
	}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake), PromptIdle: 300 * time.Millisecond}
	defer p.Close()
	if _, _, err := p.Models(context.Background()); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var msg provider.Message
	var perr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		msg, perr = p.Complete(context.Background(), "", []provider.Message{{Role: provider.RoleUser, Content: "go"}}, nil)
	}()
	for i := 0; i < 5; i++ {
		time.Sleep(50 * time.Millisecond)
		if _, ok, err := p.SubscriptionUsage(context.Background()); !ok || err != nil {
			t.Fatalf("billing during prompt: ok=%v err=%v", ok, err)
		}
	}
	wg.Wait()
	if perr != nil || msg.Content != fake.Reply {
		t.Fatalf("prompt: %q %v", msg.Content, perr)
	}
}
