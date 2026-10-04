package siwc

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/StephenSHorton/rock/internal/config"
)

// LoginOpts is the interactive SIWC flow. Tests inject a fake OpenURL and
// Endpoints. Production opens the system browser.
type LoginOpts struct {
	Out     io.Writer
	OpenURL func(string) error
	HTTP    *http.Client
	EP      Endpoints
	Store   *Store
	Timeout time.Duration
}

// Login runs PKCE loopback sign-in at /auth/callback.
func Login(ctx context.Context, opts LoginOpts) error {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.HTTP == nil {
		opts.HTTP = &http.Client{Timeout: 30 * time.Second}
	}
	if opts.Store == nil {
		opts.Store = OpenStore()
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Minute
	}
	if opts.EP.Issuer == "" && opts.EP.Resource == "" {
		opts.EP = FromEnv()
	}
	hostID, err := HostID()
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("siwc: loopback listen: %w", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	redirect := fmt.Sprintf("http://127.0.0.1:%d%s", port, CallbackPath)

	clientID := FirstClientID
	var idHint, loginHint string
	if rec, err := opts.Store.Load(); err == nil && rec.ClientID != "" && rec.ClientID != FirstClientID {
		clientID = rec.ClientID
		idHint = rec.IDToken
		loginHint = rec.Email
	}

	verifier, challenge, err := newPKCE()
	if err != nil {
		return err
	}
	state, nonce, err := newStateNonce()
	if err != nil {
		return err
	}
	authURL, err := authorizeQuery(opts.EP, clientID, hostID, redirect, state, nonce, challenge, idHint, loginHint)
	if err != nil {
		return err
	}

	type result struct {
		code   string
		issued string
		err    error
		scopes string
		denied bool
	}
	got := make(chan result, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(CallbackPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			select {
			case got <- result{err: fmt.Errorf("siwc: callback state mismatch")}:
			default:
			}
			return
		}
		if errCode := q.Get("error"); errCode != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, "<p>Rock sign-in was cancelled.</p>")
			select {
			case got <- result{denied: true, err: fmt.Errorf("siwc: %s", errCode)}:
			default:
			}
			return
		}
		code := q.Get("code")
		issued := q.Get("client_id")
		if issued != "" && clientID != FirstClientID && issued != clientID {
			http.Error(w, "client id mismatch", http.StatusBadRequest)
			select {
			case got <- result{err: fmt.Errorf("siwc: callback client_id does not match the saved registration")}:
			default:
			}
			return
		}
		if clientID == FirstClientID && issued == "" {
			http.Error(w, "missing issued client id", http.StatusBadRequest)
			select {
			case got <- result{err: fmt.Errorf("siwc: registration callback missing issued client_id")}:
			default:
			}
			return
		}
		if issued == "" {
			issued = clientID
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<p>Rock is signed in with ChatGPT. You can close this tab.</p>")
		select {
		case got <- result{code: code, issued: issued, scopes: q.Get("scope")}:
		default:
		}
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	open := opts.OpenURL
	if open == nil {
		open = openBrowser
	}
	fmt.Fprintf(opts.Out, "Continue with ChatGPT:\n%s\n", authURL)
	if err := open(authURL); err != nil {
		fmt.Fprintf(opts.Out, "open the URL in a browser if it did not launch: %v\n", err)
	}

	timer := time.NewTimer(opts.Timeout)
	defer timer.Stop()
	var res result
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("siwc: timed out waiting for /auth/callback")
	case res = <-got:
	}
	if res.err != nil {
		return res.err
	}
	if res.issued == CodexClientID || res.issued == FirstClientID {
		return fmt.Errorf("siwc: refusing to save client id %s", res.issued)
	}

	tok, err := exchangeCode(ctx, opts.HTTP, opts.EP, res.issued, res.code, verifier, redirect)
	if err != nil {
		return err
	}
	now := time.Now()
	claims, err := verifyIDToken(tok.IDToken, opts.EP.issuer(), res.issued, nonce, opts.HTTP, now)
	if err != nil {
		return err
	}
	scopes := tok.scopes()
	if len(scopes) == 0 {
		scopes = strings.Fields(res.scopes)
	}
	hasPlan := false
	for _, s := range scopes {
		if s == PlanUsageScope {
			hasPlan = true
			break
		}
	}
	if !hasPlan {
		return fmt.Errorf("siwc: ChatGPT plan usage was not granted (missing %s). The user can enable it in ChatGPT settings", PlanUsageScope)
	}

	rec := Record{
		Email:          claims.Email,
		Issuer:         claims.Issuer,
		Subject:        claims.Subject,
		ClientID:       res.issued,
		ExtAgentHostID: hostID,
		Scopes:         scopes,
	}
	tok.apply(&rec, now)
	if err := opts.Store.Save(rec); err != nil {
		return err
	}
	fmt.Fprintf(opts.Out, "signed in as %s (client %s)\n", displayAccount(rec), rec.ClientID)
	return nil
}

func displayAccount(rec Record) string {
	if rec.Email != "" {
		return rec.Email
	}
	if rec.Subject != "" {
		return rec.Subject
	}
	return rec.ClientID
}

// Logout revokes the refresh token when possible and clears local tokens.
// The host id and issued client id stay so the next login is a reauthorization.
func Logout(ctx context.Context, opts LoginOpts) error {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.HTTP == nil {
		opts.HTTP = &http.Client{Timeout: 15 * time.Second}
	}
	if opts.Store == nil {
		opts.Store = OpenStore()
	}
	if opts.EP.Issuer == "" && opts.EP.Resource == "" {
		opts.EP = FromEnv()
	}
	rec, err := opts.Store.Load()
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(opts.Out, "not signed in with ChatGPT")
			return nil
		}
		return err
	}
	revoked := true
	if rec.RefreshToken != "" {
		if err := revokeRefresh(ctx, opts.HTTP, opts.EP, rec.ClientID, rec.RefreshToken); err != nil {
			revoked = false
			fmt.Fprintf(opts.Out, "remote revocation was not confirmed: %v\nDisconnect the app in ChatGPT Settings if it still appears.\n", err)
		}
	}
	if err := opts.Store.ClearTokens(); err != nil {
		return err
	}
	if revoked {
		fmt.Fprintln(opts.Out, "signed out of ChatGPT. The issued client stays for the next login.")
	}
	return nil
}

func openBrowser(u string) error {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("ROCK_SIWC_OPEN_URL"))); v == "0" || v == "false" || v == "no" {
		return nil
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	config.ScrubCmdEnv(cmd)
	return cmd.Start()
}
