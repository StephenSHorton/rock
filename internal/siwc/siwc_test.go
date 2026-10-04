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

type fakeAuth struct {
	*httptest.Server
	key         *rsa.PrivateKey
	issued      string
	codes       map[string]pending
	refresh     map[string]int
	mu          sync.Mutex
	revoked     bool
	tokenHits   int
	slowRefresh time.Duration
}

type pending struct {
	verifier string
	nonce    string
	client   string
	redirect string
}

func newFakeAuth(t *testing.T) *fakeAuth {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeAuth{
		key:     key,
		issued:  "oaiapp_rock_test",
		codes:   map[string]pending{},
		refresh: map[string]int{"refresh-1": 1},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 f.URL,
			"jwks_uri":               f.URL + "/.well-known/jwks.json",
			"revocation_endpoint":    f.URL + "/oauth/revoke",
			"authorization_endpoint": f.URL + "/api/accounts/authorize",
			"token_endpoint":         f.URL + "/api/accounts/oauth/token",
		})
	})
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		n := base64.RawURLEncoding.EncodeToString(f.key.N.Bytes())
		e := base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1})
		_ = json.NewEncoder(w).Encode(jwks{Keys: []jwk{{
			Kty: "RSA", Kid: "test", N: n, E: e, Alg: "RS256", Use: "sig",
		}}})
	})
	mux.HandleFunc("/api/accounts/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("client_id") == CodexClientID {
			http.Error(w, "codex client forbidden", 400)
			return
		}
		if q.Get("redirect_uri") == "" || !strings.HasSuffix(q.Get("redirect_uri"), CallbackPath) {
			http.Error(w, "bad redirect", 400)
			return
		}
		if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
			http.Error(w, "pkce", 400)
			return
		}
		if q.Get("ext_agent_host_id") == "" {
			http.Error(w, "host", 400)
			return
		}
		client := q.Get("client_id")
		if client == FirstClientID && q.Get("agent_name_hint") != AgentNameHint {
			http.Error(w, "hint", 400)
			return
		}
		code := fmt.Sprintf("code-%d", time.Now().UnixNano())
		f.mu.Lock()
		f.codes[code] = pending{
			verifier: q.Get("code_challenge"),
			nonce:    q.Get("nonce"),
			client:   client,
			redirect: q.Get("redirect_uri"),
		}
		f.mu.Unlock()
		u, _ := url.Parse(q.Get("redirect_uri"))
		qq := u.Query()
		qq.Set("code", code)
		qq.Set("state", q.Get("state"))
		qq.Set("scope", Scope())
		if client == FirstClientID {
			qq.Set("client_id", f.issued)
		}
		u.RawQuery = qq.Encode()
		http.Redirect(w, r, u.String(), http.StatusFound)
	})
	mux.HandleFunc("/api/accounts/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		f.tokenHits++
		slow := f.slowRefresh
		f.mu.Unlock()
		if r.Form.Get("client_id") == CodexClientID || r.Form.Get("client_id") == FirstClientID {
			http.Error(w, `{"error":"invalid_client"}`, 400)
			return
		}
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			f.mu.Lock()
			p, ok := f.codes[r.Form.Get("code")]
			delete(f.codes, r.Form.Get("code"))
			f.mu.Unlock()
			if !ok {
				http.Error(w, `{"error":"invalid_grant"}`, 400)
				return
			}
			if r.Form.Get("redirect_uri") != p.redirect {
				http.Error(w, `{"error":"invalid_grant","error_description":"redirect"}`, 400)
				return
			}
			if !pkceOK(r.Form.Get("code_verifier"), p.verifier) {
				http.Error(w, `{"error":"invalid_grant","error_description":"pkce"}`, 400)
				return
			}
			idTok, err := f.signID(p.nonce, r.Form.Get("client_id"))
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			_ = json.NewEncoder(w).Encode(tokenResponse{
				AccessToken:  "access-1",
				RefreshToken: "refresh-1",
				IDToken:      idTok,
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				Scope:        Scope(),
			})
		case "refresh_token":
			if slow > 0 {
				time.Sleep(slow)
			}
			old := r.Form.Get("refresh_token")
			f.mu.Lock()
			n := f.refresh[old]
			if n == 0 {
				f.mu.Unlock()
				http.Error(w, `{"error":"invalid_grant"}`, 400)
				return
			}
			delete(f.refresh, old)
			next := fmt.Sprintf("refresh-%d", n+1)
			f.refresh[next] = n + 1
			f.mu.Unlock()
			_ = json.NewEncoder(w).Encode(tokenResponse{
				AccessToken:  fmt.Sprintf("access-%d", n+1),
				RefreshToken: next,
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				Scope:        Scope(),
			})
		default:
			http.Error(w, `{"error":"unsupported_grant_type"}`, 400)
		}
	})
	mux.HandleFunc("/oauth/revoke", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		f.revoked = true
		delete(f.refresh, r.Form.Get("token"))
		f.mu.Unlock()
		w.WriteHeader(200)
	})
	f.Server = httptest.NewServer(mux)
	return f
}

func (f *fakeAuth) signID(nonce, aud string) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "test", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]any{
		"iss":   f.URL,
		"sub":   "user-1",
		"aud":   aud,
		"email": "dev@example.com",
		"nonce": nonce,
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	h := base64.RawURLEncoding.EncodeToString(header)
	p := base64.RawURLEncoding.EncodeToString(payload)
	sum := sha256.Sum256([]byte(h + "." + p))
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return h + "." + p + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func pkceOK(verifier, challenge string) bool {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:]) == challenge
}
