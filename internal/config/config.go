// Package config loads TOML. API keys stay in the environment. Project allow
// rules apply only after the folder is trusted. Deny rules always apply.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/pelletier/go-toml/v2"
)

type File struct {
	Model       string               `toml:"model"`
	FastModel   string               `toml:"fast_model"`
	StrongModel string               `toml:"strong_model"`
	BaseURL     string               `toml:"base_url"`
	Mode        string               `toml:"mode"`
	MaxSteps    int                  `toml:"max_steps"`
	Permissions Perms                `toml:"permissions"`
	Jev         Jev                  `toml:"jev"`
	Git         Git                  `toml:"git"`
	MCP         map[string]MCPServer `toml:"mcp"`
}

type Perms struct {
	Allow []string `toml:"allow"`
	Ask   []string `toml:"ask"`
	Deny  []string `toml:"deny"`
}

type Jev struct {
	Enabled          *bool   `toml:"enabled"`
	BaseURL          string  `toml:"base_url"`
	Model            string  `toml:"model"`
	MinConfidence    float64 `toml:"min_confidence"`
	RiskBlock        float64 `toml:"risk_block"`
	AllowDestructive bool    `toml:"allow_destructive"`
	// Nudge is the soft validation hint after edit/write and allowed
	// shell. nil means on (the product default). false or nudge_every
	// < 0 disables. The hint is text; it never calls Jev.
	Nudge      *bool `toml:"nudge"`
	NudgeEvery int   `toml:"nudge_every"`
	// Triage attaches a Jev classification to a failed shell (default on).
	// Filter runs KeepSnippet / grep through the same Ask primitive (default on).
	Triage    *bool `toml:"triage"`
	Filter    *bool `toml:"filter"`
	ClipBytes int   `toml:"clip_bytes"`
}

// LiveAllowed is false only when jev.enabled is explicitly false.
func (j Jev) LiveAllowed() bool {
	if j.Enabled != nil {
		return *j.Enabled
	}
	return true
}

func (j Jev) TriageOn() bool {
	if j.Triage != nil {
		return *j.Triage
	}
	return true
}

func (j Jev) FilterOn() bool {
	if j.Filter != nil {
		return *j.Filter
	}
	return true
}

func (j Jev) ClipSize() int {
	if j.ClipBytes <= 0 {
		return 1500
	}
	if j.ClipBytes > 8000 {
		return 8000
	}
	return j.ClipBytes
}

// NudgeInterval is how often the harness appends a validation hint.
// 2 is the default: first qualifying event, then every second one.
// -1 means off.
func (j Jev) NudgeInterval() int {
	if j.Nudge != nil && !*j.Nudge {
		return -1
	}
	if j.NudgeEvery < 0 {
		return -1
	}
	if j.NudgeEvery == 0 {
		return 2
	}
	return j.NudgeEvery
}

type Git struct {
	Checkpoint bool `toml:"checkpoint"`
	ReviewOnly bool `toml:"review_only"`
}

type MCPServer struct {
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
	Enabled bool     `toml:"enabled"`
}

type Loaded struct {
	File    File
	Path    string
	CWD     string
	Trusted bool
	Ignored []string
	Policy  perms.Policy
}

func Default() File {
	return File{
		Model:       "gpt-4o-mini",
		FastModel:   "gpt-4o-mini",
		StrongModel: "gpt-4o",
		BaseURL:     "https://api.openai.com/v1",
		Mode:        "default",
		MaxSteps:    12,
		Permissions: Perms{
			Allow: []string{"read_file", "grep", "glob", "web_fetch", "ask_jev", "update_plan"},
			Ask:   []string{"edit_file", "write_file", "shell", "spawn_subagent"},
		},
		Jev: Jev{MinConfidence: 0.55, RiskBlock: 0.72},
	}
}

func ConfigPath() string {
	if p := os.Getenv("ROCK_CONFIG"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "config.toml"
	}
	return filepath.Join(dir, "rock", "config.toml")
}

func Load(cwd string) (Loaded, error) {
	file := Default()
	path := ConfigPath()
	if raw, err := os.ReadFile(path); err == nil {
		if err := toml.Unmarshal(raw, &file); err != nil {
			return Loaded{}, err
		}
	}
	trusted := trusted(cwd)
	var ignored []string
	project := filepath.Join(cwd, ".rock", "config.toml")
	if raw, err := os.ReadFile(project); err == nil {
		var extra File
		if err := toml.Unmarshal(raw, &extra); err != nil {
			return Loaded{}, err
		}
		if !trusted {
			ignored = append(ignored, extra.Permissions.Allow...)
			extra.Permissions.Allow = nil
		}
		merge(&file, extra)
	}
	if file.MaxSteps <= 0 {
		file.MaxSteps = 12
	}
	mode := perms.Mode(file.Mode)
	switch mode {
	case perms.ModePlan, perms.ModeYolo, perms.ModeDefault:
	default:
		mode = perms.ModeDefault
	}
	return Loaded{
		File:    file,
		Path:    path,
		CWD:     cwd,
		Trusted: trusted,
		Ignored: ignored,
		Policy: perms.Policy{
			Mode:       mode,
			ReviewOnly: file.Git.ReviewOnly,
			Allow:      file.Permissions.Allow,
			Ask:        file.Permissions.Ask,
			Deny:       file.Permissions.Deny,
		},
	}, nil
}

func merge(dst *File, src File) {
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.FastModel != "" {
		dst.FastModel = src.FastModel
	}
	if src.StrongModel != "" {
		dst.StrongModel = src.StrongModel
	}
	if src.BaseURL != "" {
		dst.BaseURL = src.BaseURL
	}
	if src.Mode != "" {
		dst.Mode = src.Mode
	}
	if src.MaxSteps != 0 {
		dst.MaxSteps = src.MaxSteps
	}
	dst.Permissions.Allow = append(dst.Permissions.Allow, src.Permissions.Allow...)
	dst.Permissions.Ask = append(dst.Permissions.Ask, src.Permissions.Ask...)
	dst.Permissions.Deny = append(dst.Permissions.Deny, src.Permissions.Deny...)
	if src.Git.Checkpoint {
		dst.Git.Checkpoint = true
	}
	if src.Git.ReviewOnly {
		dst.Git.ReviewOnly = true
	}
	if src.Jev.BaseURL != "" {
		dst.Jev.BaseURL = src.Jev.BaseURL
	}
	if src.Jev.Model != "" {
		dst.Jev.Model = src.Jev.Model
	}
	if src.Jev.MinConfidence != 0 {
		dst.Jev.MinConfidence = src.Jev.MinConfidence
	}
	if src.Jev.RiskBlock != 0 {
		dst.Jev.RiskBlock = src.Jev.RiskBlock
	}
	if src.Jev.AllowDestructive {
		dst.Jev.AllowDestructive = true
	}
	if src.Jev.Nudge != nil {
		dst.Jev.Nudge = src.Jev.Nudge
	}
	if src.Jev.NudgeEvery != 0 {
		dst.Jev.NudgeEvery = src.Jev.NudgeEvery
	}
	if src.Jev.Enabled != nil {
		dst.Jev.Enabled = src.Jev.Enabled
	}
	if src.Jev.Triage != nil {
		dst.Jev.Triage = src.Jev.Triage
	}
	if src.Jev.Filter != nil {
		dst.Jev.Filter = src.Jev.Filter
	}
	if src.Jev.ClipBytes != 0 {
		dst.Jev.ClipBytes = src.Jev.ClipBytes
	}
	if len(src.MCP) > 0 && dst.MCP == nil {
		dst.MCP = map[string]MCPServer{}
	}
	for k, v := range src.MCP {
		dst.MCP[k] = v
	}
}

func trusted(cwd string) bool {
	if _, err := os.Stat(filepath.Join(cwd, ".rock", "trusted")); err == nil {
		return true
	}
	return false
}

func Write(path string, file File) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := toml.Marshal(file)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func APIKey() string {
	for _, k := range []string{"ROCK_API_KEY", "OPENAI_API_KEY"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func JevKey() (key, source string) {
	if v := strings.TrimSpace(os.Getenv("JEV_API_KEY")); v != "" {
		return v, "JEV_API_KEY"
	}
	if v := strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY")); v != "" {
		return v, "TYPESAFE_API_KEY"
	}
	return "", ""
}
