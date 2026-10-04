package tui

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/x/ansi"

	"github.com/StephenSHorton/rock/internal/perms"
)

const sandboxNote = "Permissions are not a sandbox. Plan mode blocks every shell command because redirections are not inspected."

func (m *Model) layout() {
	w, h := max(m.width, 20), max(m.height, 6)
	g := geometry{w: w, h: h, composerRows: 3, infoRows: 1, helpRows: 1}
	g.compact = h <= compactAt
	if !g.compact && h > shortAt {
		g.padT, g.padB, g.padL, g.padR = 1, 1, 1, 1
	}
	g.innerW = max(1, w-g.padL-g.padR)
	g.bodyY = g.padT
	chrome := func() int {
		return g.padT + 1 + g.composerRows + g.infoRows + g.helpRows + g.padB
	}
	for g.composerRows > 1 && h-chrome() < 4 {
		g.composerRows--
	}
	if g.infoRows > 0 && h-chrome() < 3 {
		g.infoRows = 0
	}
	if h-chrome() < 1 {
		g.helpRows = 0
	}
	g.bodyH = max(1, h-chrome())
	g.transcriptW = g.innerW
	switch {
	case m.wantPlan() && g.innerW >= splitMin:
		g.planW = min(max(g.innerW*32/100, 28), 46)
		g.transcriptW = g.innerW - g.planW - 1
	case m.wantPlan():
		inner := max(1, g.innerW-4)
		m.planContent(inner)
		want := 2 + lipgloss.Height(m.planHeader(inner, true)) + m.planVP.TotalLineCount()
		g.planH = min(max(want, 5), max(5, g.bodyH/2))
	}
	if m.pending != nil {
		g.askH = min(m.askHeight(g.innerW), g.bodyH)
	}
	if g.planH > 0 && g.bodyH-g.askH-g.planH < 3 {
		g.planH = 0
	}
	g.vpY = g.bodyY + g.planH
	g.vpH = max(0, g.bodyH-g.planH-g.askH)
	g.barX = g.padL + g.transcriptW - 1
	m.geo = g
	m.input.SetWidth(max(1, g.innerW-2))
	m.input.SetHeight(g.composerRows)
	m.help.SetWidth(max(1, g.innerW-1))
	m.meter.SetWidth(meterWidth(g.innerW))
	contentW := max(1, g.transcriptW-2)
	m.vp.SetWidth(contentW)
	m.vp.SetHeight(g.vpH)
	if contentW != m.contentW {
		m.contentW = contentW
		m.syncView()
	} else if m.follow {
		m.vp.GotoBottom()
	} else {
		m.vp.SetYOffset(m.vp.YOffset())
	}
	if pw, ph, stacked := m.planBox(); pw > 0 {
		inner := max(1, pw-4)
		m.planContent(inner)
		headH := lipgloss.Height(m.planHeader(inner, stacked))
		footH := 0
		if !stacked {
			footH = lipgloss.Height(m.planFooter(inner))
		}
		m.planVP.SetWidth(inner)
		m.planVP.SetHeight(max(0, ph-2-headH-footH))
	}
	innerW, innerH := max(1, g.innerW-4), max(1, g.bodyH-2)
	listH := max(1, innerH-2)
	m.sessions.SetSize(innerW, listH)
	m.palette.SetSize(innerW, listH)
	m.perms.SetSize(innerW, max(1, listH-lipgloss.Height(wrapText(sandboxNote, innerW))-1))
	m.agents.SetColumns(agentColumns(innerW))
	m.agents.SetWidth(innerW)
	m.agents.SetHeight(max(2, listH))
	m.sheet.SetWidth(max(1, innerW-2))
	m.sheet.SetHeight(listH)
	if m.overlay == helpOverlay {
		m.sheet.SetContent(m.helpText(max(1, innerW-2)))
	}
	m.choices.SetSize(innerW, 2)
}
