// Package siwc is Rock's Sign in with ChatGPT path for open-source tools.
// It registers a dynamic OSS client, never Codex's first-party client, and
// never reads ~/.codex.
package siwc

import (
	"os"
	"strings"
)

const (
	// FirstClientID is the documented first-time registration entrypoint.
	// It is not saved. The callback returns an issued oaiapp_… id.
	FirstClientID = "dynamic_agent_client"
	// AgentNameHint is the product name sent only on first registration.
	AgentNameHint = "rock"
	// DefaultIssuer is OpenAI's SIWC authorization host.
	DefaultIssuer = "https://auth.openai.com"
	// DefaultResource is the Responses / models audience.
	DefaultResource = "https://api.openai.com/v1"
	// CallbackPath must stay /auth/callback. /callback does not match.
	CallbackPath = "/auth/callback"
	// IdentityScope and PlanScope are the documented OSS scopes.
	IdentityScope = "openid profile email"
	PlanScope     = "offline_access resource.invoke chatgpt.tokens.use.direct"
	// PlanUsageScope is the grant that authorizes ChatGPT plan inference.
	PlanUsageScope = "chatgpt.tokens.use.direct"
	// CodexClientID is Codex CLI's first-party client. Rock must never
	// send it. Kept here so tests can assert the forbidden value.
	CodexClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	// RefreshLifetime is the documented rotating refresh lifetime.
	RefreshLifetime = 30 * 24 * 60 * 60
)

// Scope is the space-separated authorize scope list.
func Scope() string {
	return IdentityScope + " " + PlanScope
}

// Endpoints are the SIWC HTTP contracts. Tests point them at loopback fakes.
type Endpoints struct {
	Issuer   string
	Resource string
}

func (e Endpoints) issuer() string {
	if e.Issuer != "" {
		return strings.TrimRight(e.Issuer, "/")
	}
	return DefaultIssuer
}

func (e Endpoints) resource() string {
	return e.APIBase()
}

// APIBase is the Responses host, defaulting to api.openai.com/v1.
func (e Endpoints) APIBase() string {
	if e.Resource != "" {
		return strings.TrimRight(e.Resource, "/")
	}
	return DefaultResource
}

func (e Endpoints) AuthorizeURL() string {
	return e.issuer() + "/api/accounts/authorize"
}

func (e Endpoints) TokenURL() string {
	return e.issuer() + "/api/accounts/oauth/token"
}

func (e Endpoints) DiscoveryURL() string {
	return e.issuer() + "/.well-known/openid-configuration"
}

// FromEnv reads ROCK_SIWC_ISSUER and ROCK_SIWC_RESOURCE. Production leaves
// both unset. Tests set them to fake servers. No network happens until a
// login or refresh actually runs.
func FromEnv() Endpoints {
	return Endpoints{
		Issuer:   strings.TrimSpace(os.Getenv("ROCK_SIWC_ISSUER")),
		Resource: strings.TrimSpace(os.Getenv("ROCK_SIWC_RESOURCE")),
	}
}
