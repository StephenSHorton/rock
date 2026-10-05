package tui

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/StephenSHorton/rock/internal/perms"
)

func TestComposerAndStatusBottomRender(t *testing.T) {
	m := sized(t, 100, 24)
	m.deps.JevMode = "live"
	m.deps.CWD = os.Getenv("HOME")
	if m.deps.CWD == "" {
		m.deps.CWD = "/home/user"
	}
	m.status = "ready"
	m.deps.Mode = perms.ModeDefault
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})

	rows := strings.Split(ansi.Strip(m.View().Content), "\n")
	g := m.geo
	if g.composerRows != 1 || g.statusY()-g.composerY() != 3 {
		t.Fatalf("resting composer should be 3 lines (frame+row): %+v", g)
	}
	band := rows[g.composerY() : g.statusY()+1]
	got := strings.Join(band, "\n")
	t.Logf("after render:\n%s", got)

	top, mid, bot, status := band[0], band[1], band[2], band[3]
	if !strings.Contains(top, "╭") || !strings.Contains(top, "gpt-4o-mini") || !strings.Contains(top, "╮") {
		t.Fatalf("top border should carry the model: %q", top)
	}
	if strings.Contains(top, "default") {
		t.Fatalf("default mode on top border: %q", top)
	}
	if !strings.Contains(mid, "❯") || !strings.Contains(mid, "Ask Rock") {
		t.Fatalf("input row: %q", mid)
	}
	if !strings.Contains(bot, "╰") || strings.Contains(bot, "gpt-4o-mini") {
		t.Fatalf("bottom border should be plain: %q", bot)
	}
	if strings.Contains(status, "DEFAULT") || strings.Contains(status, "jev:") || strings.Contains(status, "gpt-4o-mini") || strings.Contains(status, "ready") {
		t.Fatalf("lean status should drop default/jev-live/model/ready: %q", status)
	}
	if !strings.Contains(status, "ctx") || !strings.Contains(status, "0%") {
		t.Fatalf("status should keep cwd/ctx: %q", status)
	}

	m.deps.Mode = perms.ModePlan
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	status = strings.Split(ansi.Strip(m.View().Content), "\n")[m.geo.statusY()]
	if !strings.Contains(status, "plan mode") {
		t.Fatalf("plan mode missing: %q", status)
	}
}
