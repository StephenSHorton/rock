package tui

import (
	"context"
	"fmt"
	"runtime"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/StephenSHorton/rock/internal/trace"
)

// stallAfter is how long a turn may go without any progress (a harness
// event, a stage line, a streamed chunk) before the watchdog writes
// WARN tui turn stall plus every goroutine stack to rock.log.
var (
	stallAfter = 20 * time.Second
	stallTick  = time.Second
)

// stallDumps caps stack dumps per turn; later stalls only log one line.
const stallDumps = 3

const maxStackBytes = 1 << 20

type stallMsg struct {
	stage string
	idle  time.Duration
}

// watchTurn runs beside a turn until ctx ends (turnDone cancels it). It
// never touches Model state; it only logs and posts stallMsg.
// after and tick are passed in (read on the Update goroutine) so tests can
// tune the package vars without racing a previous turn's watchdog.
func (m *Model) watchTurn(ctx context.Context, at time.Time, after, tick time.Duration) {
	t := time.NewTicker(tick)
	defer t.Stop()
	dumps := 0
	var lastWarn time.Time
	for {
		var now time.Time
		select {
		case <-ctx.Done():
			return
		case now = <-t.C:
		}
		stage, last := trace.Last()
		if last.Before(at) {
			last, stage = at, "turn start"
		}
		idle := now.Sub(last)
		if idle < after || (!lastWarn.IsZero() && now.Sub(lastWarn) < after) {
			continue
		}
		lastWarn = now
		m.log("warn", "tui turn stall", "idle_s", int(idle.Seconds()), "elapsed_s", int(now.Sub(at).Seconds()), "stage", stage, "dump", dumps < stallDumps)
		if dumps < stallDumps {
			dumps++
			m.log("warn", "tui goroutines", "stacks", allStacks())
		}
		if m.send != nil {
			m.send(stallMsg{stage: stage, idle: idle})
		}
	}
}

func allStacks() string {
	buf := make([]byte, 64<<10)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) || len(buf) >= maxStackBytes {
			return string(buf[:n])
		}
		buf = make([]byte, len(buf)*2)
	}
}

func (m *Model) onStall(msg stallMsg) tea.Cmd {
	if !m.busy {
		return nil
	}
	m.stall = fmt.Sprintf("no progress for %ds (%s)", int(msg.idle.Seconds()), msg.stage)
	return nil
}
