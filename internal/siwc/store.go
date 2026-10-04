package siwc

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Keyring is the OS secret store. cli.Open wires this to config.Secrets
// (the same helper as the Jev key). Get/Set/Delete use service "rock"
// and user "chatgpt". A missing keychain falls back to the 0600 file.
var Keyring SecretStore

// SecretStore is the optional keyring surface. Get returns os.ErrNotExist
// when the name is missing.
type SecretStore interface {
	Get(name string) ([]byte, error)
	Set(name string, value []byte) error
	Delete(name string) error
}

const keyringName = "chatgpt"

// Record is one ChatGPT registration, matching the SIWC credential example.
type Record struct {
	Email           string   `json:"email,omitempty"`
	Issuer          string   `json:"issuer"`
	Subject         string   `json:"subject"`
	ClientID        string   `json:"client_id"`
	ExtAgentHostID  string   `json:"ext_agent_host_id"`
	IDToken         string   `json:"id_token,omitempty"`
	AccessToken     string   `json:"access_token"`
	RefreshToken    string   `json:"refresh_token,omitempty"`
	TokenType       string   `json:"token_type,omitempty"`
	ExpiresIn       int      `json:"expires_in,omitempty"`
	ExpiresAt       int64    `json:"expires_at,omitempty"`
	RefreshUntil    int64    `json:"refresh_until,omitempty"`
	EarliestRefresh int64    `json:"earliest_refresh_at,omitempty"`
	Scopes          []string `json:"scopes,omitempty"`
	SavedAt         string   `json:"saved_at,omitempty"`
}

// HasPlanUsage reports chatgpt.tokens.use.direct on the saved grant.
func (r Record) HasPlanUsage() bool {
	for _, s := range r.Scopes {
		if s == PlanUsageScope {
			return true
		}
	}
	return false
}

func (r Record) accessExpiry() time.Time {
	if r.ExpiresAt > 0 {
		return time.Unix(r.ExpiresAt, 0)
	}
	return time.Time{}
}

// Store is the local credential file plus an optional keyring. Refreshes
// for the same record are serialized.
type Store struct {
	mu   sync.Mutex
	path string
	ring SecretStore
}

func OpenStore() *Store {
	return &Store{path: credPath(), ring: Keyring}
}

func (s *Store) Path() string {
	if s == nil || s.path == "" {
		return credPath()
	}
	return s.path
}

func (s *Store) Load() (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Store) loadLocked() (Record, error) {
	if s.ring != nil {
		raw, err := s.ring.Get(keyringName)
		if err == nil && len(raw) > 0 {
			var rec Record
			if err := json.Unmarshal(raw, &rec); err != nil {
				return Record{}, err
			}
			if err := rec.validate(); err != nil {
				return Record{}, err
			}
			return rec, nil
		}
	}
	raw, err := os.ReadFile(s.Path())
	if err != nil {
		return Record{}, err
	}
	var rec Record
	if err := json.Unmarshal(raw, &rec); err != nil {
		return Record{}, err
	}
	if err := rec.validate(); err != nil {
		return Record{}, err
	}
	return rec, nil
}

func (s *Store) Save(rec Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(rec)
}

func (s *Store) saveLocked(rec Record) error {
	if err := rec.validate(); err != nil {
		return err
	}
	if rec.SavedAt == "" {
		rec.SavedAt = time.Now().UTC().Format(time.RFC3339)
	}
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if s.ring != nil {
		if err := s.ring.Set(keyringName, raw); err == nil {
			_ = os.Remove(s.Path())
			return nil
		}
	}
	path := s.Path()
	if rejectsCodexPath(path) {
		return fmt.Errorf("siwc: refusing to write a Codex path %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Chmod(path, 0o600)
}

func (s *Store) ClearTokens() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.loadLocked()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	rec.AccessToken = ""
	rec.RefreshToken = ""
	rec.IDToken = ""
	rec.ExpiresAt = 0
	rec.ExpiresIn = 0
	rec.RefreshUntil = 0
	rec.EarliestRefresh = 0
	rec.SavedAt = time.Now().UTC().Format(time.RFC3339)
	return s.saveLocked(rec)
}

func (s *Store) Remove() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ring != nil {
		_ = s.ring.Delete(keyringName)
	}
	err := os.Remove(s.Path())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (r Record) validate() error {
	if r.ClientID == "" || r.ClientID == FirstClientID {
		return fmt.Errorf("siwc: missing issued client id")
	}
	if r.ClientID == CodexClientID {
		return fmt.Errorf("siwc: refusing Codex client id")
	}
	// Production issued ids are oaiapp_. The fake auth server uses the
	// same prefix so a Codex id cannot sneak in as a test fixture.
	if !strings.HasPrefix(r.ClientID, "oaiapp_") {
		return fmt.Errorf("siwc: issued client id %q is not an oaiapp_ id", r.ClientID)
	}
	if r.ExtAgentHostID == "" {
		return fmt.Errorf("siwc: missing ext_agent_host_id")
	}
	return nil
}

// LoggedIn is true when a record with tokens and plan usage exists.
func LoggedIn() bool {
	rec, err := OpenStore().Load()
	if err != nil {
		return false
	}
	return rec.AccessToken != "" && rec.HasPlanUsage()
}
