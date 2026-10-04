package siwc

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDirNeverTouchesCodex(t *testing.T) {
	t.Setenv("ROCK_SIWC_HOME", t.TempDir())
	t.Setenv("ROCK_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("ROCK_HOME", t.TempDir())
	for _, p := range []string{Dir(), hostPath(), credPath()} {
		if rejectsCodexPath(p) || strings.Contains(p, ".codex") {
			t.Fatalf("path leaked Codex: %s", p)
		}
	}
}

type memSecrets struct{ m map[string][]byte }

func (s memSecrets) Get(name string) ([]byte, error) {
	v, ok := s.m[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return v, nil
}
func (s memSecrets) Set(name string, value []byte) error {
	s.m[name] = append([]byte(nil), value...)
	return nil
}
func (s memSecrets) Delete(name string) error {
	delete(s.m, name)
	return nil
}

type failSecrets struct{}

func (failSecrets) Get(string) ([]byte, error) { return nil, os.ErrNotExist }
func (failSecrets) Set(string, []byte) error   { return fmt.Errorf("no keychain") }
func (failSecrets) Delete(string) error        { return fmt.Errorf("no keychain") }

func TestStorePrefersKeyringAndFallsBackToFile(t *testing.T) {
	t.Setenv("ROCK_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	host, err := HostID()
	if err != nil {
		t.Fatal(err)
	}
	rec := Record{ClientID: "oaiapp_ring", ExtAgentHostID: host, AccessToken: "tok", Scopes: []string{PlanUsageScope}}

	orig := Keyring
	t.Cleanup(func() { Keyring = orig })

	mem := memSecrets{m: map[string][]byte{}}
	Keyring = mem
	s := OpenStore()
	if err := s.Save(rec); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Path()); !os.IsNotExist(err) {
		t.Fatal("keyring success must not write the 0600 file")
	}
	got, err := s.Load()
	if err != nil || got.AccessToken != "tok" {
		t.Fatalf("%+v %v", got, err)
	}

	Keyring = failSecrets{}
	s = OpenStore()
	if err := s.Save(rec); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(s.Path())
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("file fallback: %v %v", err, st)
	}
}

func TestStoreRejectsCodexClient(t *testing.T) {
	t.Setenv("ROCK_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	s := OpenStore()
	err := s.Save(Record{
		ClientID:       CodexClientID,
		ExtAgentHostID: "urn:uuid:11111111-1111-4111-8111-111111111111",
		AccessToken:    "x",
	})
	if err == nil {
		t.Fatal("saved Codex client")
	}
	err = s.Save(Record{
		ClientID:       FirstClientID,
		ExtAgentHostID: "urn:uuid:11111111-1111-4111-8111-111111111111",
		AccessToken:    "x",
	})
	if err == nil {
		t.Fatal("saved dynamic_agent_client")
	}
}

func TestHostIDStableAndFile0600(t *testing.T) {
	t.Setenv("ROCK_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	a, err := HostID()
	if err != nil {
		t.Fatal(err)
	}
	b, err := HostID()
	if err != nil || a != b {
		t.Fatalf("%q %q %v", a, b, err)
	}
	if !strings.HasPrefix(a, "urn:uuid:") {
		t.Fatal(a)
	}
	st, err := os.Stat(hostPath())
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("host mode %o", st.Mode().Perm())
	}
}
