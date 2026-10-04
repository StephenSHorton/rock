package tui

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/StephenSHorton/rock/internal/harness"
	"github.com/StephenSHorton/rock/internal/perms"
)

// textRoles are the palette entries that paint glyphs. Light values are
// measured against white; dark values against the dark panel. 4.5:1 is
// WCAG AA for normal text. If a specified pair falls short, keep the
// hex and mark that case with knownException instead of shifting the color.
var textRoles = []struct {
	name           string
	pair           adaptive
	knownException string
}{
	{name: "brand", pair: palette.Brand},
	{name: "attention", pair: palette.Attention},
	{name: "trim", pair: palette.Trim},
	{name: "text", pair: palette.Text},
	{name: "muted", pair: palette.Muted},
	{name: "error", pair: palette.Error},
	{name: "diff-add", pair: palette.Add},
	{name: "diff-remove", pair: palette.Remove},
}

const (
	wcagMin     = 4.5
	lightGround = "#FFFFFF"
	darkGround  = "#1E2228"
)

func TestTextRoleContrast(t *testing.T) {
	for _, role := range textRoles {
		t.Run(role.name, func(t *testing.T) {
			light := contrastRatio(role.pair.Light, lightGround)
			dark := contrastRatio(role.pair.Dark, darkGround)
			t.Logf("%s light %s on %s = %.2f:1; dark %s on %s = %.2f:1",
				role.name, role.pair.Light, lightGround, light,
				role.pair.Dark, darkGround, dark)

			if role.knownException != "" {
				// Specified hex kept on purpose. Do not "fix" the color
				// to chase the ratio.
				t.Logf("known exception: %s", role.knownException)
				return
			}
			if light < wcagMin {
				t.Errorf("light %s on %s is %.2f:1, want at least %.1f:1",
					role.pair.Light, lightGround, light, wcagMin)
			}
			if dark < wcagMin {
				t.Errorf("dark %s on %s is %.2f:1, want at least %.1f:1",
					role.pair.Dark, darkGround, dark, wcagMin)
			}
		})
	}
}

func TestLightThemeNeverPaintsYellowText(t *testing.T) {
	m := sized(t, 100, 24)
	m.Update(tea.BackgroundColorMsg{Color: color.White})
	m.deps.Mode = perms.ModeYolo
	m.Update(eventMsg{harness.Event{Kind: harness.EvToolCall, Name: "edit_file", Text: `{"path":"greeting.txt","old":"helo","new":"hello"}`}})
	reply := make(chan perms.Decision, 1)
	m.Update(askMsg{tool: "edit_file", detail: "greeting.txt", reply: reply})
	raw := m.View().Content
	if strings.Contains(raw, "38;2;242;183;5") {
		t.Fatal("light theme painted #F2B705 as text; that yellow is badge fill only")
	}
	if !strings.Contains(raw, "38;2;138;106;0") {
		t.Fatal("light theme should use #8A6A00 for attention text")
	}
}

func TestPaletteHexes(t *testing.T) {
	want := []struct {
		name        string
		got         adaptive
		light, dark string
	}{
		{"brand", palette.Brand, "#2B59E8", "#7A9BFF"},
		{"attention", palette.Attention, "#8A6A00", "#F2B705"},
		{"trim", palette.Trim, "#3B4552", "#9AA4B2"},
		{"text", palette.Text, "#1C1F24", "#ECEAE4"},
		{"muted", palette.Muted, "#6B7280", "#8B93A1"},
		{"panel", palette.Panel, "#F7F5F0", "#1E2228"},
		{"error", palette.Error, "#C0362C", "#FF7A70"},
		{"diff-add", palette.Add, "#2E7D4F", "#6CCB8F"},
		{"diff-remove", palette.Remove, "#C0362C", "#FF7A70"},
	}
	for _, tc := range want {
		if tc.got.Light != tc.light || tc.got.Dark != tc.dark {
			t.Errorf("%s: got %s/%s, want %s/%s", tc.name, tc.got.Light, tc.got.Dark, tc.light, tc.dark)
		}
	}
	if attentionFill != "#F2B705" || ink != "#1C1F24" {
		t.Fatalf("attention badge fill %s ink %s", attentionFill, ink)
	}
}

func contrastRatio(a, b string) float64 {
	l1, l2 := relativeLuminance(a), relativeLuminance(b)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func relativeLuminance(hex string) float64 {
	r, g, b := parseHex(hex)
	return 0.2126*srgbLin(r) + 0.7152*srgbLin(g) + 0.0722*srgbLin(b)
}

func srgbLin(c uint8) float64 {
	s := float64(c) / 255
	if s <= 0.04045 {
		return s / 12.92
	}
	return math.Pow((s+0.055)/1.055, 2.4)
}

func parseHex(hex string) (r, g, b uint8) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		panic(fmt.Sprintf("hex color %q", hex))
	}
	n, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		panic(err)
	}
	return uint8(n >> 16), uint8(n >> 8), uint8(n)
}
