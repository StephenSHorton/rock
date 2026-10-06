package tui

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/StephenSHorton/rock/internal/grokcli"
	"github.com/StephenSHorton/rock/internal/provider"
)

// pinUsageClock fixes "now" at Mon Oct 5 2026 9:30 PM in America/Denver.
func pinUsageClock(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Skip("no tzdata:", err)
	}
	now := time.Date(2026, 10, 5, 21, 30, 0, 0, loc)
	oldNow, oldLoc := usageNow, usageLoc
	usageNow = func() time.Time { return now }
	usageLoc = func() *time.Location { return loc }
	t.Cleanup(func() { usageNow, usageLoc = oldNow, oldLoc })
	return loc
}

// The period end grok 1.0.41 reported: Thu Oct 8 17:47 UTC = 11:47 AM MDT.
var weeklyEnd = time.Date(2026, 10, 8, 17, 47, 3, 0, time.UTC)

func weekly(pct float64) *Usage {
	return &Usage{Percent: pct, Label: "Weekly limit", ResetsAt: weeklyEnd, Plan: "SuperGrok Heavy"}
}

func TestUsageThresholds(t *testing.T) {
	for pct, want := range map[float64]int{0: 0, 50: 0, 74.99: 0, 75: 1, 89.9: 1, 90: 2, 96: 2, 100: 2, 130: 2} {
		if got := usageLevel(pct); got != want {
			t.Fatalf("level(%v)=%d want %d", pct, got, want)
		}
	}
}

func TestUsageTexts(t *testing.T) {
	loc := pinUsageClock(t)
	now := usageNow()
	cases := []struct {
		u    *Usage
		want []string
	}{
		{nil, nil},
		{weekly(74.9), nil},
		{weekly(75), []string{"Weekly limit 75% · resets Thu 11:47 AM", "Weekly limit 75%"}},
		{weekly(96), []string{"Weekly limit 96% · resets Thu 11:47 AM", "Weekly limit 96%"}},
		{weekly(99.7), []string{"Weekly limit 99% · resets Thu 11:47 AM", "Weekly limit 99%"}},
		{weekly(100), []string{"Weekly limit reached · resets Thu 11:47 AM", "Weekly limit reached"}},
		{&Usage{Percent: 80, Label: "Monthly limit", ResetsAt: time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)}, []string{"Monthly limit 80% · resets Nov 1 12:00 AM", "Monthly limit 80%"}},
		{&Usage{Percent: 91}, []string{"Usage 91%"}},
		{&Usage{Percent: 91, ResetsAt: now.Add(-time.Hour)}, []string{"Usage 91%"}}, // stale reset dropped
	}
	for _, c := range cases {
		got := usageTexts(c.u, now, loc)
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Fatalf("%+v:\n got %q\nwant %q", c.u, got, c.want)
		}
	}
	if d := usageDetail(*weekly(42), now, loc); d != "Weekly limit 42% used · resets Thu Oct 8 11:47 AM · SuperGrok Heavy" {
		t.Fatalf("detail %q", d)
	}
}

// statusRow renders the status bar with usage u (raw, with ANSI).
func statusRow(t *testing.T, w int, u *Usage) (*Model, string) {
	t.Helper()
	m := sized(t, w, 24)
	m.usage = u
	raw := strings.Split(m.View().Content, "\n")
	return m, raw[m.geo.statusY()]
}

func TestUsageIndicatorRendersAt96Percent(t *testing.T) {
	pinUsageClock(t)
	m, raw := statusRow(t, 100, weekly(96))
	plain := ansi.Strip(raw)
	want := "Weekly limit 96% · resets Thu 11:47 AM"
	if !strings.Contains(plain, want) {
		t.Fatalf("status row lacks %q:\n%s", want, plain)
	}
	if !strings.HasSuffix(strings.TrimRight(plain, " "), want) {
		t.Fatalf("indicator should sit at the right edge:\n%q", plain)
	}
	if !strings.Contains(raw, m.th.danger.Render(want)) {
		t.Fatalf("96%% should use the error color:\n%q", raw)
	}
	_ = os.WriteFile(os.Getenv("ROCK_USAGE_RENDER"), []byte(strings.Join(screen(m), "\n")), 0o644)
	assertFrame(t, m, 100, 24)
}

func TestUsageIndicatorColorsAndHiding(t *testing.T) {
	pinUsageClock(t)
	m, raw := statusRow(t, 100, weekly(80))
	if want := "Weekly limit 80% · resets Thu 11:47 AM"; !strings.Contains(raw, m.th.alarm.Render(want)) {
		t.Fatalf("80%% should use the warning color:\n%q", raw)
	}
	_, raw = statusRow(t, 100, weekly(100))
	if !strings.Contains(ansi.Strip(raw), "Weekly limit reached") {
		t.Fatalf("100%%: %q", ansi.Strip(raw))
	}
	for _, u := range []*Usage{nil, weekly(74)} {
		_, raw = statusRow(t, 100, u)
		if strings.Contains(ansi.Strip(raw), "limit") {
			t.Fatalf("%+v should hide the indicator: %q", u, ansi.Strip(raw))
		}
	}
	// Narrow: drop the reset time first, then the whole indicator; never overflow.
	for _, w := range []int{60, 40, 24} {
		m, raw := statusRow(t, w, weekly(96))
		plain := ansi.Strip(raw)
		if ansi.StringWidth(plain) > w {
			t.Fatalf("%d cols: status overflows: %q", w, plain)
		}
		if w == 60 && !strings.Contains(plain, "Weekly limit 96%") {
			t.Fatalf("60 cols should keep the short form: %q", plain)
		}
		assertFrame(t, m, w, 24)
	}
}

func TestUsageSharesTheRowWithAStatus(t *testing.T) {
	pinUsageClock(t)
	m := sized(t, 120, 24)
	m.usage = weekly(96)
	m.status = "default mode"
	row := ansi.Strip(screen(m)[m.geo.statusY()])
	if !strings.Contains(row, "default mode │ Weekly limit 96%") {
		t.Fatalf("status + usage: %q", row)
	}
}

// fakeUsage counts fetches and returns u.
func fakeUsage(u Usage, ok bool, err error, calls *atomic.Int32) func() UsageFetch {
	return func() UsageFetch {
		return func(ctx context.Context) (Usage, bool, error) {
			calls.Add(1)
			return u, ok, err
		}
	}
}

func TestUsageFetchThrottle(t *testing.T) {
	pinUsageClock(t)
	var calls atomic.Int32
	m := sized(t, 100, 24)
	m.deps.Usage = fakeUsage(*weekly(96), true, nil, &calls)

	cmd := m.fetchUsage(true, false) // startup
	if cmd == nil {
		t.Fatal("startup fetch should run")
	}
	m.Update(cmd())
	if m.usage == nil || m.usage.Percent != 96 || calls.Load() != 1 {
		t.Fatalf("usage %+v calls %d", m.usage, calls.Load())
	}
	// A turn finishing inside the minute does not refetch.
	if c := m.fetchUsage(false, false); c != nil {
		t.Fatal("throttled fetch should be nil")
	}
	// After a minute it does.
	m.usageAt = m.usageAt.Add(-usageEvery - time.Second)
	c := m.fetchUsage(false, false)
	if c == nil {
		t.Fatal("fetch after 60s should run")
	}
	// While one is in flight, no second one starts.
	if m.fetchUsage(true, false) != nil {
		t.Fatal("no overlapping fetches")
	}
	m.Update(c())
	if calls.Load() != 2 {
		t.Fatalf("calls %d", calls.Load())
	}
}

func TestTurnDoneFetchesUsageThrottled(t *testing.T) {
	pinUsageClock(t)
	var calls atomic.Int32
	m := sized(t, 100, 24)
	m.deps.Usage = fakeUsage(*weekly(96), true, nil, &calls)
	m.usageAt = usageNow().Add(-2 * usageEvery)
	m.busy = true
	cmd := m.update(turnDone{})
	runAll(m, cmd)
	if calls.Load() != 1 || m.usage == nil {
		t.Fatalf("turn done should fetch: calls %d usage %+v", calls.Load(), m.usage)
	}
	m.busy = true
	runAll(m, m.update(turnDone{}))
	if calls.Load() != 1 {
		t.Fatalf("second turn inside a minute refetched: %d", calls.Load())
	}
}

// runAll executes cmd (expanding batches) and feeds usage answers back.
func runAll(m *Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			runAll(m, c)
		}
	case usageMsg:
		m.Update(msg)
	}
}

func TestUsageFailuresHideTheIndicator(t *testing.T) {
	pinUsageClock(t)
	var logged []string
	var calls atomic.Int32
	m := sized(t, 100, 24)
	m.deps.Log = func(level, msg string, kv ...any) { logged = append(logged, level+" "+msg) }
	m.usage = weekly(96)
	m.deps.Usage = fakeUsage(Usage{}, false, errors.New("Method not found"), &calls)
	m.Update(m.fetchUsage(true, false)())
	if m.usage != nil {
		t.Fatal("an error should hide the indicator")
	}
	if !strings.Contains(strings.Join(logged, "\n"), "warn tui usage error") {
		t.Fatalf("not logged: %v", logged)
	}
	if m.alert || strings.Contains(m.status, "Method") {
		t.Fatalf("automatic failures stay quiet: status %q alert %v", m.status, m.alert)
	}
	// A provider without usage reports nothing and no fetch runs.
	m.deps.Usage = func() UsageFetch { return nil }
	if m.fetchUsage(true, false) != nil {
		t.Fatal("nil fetcher should not run")
	}
}

func TestUsageTimeoutNeverHangs(t *testing.T) {
	pinUsageClock(t)
	old := usageTimeout
	usageTimeout = 50 * time.Millisecond
	defer func() { usageTimeout = old }()
	m := sized(t, 100, 24)
	block := make(chan struct{})
	defer close(block)
	m.deps.Usage = func() UsageFetch {
		return func(ctx context.Context) (Usage, bool, error) {
			<-block // ignores ctx on purpose
			return Usage{}, false, nil
		}
	}
	at := time.Now()
	msg := m.fetchUsage(true, false)()
	if time.Since(at) > time.Second {
		t.Fatalf("fetch took %s", time.Since(at))
	}
	m.Update(msg)
	if m.usage != nil || m.usageBusy {
		t.Fatalf("timeout: usage %+v busy %v", m.usage, m.usageBusy)
	}
}

func TestSlashUsageShowsNumbersBelowTheThreshold(t *testing.T) {
	pinUsageClock(t)
	var calls atomic.Int32
	m := sized(t, 160, 24)
	m.deps.Usage = fakeUsage(*weekly(42), true, nil, &calls)
	m.usageAt = usageNow() // throttled for automatic fetches
	m.input.SetValue("/usage")
	cmd := m.submit()
	if cmd == nil {
		t.Fatal("/usage should fetch")
	}
	runAll(m, cmd)
	if m.status != "Weekly limit 42% used · resets Thu Oct 8 11:47 AM · SuperGrok Heavy" {
		t.Fatalf("status %q", m.status)
	}
	if row := ansi.Strip(screen(m)[m.geo.statusY()]); !strings.Contains(row, "Weekly limit 42% used") {
		t.Fatalf("row %q", row)
	}
	// Below 75% the persistent indicator stays hidden.
	m.status = ""
	if row := ansi.Strip(screen(m)[m.geo.statusY()]); strings.Contains(row, "limit") {
		t.Fatalf("42%% indicator should be hidden: %q", row)
	}
	// Without a reporting provider /usage says so.
	m.deps.Usage = nil
	m.deps.Provider = "openai"
	m.input.SetValue("/usage")
	m.submit()
	if !strings.Contains(m.status, "does not report") {
		t.Fatalf("status %q", m.status)
	}
}

func TestUsageDroppedOnProviderSwitch(t *testing.T) {
	pinUsageClock(t)
	var calls atomic.Int32
	m := sized(t, 100, 24)
	m.deps.Usage = fakeUsage(*weekly(96), true, nil, &calls)
	cmd := m.fetchUsage(true, false)
	m.resetUsage() // e.g. /provider switched auth while the query ran
	m.Update(cmd())
	if m.usage != nil {
		t.Fatal("a stale answer from the old provider must be ignored")
	}
}

// End to end with the in-repo fake grok answering _x.ai/billing.
func TestUsageFromFakeGrok(t *testing.T) {
	pinUsageClock(t)
	fake := &grokcli.FakeScript{Reply: "pong", Billing: []byte(`{"config":{"creditUsagePercent":96.0,"currentPeriod":{"type":"USAGE_PERIOD_TYPE_WEEKLY","start":"2026-10-01T17:47:03.008589+00:00","end":"2026-10-08T17:47:03.008589+00:00"},"onDemandCap":{"val":0},"onDemandUsed":{"val":0},"prepaidBalance":{"val":0},"isUnifiedBillingUser":true},"subscription_tier":"SuperGrok Heavy"}`)}
	p := &grokcli.Provider{CWD: t.TempDir(), Start: grokcli.StartFake(fake)}
	defer p.Close()
	if _, _, err := p.Models(context.Background()); err != nil { // child up, as startTUI does
		t.Fatal(err)
	}
	var r provider.UsageReporter = p
	m := sized(t, 100, 24)
	m.deps.Usage = func() UsageFetch {
		return func(ctx context.Context) (Usage, bool, error) {
			u, ok, err := r.SubscriptionUsage(ctx)
			return Usage{Percent: u.Percent, Label: u.Label, ResetsAt: u.ResetsAt, Plan: u.Plan}, ok, err
		}
	}
	runAll(m, m.Init())
	if m.usage == nil || fake.BillingCalls() != 1 {
		t.Fatalf("init should fetch once: usage %+v calls %d", m.usage, fake.BillingCalls())
	}
	row := ansi.Strip(screen(m)[m.geo.statusY()])
	if !strings.Contains(row, "Weekly limit 96% · resets Thu 11:47 AM") {
		t.Fatalf("row %q", row)
	}
}
