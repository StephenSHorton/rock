// Package session stores transcripts as JSONL under ROCK_HOME. The TUI is not
// the owner of this state.
package session

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/StephenSHorton/rock/internal/provider"
)

type Meta struct {
	ID        string    `json:"id"`
	CWD       string    `json:"cwd"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Mode      string    `json:"mode"`
}

type Session struct {
	Meta     Meta
	Messages []provider.Message
	Dir      string
	root     string
}

func Home() string {
	if h := os.Getenv("ROCK_HOME"); h != "" {
		return h
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".rock"
	}
	return filepath.Join(home, ".rock")
}

func NewID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func cwdKey(cwd string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(cwd)))
	return hex.EncodeToString(sum[:8])
}

func dirFor(root, cwd, id string) string {
	return filepath.Join(root, "sessions", cwdKey(cwd), id)
}

func Create(cwd, id, title string) (*Session, error) {
	if id == "" {
		id = NewID()
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	root := Home()
	dir := dirFor(root, abs, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Session{
		Meta: Meta{ID: id, CWD: abs, Title: title, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Mode: "default"},
		Dir:  dir,
		root: root,
	}
	return s, s.Save()
}

func Load(cwd, id string) (*Session, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	root := Home()
	dir := dirFor(root, abs, id)
	metaRaw, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return nil, err
	}
	var meta Meta
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(dir, "transcript.jsonl"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var msgs []provider.Message
	if f != nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 2<<20)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			var m provider.Message
			if err := json.Unmarshal([]byte(line), &m); err != nil {
				return nil, err
			}
			msgs = append(msgs, m)
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
	}
	return &Session{Meta: meta, Messages: msgs, Dir: dir, root: root}, nil
}

func List(cwd string) ([]Meta, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	parent := filepath.Join(Home(), "sessions", cwdKey(abs))
	entries, err := os.ReadDir(parent)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Meta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(parent, e.Name(), "meta.json"))
		if err != nil {
			continue
		}
		var meta Meta
		if json.Unmarshal(raw, &meta) == nil {
			out = append(out, meta)
		}
	}
	return out, nil
}

func (s *Session) Append(m provider.Message) {
	s.Messages = append(s.Messages, m)
	s.Meta.UpdatedAt = time.Now().UTC()
}

func (s *Session) PlanPath() string {
	return filepath.Join(s.Dir, "plan.md")
}

func (s *Session) Save() error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	s.Meta.UpdatedAt = time.Now().UTC()
	raw, err := json.MarshalIndent(s.Meta, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.Dir, "meta.json"), raw, 0o644); err != nil {
		return err
	}
	var b strings.Builder
	for _, m := range s.Messages {
		line, err := json.Marshal(m)
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return os.WriteFile(filepath.Join(s.Dir, "transcript.jsonl"), []byte(b.String()), 0o644)
}

func (s *Session) Bytes() int {
	n := 0
	for _, m := range s.Messages {
		n += len(m.Content) + len(m.Name)
		for _, c := range m.ToolCalls {
			n += len(c.Arguments) + len(c.Name)
		}
	}
	return n
}

func (s *Session) RecentTools() []string {
	var names []string
	for _, m := range s.Messages {
		for _, c := range m.ToolCalls {
			names = append(names, c.Name)
		}
	}
	if len(names) > 8 {
		names = names[len(names)-8:]
	}
	return names
}

func (s *Session) ReplaceSystem(content string) {
	if len(s.Messages) > 0 && s.Messages[0].Role == provider.RoleSystem {
		s.Messages[0].Content = content
		return
	}
	s.Messages = append([]provider.Message{{Role: provider.RoleSystem, Content: content}}, s.Messages...)
}

// Compact keeps the system prompt, a summary, and the last keep messages.
func (s *Session) Compact(summary string, keep int) {
	if keep < 1 {
		keep = 4
	}
	start := 0
	if len(s.Messages) > 0 && s.Messages[0].Role == provider.RoleSystem {
		start = 1
	}
	if len(s.Messages)-start <= keep {
		return
	}
	tail := s.Messages[len(s.Messages)-keep:]
	head := []provider.Message{}
	if start == 1 {
		head = append(head, s.Messages[0])
	}
	head = append(head, provider.Message{Role: provider.RoleSystem, Content: "Compacted history:\n" + summary})
	s.Messages = append(head, tail...)
}

func (s *Session) LastAssistant() string {
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role == provider.RoleAssistant && s.Messages[i].Content != "" && len(s.Messages[i].ToolCalls) == 0 {
			return s.Messages[i].Content
		}
	}
	return ""
}

func MustSave(s *Session) error {
	if s == nil {
		return fmt.Errorf("nil session")
	}
	return s.Save()
}
