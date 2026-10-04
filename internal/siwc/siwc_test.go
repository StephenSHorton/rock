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

func TestLoginRefreshLogoutAgainstFakeAuth(t *testing.T) {
	t.Setenv("ROCK_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("ROCK_SIWC_OPEN_URL", "0")
	auth := newFakeAuth(t)
	defer auth.Close()
	t.Setenv("ROCK_SIWC_ISSUER", auth.URL)
	t.Setenv("ROCK_SIWC_RESOURCE", auth.URL+"/v1")

	store := OpenStore()
	var opened string
	opts := LoginOpts{
		Out:     io.Discard,
		HTTP:    auth.Client(),
		EP:      Endpoints{Issuer: auth.URL, Resource: auth.URL + "/v1"},
		Store:   store,
		Timeout: 8 * time.Second,
		OpenURL: func(u string) error {
			opened = u
			go func() {
				client := &http.Client{Timeout: 5 * time.Second}
				_, _ = client.Get(u)
			}()
			return nil
		},
	}
	if err := Login(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if opened == "" || strings.Contains(opened, CodexClientID) {
		t.Fatalf("authorize URL: %s", opened)
	}
	if !strings.Contains(opened, "client_id="+FirstClientID) && !strings.Contains(opened, "client_id=dynamic_agent_client") {
		t.Fatalf("first login must use dynamic_agent_client: %s", opened)
	}
	if !strings.Contains(opened, "agent_name_hint=rock") {
		t.Fatalf("missing agent_name_hint: %s", opened)
	}
	if strings.Contains(opened, "/oauth/authorize") && !strings.Contains(opened, "/api/accounts/authorize") {
		t.Fatal("used Codex authorize path")
	}
	rec, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if rec.ClientID != auth.issued {
		t.Fatalf("client %s", rec.ClientID)
	}
	if rec.ExtAgentHostID == "" || rec.AccessToken == "" || rec.RefreshToken == "" {
		t.Fatalf("%+v", rec)
	}
	if !rec.HasPlanUsage() {
		t.Fatal(rec.Scopes)
	}
	st, err := os.Stat(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("cred mode %o", st.Mode().Perm())
	}
	raw, _ := os.ReadFile(store.Path())
	if strings.Contains(string(raw), CodexClientID) {
		t.Fatal("file contains Codex client")
	}

	rec.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	if err := store.Save(rec); err != nil {
		t.Fatal(err)
	}
	tok1, err := store.AccessAt(context.Background(), opts.EP, auth.Client(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	tok2, err := store.AccessAt(context.Background(), opts.EP, auth.Client(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if tok1 == rec.AccessToken {
		t.Fatal("refresh did not rotate access")
	}
	fresh, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if fresh.RefreshToken == rec.RefreshToken {
		t.Fatal("refresh token did not rotate")
	}
	if tok2 != tok1 && tok2 != fresh.AccessToken {
		t.Fatalf("serialized refresh produced a surprise token %s", tok2)
	}

	if err := Logout(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if !auth.revoked {
		t.Fatal("refresh token was not revoked")
	}
	cleared, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cleared.AccessToken != "" || cleared.RefreshToken != "" {
		t.Fatalf("tokens still present: %+v", cleared)
	}
	if cleared.ClientID != auth.issued {
		t.Fatal("logout must keep the issued client")
	}
}

func TestRefreshIsSerialized(t *testing.T) {
	t.Setenv("ROCK_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	auth := newFakeAuth(t)
	defer auth.Close()
	store := OpenStore()
	host, err := HostID()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(Record{
		ClientID:       auth.issued,
		ExtAgentHostID: host,
		AccessToken:    "old",
		RefreshToken:   "refresh-1",
		ExpiresAt:      time.Now().Add(-time.Minute).Unix(),
		Scopes:         []string{PlanUsageScope},
	}); err != nil {
		t.Fatal(err)
	}
	auth.slowRefresh = 40 * time.Millisecond
	ep := Endpoints{Issuer: auth.URL, Resource: auth.URL + "/v1"}
	var wg sync.WaitGroup
	got := make([]string, 2)
	errs := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got[i], errs[i] = store.AccessAt(context.Background(), ep, auth.Client(), time.Now())
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
	}
	if got[0] == "" || got[0] != got[1] {
		t.Fatalf("raced refresh: %q %q (token hits %d)", got[0], got[1], auth.tokenHits)
	}
	if auth.tokenHits != 1 {
		t.Fatalf("expected one refresh POST, got %d", auth.tokenHits)
	}
}

func TestResolvePrefersSIWCThenAPIKey(t *testing.T) {
	t.Setenv("ROCK_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	if got := Resolve("", ""); got != AuthOfflineModel {
		t.Fatal(got)
	}
	if got := Resolve("", "sk-test"); got != AuthAPIKey {
		t.Fatal(got)
	}
	host, _ := HostID()
	_ = OpenStore().Save(Record{
		ClientID:       "oaiapp_saved",
		ExtAgentHostID: host,
		AccessToken:    "tok",
		Scopes:         []string{PlanUsageScope},
	})
	if got := Resolve("", "sk-test"); got != AuthSIWC {
		t.Fatal(got)
	}
	if got := Resolve(AuthAPIKey, "sk-test"); got != AuthAPIKey {
		t.Fatal(got)
	}
	if got := Resolve(AuthSIWC, "sk-test"); got != AuthSIWC {
		t.Fatal(got)
	}
}

func TestAuthorizeQueryRefusesCodex(t *testing.T) {
	_, err := authorizeQuery(Endpoints{}, CodexClientID, "urn:uuid:x", "http://127.0.0.1:1/auth/callback", "s", "n", "c", "", "")
	if err == nil {
		t.Fatal("codex client")
	}
}
