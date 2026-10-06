package tui

import (
	"context"
	"fmt"
	"math"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Usage is the subscription allowance used this period, as the provider
// reports it (see provider.SubscriptionUsage).
type Usage struct {
	Percent  float64
	Label    string // "Weekly limit", "Monthly limit", or "Usage"
	ResetsAt time.Time
	Plan     string
}

// UsageFetch queries the provider once. ok=false means nothing to report.
type UsageFetch func(ctx context.Context) (u Usage, ok bool, err error)

const (
	usageWarnAt  = 75.0 // show the indicator (warning color)
	usageAlarmAt = 90.0 // error color
)

var (
	usageEvery   = 60 * time.Second // at most one automatic fetch per minute
	usageTimeout = 5 * time.Second
	// usageNow / usageLoc render the reset time; tests pin them.
	usageNow = time.Now
	usageLoc = func() *time.Location { return time.Local }
)

type usageMsg struct {
	u   Usage
	ok  bool
	err error
	gen int // provider generation; a switch drops stale answers
}

// usageLevel is 0 below the threshold (hidden), 1 warning, 2 alarm.
func usageLevel(pct float64) int {
	switch {
	case pct >= usageAlarmAt:
		return 2
	case pct >= usageWarnAt:
		return 1
	}
	return 0
}

func usageLabel(u Usage) string {
	if u.Label == "" {
		return "Usage"
	}
	return u.Label
}

// usagePct floors so 99.6% never reads as a full 100%.
func usagePct(p float64) string {
	return fmt.Sprintf("%.0f%%", math.Floor(math.Max(0, math.Min(100, p))))
}

// resetText is the local reset time: "Thu 11:47 AM" within the week,
// "Oct 8 11:47 AM" further out, "" when unknown or already past.
func resetText(at, now time.Time, loc *time.Location) string {
	if at.IsZero() || !at.After(now) {
		return ""
	}
	at = at.In(loc)
	if at.Sub(now) < 6*24*time.Hour {
		return at.Format("Mon 3:04 PM")
	}
	return at.Format("Jan 2 3:04 PM")
}

// usageTexts is the indicator, longest first; the status bar uses the
// first that fits. Empty below the threshold.
func usageTexts(u *Usage, now time.Time, loc *time.Location) []string {
	if u == nil || usageLevel(u.Percent) == 0 {
		return nil
	}
	head := usageLabel(*u) + " " + usagePct(u.Percent)
	if u.Percent >= 100 {
		head = usageLabel(*u) + " reached"
	}
	out := []string{}
	if r := resetText(u.ResetsAt, now, loc); r != "" {
		out = append(out, head+" · resets "+r)
	}
	return append(out, head)
}

// usageDetail is the /usage line, shown whatever the percentage.
func usageDetail(u Usage, now time.Time, loc *time.Location) string {
	s := usageLabel(u) + " " + usagePct(u.Percent) + " used"
	if u.Percent >= 100 {
		s = usageLabel(u) + " reached"
	}
	if !u.ResetsAt.IsZero() {
		at := u.ResetsAt.In(loc)
		s += " · resets " + at.Format("Mon Jan 2 3:04 PM")
	}
	if u.Plan != "" {
		s += " · " + u.Plan
	}
	return s
}

// fetchUsage starts a background usage query. Automatic fetches are
// throttled to one per usageEvery; force (startup, /usage) skips that.
// The query never blocks the UI or a turn and gives up after usageTimeout.
func (m *Model) fetchUsage(force, show bool) tea.Cmd {
	var fetch UsageFetch
	if m.deps.Usage != nil {
		fetch = m.deps.Usage()
	}
	if fetch == nil {
		if show {
			m.status, m.alert = "usage: "+m.deps.Provider+" does not report a subscription limit", false
		}
		return nil
	}
	if show {
		m.usageShow = true
		m.status, m.alert = "checking usage…", false
	}
	now := usageNow()
	if m.usageBusy || (!force && !m.usageAt.IsZero() && now.Sub(m.usageAt) < usageEvery) {
		return nil
	}
	m.usageBusy, m.usageAt = true, now
	timeout, gen := usageTimeout, m.usageGen
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		type res struct {
			u   Usage
			ok  bool
			err error
		}
		ch := make(chan res, 1)
		go func() {
			u, ok, err := fetch(ctx)
			ch <- res{u, ok, err}
		}()
		select {
		case r := <-ch:
			return usageMsg{u: r.u, ok: r.ok, err: r.err, gen: gen}
		case <-ctx.Done():
			return usageMsg{err: fmt.Errorf("usage query timed out after %s", timeout), gen: gen}
		}
	}
}

// resetUsage forgets usage after a provider switch.
func (m *Model) resetUsage() {
	m.usage, m.usageAt, m.usageBusy = nil, time.Time{}, false
	m.usageGen++
}

func (m *Model) onUsage(msg usageMsg) {
	if msg.gen != m.usageGen {
		return
	}
	m.usageBusy = false
	switch {
	case msg.err != nil:
		m.log("warn", "tui usage error", "err", oneLineErr(msg.err))
		m.usage = nil
	case !msg.ok:
		m.log("info", "tui usage none")
		m.usage = nil
	default:
		u := msg.u
		m.usage = &u
		m.log("info", "tui usage", "pct", fmt.Sprintf("%.1f", u.Percent), "label", usageLabel(u), "resets", u.ResetsAt.Format(time.RFC3339))
	}
	if !m.usageShow {
		return
	}
	m.usageShow = false
	switch {
	case msg.err != nil:
		m.status, m.alert = "usage: "+oneLineErr(msg.err), false
	case !msg.ok:
		m.status, m.alert = "usage: "+m.deps.Provider+" did not report a subscription limit", false
	default:
		m.status, m.alert = usageDetail(msg.u, usageNow(), usageLoc()), false
	}
}
