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

// palette is site/styles.css. Dark values are the site's own. Light values
// keep each hue and hold at least 5:1 contrast on a pale background.
var palette = struct {
	Ground, Panel, Copper, Hot, Text, Mute adaptive
}{
	Ground: adaptive{Light: "#fbf7f2", Dark: "#0c0b0a"},
	Panel:  adaptive{Light: "#efe6db", Dark: "#161311"},
	Copper: adaptive{Light: "#a4561b", Dark: "#e7a15a"},
	Hot:    adaptive{Light: "#c43d0f", Dark: "#ff7a3c"},
	Text:   adaptive{Light: "#1a1008", Dark: "#f4ece3"},
	Mute:   adaptive{Light: "#6e6154", Dark: "#9c8f82"},
}

// theme is every style the screen draws with, resolved for one background.
type theme struct {
	dark bool

	ground, panel, copper, hot, text, mute color.Color

	plain      lipgloss.Style
	strong     lipgloss.Style
	faint      lipgloss.Style
	accent     lipgloss.Style
	accentBold lipgloss.Style
	alarm      lipgloss.Style
	alarmBold  lipgloss.Style

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
		dark:   dark,
		ground: pick(palette.Ground),
		panel:  pick(palette.Panel),
		copper: pick(palette.Copper),
		hot:    pick(palette.Hot),
		text:   pick(palette.Text),
		mute:   pick(palette.Mute),
	}
	s := lipgloss.NewStyle()
	t.plain = s.Foreground(t.text)
	t.strong = s.Foreground(t.text).Bold(true)
	t.faint = s.Foreground(t.mute)
	t.accent = s.Foreground(t.copper)
	t.accentBold = s.Foreground(t.copper).Bold(true)
	t.alarm = s.Foreground(t.hot)
	t.alarmBold = s.Foreground(t.hot).Bold(true)

	t.bar = s.Background(t.panel)
	t.barBrand = t.bar.Foreground(t.copper).Bold(true)
	t.barText = t.bar.Foreground(t.text)
	t.barMute = t.bar.Foreground(t.mute)

	t.you = s.Foreground(t.text).Bold(true)
	t.rock = s.Foreground(t.copper).Bold(true)

	badge := s.Bold(true).Padding(0, 1)
	t.badgeDefault = badge.Foreground(t.text).Background(t.panel)
	t.badgePlan = badge.Foreground(t.ground).Background(t.copper)
	t.badgeYolo = badge.Foreground(t.ground).Background(t.hot)
	t.badgeReview = badge.Foreground(t.copper).Background(t.panel)

	border := s.Border(lipgloss.RoundedBorder()).Padding(0, 1)
	t.box = border.BorderForeground(t.mute)
	t.boxFocus = border.BorderForeground(t.copper)
	t.boxAlert = border.BorderForeground(t.hot)

	t.track = s.Foreground(t.panel)
	t.thumb = s.Foreground(t.copper)
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
	s.Cursor.Color = t.copper
	return s
}

func (t theme) keyHelp() help.Styles {
	return help.Styles{
		Ellipsis:       t.faint,
		ShortKey:       t.strong,
		ShortDesc:      t.faint,
		ShortSeparator: t.faint,
		FullKey:        t.accent,
		FullDesc:       t.plain,
		FullSeparator:  t.faint,
	}
}

func (t theme) table() table.Styles {
	return table.Styles{
		Header: lipgloss.NewStyle().Bold(true).Foreground(t.copper).Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(t.mute),
		Cell:     lipgloss.NewStyle().Foreground(t.text).Padding(0, 1),
		Selected: lipgloss.NewStyle().Bold(true).Foreground(t.copper),
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

	ground, panel, copper := hex(palette.Ground), hex(palette.Panel), hex(palette.Copper)
	text, mute := hex(palette.Text), hex(palette.Mute)

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
			StylePrimitive: gansi.StylePrimitive{BlockSuffix: "\n", Color: copper, Bold: on()},
		},
		H1: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{Prefix: " ", Suffix: " ", Color: ground, BackgroundColor: copper, Bold: on()},
		},
		H2: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Prefix: "▍ "}},
		H3: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Prefix: "· "}},
		H4: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: text}},
		H5: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: text, Bold: off()}},
		H6: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: mute, Bold: off()}},

		Strikethrough:  gansi.StylePrimitive{CrossedOut: on()},
		Emph:           gansi.StylePrimitive{Italic: on()},
		Strong:         gansi.StylePrimitive{Bold: on()},
		HorizontalRule: gansi.StylePrimitive{Color: mute, Format: "\n────────\n"},

		Item:        gansi.StylePrimitive{BlockPrefix: "• "},
		Enumeration: gansi.StylePrimitive{BlockPrefix: ". "},
		Task:        gansi.StyleTask{Ticked: "[✓] ", Unticked: "[ ] "},

		Link:      gansi.StylePrimitive{Color: copper, Underline: on()},
		LinkText:  gansi.StylePrimitive{Color: copper, Bold: on()},
		Image:     gansi.StylePrimitive{Color: copper, Underline: on()},
		ImageText: gansi.StylePrimitive{Color: mute, Format: "Image: {{.text}} →"},

		Code: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{Prefix: " ", Suffix: " ", Color: copper, BackgroundColor: panel},
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
	copper, hot := palette.Copper.pick(dark), palette.Hot.pick(dark)
	text, mute := palette.Text.pick(dark), palette.Mute.pick(dark)
	return chroma.StyleEntries{
		chroma.Text:                text,
		chroma.Error:               hot,
		chroma.Comment:             "italic " + mute,
		chroma.CommentPreproc:      hot,
		chroma.Keyword:             hot,
		chroma.KeywordType:         copper,
		chroma.Operator:            mute,
		chroma.Punctuation:         mute,
		chroma.Name:                text,
		chroma.NameBuiltin:         copper,
		chroma.NameTag:             copper,
		chroma.NameAttribute:       copper,
		chroma.NameClass:           "bold " + text,
		chroma.NameConstant:        copper,
		chroma.NameDecorator:       hot,
		chroma.NameFunction:        copper,
		chroma.LiteralNumber:       hot,
		chroma.LiteralString:       copper,
		chroma.LiteralStringEscape: hot,
		chroma.GenericDeleted:      hot,
		chroma.GenericEmph:         "italic",
		chroma.GenericInserted:     copper,
		chroma.GenericStrong:       "bold",
		chroma.GenericSubheading:   mute,
		chroma.Background:          text,
	}
}
