package siwc

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Access returns a bearer token, refreshing once near expiry. Refreshes
// are serialized so a rotating 30-day refresh token is not raced.
func (s *Store) Access(ctx context.Context) (string, error) {
	return s.AccessAt(ctx, FromEnv(), http.DefaultClient, time.Now())
}

func (s *Store) AccessAt(ctx context.Context, ep Endpoints, httpc *http.Client, now time.Time) (string, error) {
	if s == nil {
		s = OpenStore()
	}
	if httpc == nil {
		httpc = http.DefaultClient
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.loadLocked()
	if err != nil {
		return "", err
	}
	if rec.AccessToken != "" && rec.accessExpiry().After(now.Add(60*time.Second)) {
		return rec.AccessToken, nil
	}
	if rec.RefreshToken == "" {
		if rec.AccessToken != "" && (rec.ExpiresAt == 0 || rec.accessExpiry().After(now)) {
			return rec.AccessToken, nil
		}
		return "", fmt.Errorf("siwc: session expired; run rock login chatgpt")
	}
	if rec.RefreshUntil > 0 && now.Unix() >= rec.RefreshUntil {
		return "", fmt.Errorf("siwc: refresh token expired; run rock login chatgpt")
	}
	if rec.EarliestRefresh > 0 && now.Unix() < rec.EarliestRefresh {
		if rec.AccessToken != "" {
			return rec.AccessToken, nil
		}
	}
	tok, err := refreshGrant(ctx, httpc, ep, rec.ClientID, rec.RefreshToken)
	if err != nil {
		return "", err
	}
	tok.apply(&rec, now)
	if err := s.saveLocked(rec); err != nil {
		return "", err
	}
	return rec.AccessToken, nil
}

// AuthClass is the inspect label for the active model credential.
const (
	AuthAPIKey       = "api_key"
	AuthSIWC         = "siwc"
	AuthOfflineModel = "offline_model"
)

// Resolve picks the model auth class. Preference in config (auth=) wins
// when that class is available. Otherwise SIWC if logged in, else an API
// key, else the offline model stub. API keys stay the fallback.
func Resolve(prefer, apiKey string) string {
	prefer = trimAuth(prefer)
	logged := LoggedIn()
	hasKey := apiKey != ""
	switch prefer {
	case AuthSIWC:
		if logged {
			return AuthSIWC
		}
		if hasKey {
			return AuthAPIKey
		}
		return AuthOfflineModel
	case AuthAPIKey:
		if hasKey {
			return AuthAPIKey
		}
		if logged {
			return AuthSIWC
		}
		return AuthOfflineModel
	case AuthOfflineModel:
		return AuthOfflineModel
	default:
		if logged {
			return AuthSIWC
		}
		if hasKey {
			return AuthAPIKey
		}
		return AuthOfflineModel
	}
}

func trimAuth(s string) string {
	switch s {
	case AuthSIWC, AuthAPIKey, AuthOfflineModel:
		return s
	}
	return ""
}
