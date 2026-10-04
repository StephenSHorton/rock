package siwc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type tokenResponse struct {
	AccessToken       string `json:"access_token"`
	RefreshToken      string `json:"refresh_token"`
	IDToken           string `json:"id_token"`
	TokenType         string `json:"token_type"`
	ExpiresIn         int    `json:"expires_in"`
	Scope             string `json:"scope"`
	EarliestRefreshAt int64  `json:"earliest_refresh_at"`
	Error             string `json:"error"`
	ErrorDescription  string `json:"error_description"`
}

func (t tokenResponse) scopes() []string {
	var out []string
	for _, s := range strings.Fields(t.Scope) {
		out = append(out, s)
	}
	return out
}

func (t tokenResponse) apply(rec *Record, now time.Time) {
	rec.AccessToken = t.AccessToken
	if t.RefreshToken != "" {
		rec.RefreshToken = t.RefreshToken
	}
	if t.IDToken != "" {
		rec.IDToken = t.IDToken
	}
	if t.TokenType != "" {
		rec.TokenType = t.TokenType
	}
	rec.ExpiresIn = t.ExpiresIn
	if t.ExpiresIn > 0 {
		rec.ExpiresAt = now.Add(time.Duration(t.ExpiresIn) * time.Second).Unix()
	}
	if rec.RefreshToken != "" {
		rec.RefreshUntil = now.Add(time.Duration(RefreshLifetime) * time.Second).Unix()
	}
	rec.EarliestRefresh = t.EarliestRefreshAt
	if sc := t.scopes(); len(sc) > 0 {
		rec.Scopes = sc
	}
	rec.SavedAt = now.UTC().Format(time.RFC3339)
}

func authorizeQuery(ep Endpoints, clientID, hostID, redirect, state, nonce, challenge, idTokenHint, loginHint string) (string, error) {
	if clientID == "" {
		return "", fmt.Errorf("siwc: empty client id")
	}
	if clientID == CodexClientID {
		return "", fmt.Errorf("siwc: refusing Codex client id")
	}
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", redirect)
	q.Set("scope", Scope())
	q.Set("resource", ep.resource())
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge_method", "S256")
	q.Set("code_challenge", challenge)
	q.Set("ext_agent_host_id", hostID)
	if clientID == FirstClientID {
		q.Set("agent_name_hint", AgentNameHint)
	}
	if idTokenHint != "" {
		q.Set("id_token_hint", idTokenHint)
	}
	if loginHint != "" {
		q.Set("login_hint", loginHint)
	}
	return ep.AuthorizeURL() + "?" + q.Encode(), nil
}

func exchangeCode(ctx context.Context, httpc *http.Client, ep Endpoints, clientID, code, verifier, redirect string) (tokenResponse, error) {
	if clientID == "" || clientID == FirstClientID {
		return tokenResponse{}, fmt.Errorf("siwc: code exchange needs the issued client id, not %s", FirstClientID)
	}
	if clientID == CodexClientID {
		return tokenResponse{}, fmt.Errorf("siwc: refusing Codex client id")
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("redirect_uri", redirect)
	form.Set("resource", ep.resource())
	return postToken(ctx, httpc, ep.TokenURL(), form)
}

func refreshGrant(ctx context.Context, httpc *http.Client, ep Endpoints, clientID, refresh string) (tokenResponse, error) {
	if clientID == "" || clientID == FirstClientID || clientID == CodexClientID {
		return tokenResponse{}, fmt.Errorf("siwc: refresh needs Rock's issued client id")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", clientID)
	form.Set("refresh_token", refresh)
	form.Set("resource", ep.resource())
	return postToken(ctx, httpc, ep.TokenURL(), form)
}

func revokeRefresh(ctx context.Context, httpc *http.Client, ep Endpoints, clientID, refresh string) error {
	if refresh == "" {
		return nil
	}
	endpoint, err := revocationURL(ctx, httpc, ep)
	if err != nil {
		return err
	}
	form := url.Values{}
	form.Set("token", refresh)
	form.Set("token_type_hint", "refresh_token")
	form.Set("client_id", clientID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := httpc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<16))
	if res.StatusCode >= 300 && res.StatusCode < 500 {
		return fmt.Errorf("siwc: revoke http %d", res.StatusCode)
	}
	if res.StatusCode >= 500 {
		return fmt.Errorf("siwc: revoke http %d", res.StatusCode)
	}
	return nil
}

func revocationURL(ctx context.Context, httpc *http.Client, ep Endpoints) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep.DiscoveryURL(), nil)
	if err != nil {
		return "", err
	}
	res, err := httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode >= 300 {
		return ep.issuer() + "/oauth/revoke", nil
	}
	var doc struct {
		Revocation string `json:"revocation_endpoint"`
	}
	if err := json.Unmarshal(body, &doc); err != nil || doc.Revocation == "" {
		return ep.issuer() + "/oauth/revoke", nil
	}
	return doc.Revocation, nil
}

func postToken(ctx context.Context, httpc *http.Client, tokenURL string, form url.Values) (tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := httpc.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return tokenResponse{}, err
	}
	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return tokenResponse{}, fmt.Errorf("siwc: token: %w", err)
	}
	if res.StatusCode >= 300 || tok.Error != "" {
		msg := tok.Error
		if tok.ErrorDescription != "" {
			msg = tok.Error + ": " + tok.ErrorDescription
		}
		if msg == "" {
			msg = fmt.Sprintf("http %d", res.StatusCode)
		}
		return tokenResponse{}, fmt.Errorf("siwc: token: %s", msg)
	}
	if tok.AccessToken == "" {
		return tokenResponse{}, fmt.Errorf("siwc: token response missing access_token")
	}
	return tok, nil
}
