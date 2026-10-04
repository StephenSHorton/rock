package tui

import (
	"image/color"
	"sync"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
	gansi "github.com/charmbracelet/glamour/ansi"
)

// adaptive is one Rock color and its counterpart for light terminals: the
// shape Lip Gloss v1 called AdaptiveColor. Lip Gloss v2 moved that type to
// its compat package, which queries the terminal at package init for every
// rock subcommand. Here the pair is resolved with lipgloss.LightDark once
// Bubble Tea reports the background, so nothing reads stdin behind its back.
type adaptive struct{ Light, Dark string }

func (a adaptive) pick(dark bool) string {
	if dark {
		return a.Dark
	}
	return a.Light
}

// palette is the canonical Rock TUI palette. Light / dark pairs resolve
// through lipgloss.LightDark after Bubble Tea reports tea.BackgroundColorMsg
// (the Charm v2 stand-in for lipgloss.HasDarkBackground, which queries
// stdin and is only for standalone Lip Gloss). Neutral is the default.
// Azurite (Brand) is only for defining chrome: the header wordmark, pane
// titles, markdown headings, the prompt caret, the active border, the
// active selection, and the spinner. Links, body text, and idle borders
// use Trim, Muted, or Text. Signal yellow is attention only — permission
// asks, warnings, and the active/running state. On light terminals
// Attention is #8A6A00 as text; #F2B705 is only a badge fill under ink.
// Surfaces are filled only where a background is actually drawn;
// everything else keeps the terminal default.
var palette = struct {
	Brand, Attention, Trim, Text, Muted, Panel, Error, Add, Remove adaptive
}{
	Brand:     adaptive{Light: "#2B59E8", Dark: "#7A9BFF"},
	Attention: adaptive{Light: "#8A6A00", Dark: "#F2B705"},
	Trim:      adaptive{Light: "#3B4552", Dark: "#9AA4B2"},
	Text:      adaptive{Light: "#1C1F24", Dark: "#ECEAE4"},
	Muted:     adaptive{Light: "#6B7280", Dark: "#8B93A1"},
	Panel:     adaptive{Light: "#F7F5F0", Dark: "#1E2228"},
	Error:     adaptive{Light: "#C0362C", Dark: "#FF7A70"},
	Add:       adaptive{Light: "#2E7D4F", Dark: "#6CCB8F"},
	Remove:    adaptive{Light: "#C0362C", Dark: "#FF7A70"},
}

// attentionFill is the signal-yellow badge background. Light terminals
// pair it with ink (#1C1F24). Dark terminals use Attention as text on
// the panel instead of filling the badge with yellow.
const (
	attentionFill = "#F2B705"
	ink           = "#1C1F24"
)

// theme is every style the screen draws with, resolved for one background.
type theme struct {
	dark bool

	brand, attention, trim, text, muted, panel, fail, add, remove color.Color

	plain      lipgloss.Style
	strong     lipgloss.Style
	faint      lipgloss.Style
	chrome     lipgloss.Style
	accent     lipgloss.Style
	accentBold lipgloss.Style
	alarm      lipgloss.Style
	alarmBold  lipgloss.Style
	danger     lipgloss.Style
	dangerBold lipgloss.Style
	plus       lipgloss.Style
	minus      lipgloss.Style

	bar      lipgloss.Style
	barBrand lipgloss.Style
	barText  lipgloss.Style
	barMute  lipgloss.Style

	you  lipgloss.Style
	rock lipgloss.Style

	badgeDefault lipgloss.Style
	badgePlan    lipgloss.Style
	badgeYolo    lipgloss.Style
	badgeReview  lipgloss.Style

	box      lipgloss.Style
	boxFocus lipgloss.Style
	boxAlert lipgloss.Style

	track lipgloss.Style
	thumb lipgloss.Style
}

func newTheme(dark bool) theme {
	ld := lipgloss.LightDark(dark)
	pick := func(a adaptive) color.Color { return ld(lipgloss.Color(a.Light), lipgloss.Color(a.Dark)) }
	t := theme{
		dark:      dark,
		brand:     pick(palette.Brand),
		attention: pick(palette.Attention),
		trim:      pick(palette.Trim),
		text:      pick(palette.Text),
		muted:     pick(palette.Muted),
		panel:     pick(palette.Panel),
		fail:      pick(palette.Error),
		add:       pick(palette.Add),
		remove:    pick(palette.Remove),
	}
	s := lipgloss.NewStyle()
	t.plain = s.Foreground(t.text)
	t.strong = s.Foreground(t.text).Bold(true)
	t.faint = s.Foreground(t.muted)
	t.chrome = s.Foreground(t.trim)
	t.accent = s.Foreground(t.brand)
	t.accentBold = s.Foreground(t.brand).Bold(true)
	t.alarm = s.Foreground(t.attention)
	t.alarmBold = s.Foreground(t.attention).Bold(true)
	t.danger = s.Foreground(t.fail)
	t.dangerBold = s.Foreground(t.fail).Bold(true)
	t.plus = s.Foreground(t.add)
	t.minus = s.Foreground(t.remove)

	t.bar = s.Background(t.panel)
	t.barBrand = t.bar.Foreground(t.brand).Bold(true)
	t.barText = t.bar.Foreground(t.text)
	t.barMute = t.bar.Foreground(t.trim)

	t.you = s.Foreground(t.text).Bold(true)
	t.rock = s.Foreground(t.text).Bold(true)

	badge := s.Bold(true).Padding(0, 1)
	t.badgeDefault = badge.Foreground(t.text).Background(t.panel)
	t.badgePlan = badge.Foreground(t.panel).Background(t.brand)
	t.badgeYolo = badge.Foreground(ld(lipgloss.Color(ink), lipgloss.Color(attentionFill))).
		Background(ld(lipgloss.Color(attentionFill), t.panel))
	t.badgeReview = badge.Foreground(t.trim).Background(t.panel)

	border := s.Border(lipgloss.RoundedBorder()).Padding(0, 1)
	t.box = border.BorderForeground(t.trim)
	t.boxFocus = border.BorderForeground(t.brand)
	t.boxAlert = border.BorderForeground(t.attention)

	t.track = s.Foreground(t.panel)
	t.thumb = s.Foreground(t.trim)
	return t
}

func (t theme) composer() textarea.Styles {
	s := textarea.DefaultStyles(t.dark)
	focused := textarea.StyleState{
		Base:        lipgloss.NewStyle(),
		Text:        t.plain,
		Placeholder: t.faint,
		Prompt:      t.accentBold,
		CursorLine:  lipgloss.NewStyle(),
		EndOfBuffer: t.faint,
	}
	blurred := focused
	blurred.Text = t.faint
	blurred.Prompt = t.faint
	s.Focused, s.Blurred = focused, blurred
	s.Cursor.Color = t.brand
	return s
}

func (t theme) keyHelp() help.Styles {
	return help.Styles{
		Ellipsis:       t.faint,
		ShortKey:       t.strong,
		ShortDesc:      t.faint,
		ShortSeparator: t.faint,
		FullKey:        t.chrome,
		FullDesc:       t.plain,
		FullSeparator:  t.faint,
	}
}

func (t theme) table() table.Styles {
	return table.Styles{
		Header: lipgloss.NewStyle().Bold(true).Foreground(t.brand).Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(t.trim),
		Cell:     lipgloss.NewStyle().Foreground(t.text).Padding(0, 1),
		Selected: lipgloss.NewStyle().Bold(true).Foreground(t.brand),
	}
}

func (t theme) badge(mode string) lipgloss.Style {
	switch mode {
	case "plan":
		return t.badgePlan
	case "yolo":
		return t.badgeYolo
	default:
		return t.badgeDefault
	}
}

// markdown is the Glamour style for assistant text and the plan pane. It
// is built from the same palette so markdown never brings its own colors.
func (t theme) markdown() gansi.StyleConfig {
	hex := func(a adaptive) *string { v := a.pick(t.dark); return &v }
	str := func(v string) *string { return &v }
	on := func() *bool { v := true; return &v }
	off := func() *bool { v := false; return &v }
	num := func(v uint) *uint { return &v }

	brand, panel := hex(palette.Brand), hex(palette.Panel)
	text, mute, trim := hex(palette.Text), hex(palette.Muted), hex(palette.Trim)
	// Headings keep Brand. Links and body chrome stay trim/text so azurite
	// does not wash the transcript.

	return gansi.StyleConfig{
		Document: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{Color: text},
			Margin:         num(0),
		},
		BlockQuote: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{Color: mute, Italic: on()},
			Indent:         num(1),
			IndentToken:    str("│ "),
		},
		List: gansi.StyleList{LevelIndent: 2},
		Heading: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{BlockSuffix: "\n", Color: brand, Bold: on()},
		},
		H1: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{Prefix: " ", Suffix: " ", Color: panel, BackgroundColor: brand, Bold: on()},
		},
		H2: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Prefix: "▍ "}},
		H3: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Prefix: "· "}},
		H4: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: text}},
		H5: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: text, Bold: off()}},
		H6: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: mute, Bold: off()}},

		Strikethrough:  gansi.StylePrimitive{CrossedOut: on()},
		Emph:           gansi.StylePrimitive{Italic: on()},
		Strong:         gansi.StylePrimitive{Bold: on()},
		HorizontalRule: gansi.StylePrimitive{Color: trim, Format: "\n────────\n"},

		Item:        gansi.StylePrimitive{BlockPrefix: "• "},
		Enumeration: gansi.StylePrimitive{BlockPrefix: ". "},
		Task:        gansi.StyleTask{Ticked: "[✓] ", Unticked: "[ ] "},

		Link:      gansi.StylePrimitive{Color: trim, Underline: on()},
		LinkText:  gansi.StylePrimitive{Color: trim, Bold: on()},
		Image:     gansi.StylePrimitive{Color: trim, Underline: on()},
		ImageText: gansi.StylePrimitive{Color: mute, Format: "Image: {{.text}} →"},

		Code: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{Prefix: " ", Suffix: " ", Color: text, BackgroundColor: panel},
		},
		CodeBlock: gansi.StyleCodeBlock{
			StyleBlock: gansi.StyleBlock{
				StylePrimitive: gansi.StylePrimitive{Color: text},
				Margin:         num(1),
			},
			Theme: codeTheme(t.dark),
		},
		Table: gansi.StyleTable{
			StyleBlock:      gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: text}},
			CenterSeparator: str("┼"),
			ColumnSeparator: str("│"),
			RowSeparator:    str("─"),
		},
		DefinitionDescription: gansi.StylePrimitive{BlockPrefix: "\n› "},
	}
}

var registerCodeThemes sync.Once

// codeTheme names the Chroma style for code blocks. Glamour registers an
// inline Chroma config once per process under one shared name, so a light
// and a dark config cannot both live there. Rock registers its own pair.
func codeTheme(dark bool) string {
	registerCodeThemes.Do(func() {
		for _, d := range []bool{true, false} {
			chromastyles.Register(chroma.MustNewStyle(codeThemeName(d), codeEntries(d)))
		}
	})
	return codeThemeName(dark)
}

func codeThemeName(dark bool) string {
	if dark {
		return "rock-dark"
	}
	return "rock-light"
}

func codeEntries(dark bool) chroma.StyleEntries {
	text, mute, trim := palette.Text.pick(dark), palette.Muted.pick(dark), palette.Trim.pick(dark)
	fail, plus, minus := palette.Error.pick(dark), palette.Add.pick(dark), palette.Remove.pick(dark)
	return chroma.StyleEntries{
		chroma.Text:                text,
		chroma.Error:               fail,
		chroma.Comment:             "italic " + mute,
		chroma.CommentPreproc:      trim,
		chroma.Keyword:             "bold " + text,
		chroma.KeywordType:         trim,
		chroma.Operator:            mute,
		chroma.Punctuation:         mute,
		chroma.Name:                text,
		chroma.NameBuiltin:         trim,
		chroma.NameTag:             trim,
		chroma.NameAttribute:       trim,
		chroma.NameClass:           "bold " + text,
		chroma.NameConstant:        trim,
		chroma.NameDecorator:       trim,
		chroma.NameFunction:        text,
		chroma.LiteralNumber:       trim,
		chroma.LiteralString:       text,
		chroma.LiteralStringEscape: trim,
		chroma.GenericDeleted:      minus,
		chroma.GenericEmph:         "italic",
		chroma.GenericInserted:     plus,
		chroma.GenericStrong:       "bold",
		chroma.GenericSubheading:   mute,
		chroma.Background:          text,
	}
}
