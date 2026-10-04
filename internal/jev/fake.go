package jev

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// TestFake reports whether ROCK_TEST_FAKE_JEV is set. That env is a
// test-only stub. It is never on by default and is not a user feature.
func TestFake() bool {
	return fakeMode() != ""
}

func fakeMode() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv("ROCK_TEST_FAKE_JEV")))
}

func fakeReject() bool {
	switch fakeMode() {
	case "0", "reject", "fail", "invalid", "no":
		return true
	}
	return false
}

// fakeDecide intercepts Client.Decide when ROCK_TEST_FAKE_JEV is set and
// the client has no explicit BaseURL (live httptest servers still win).
func fakeDecide(c *Client, questions map[string]Question) (Response, error, bool) {
	if !TestFake() || c == nil || strings.TrimSpace(c.BaseURL) != "" {
		return Response{}, nil, false
	}
	if d := strings.TrimSpace(os.Getenv("ROCK_TEST_FAKE_JEV_DELAY")); d != "" {
		if dur, err := time.ParseDuration(d); err == nil {
			time.Sleep(dur)
		}
	}
	if fakeReject() {
		return Response{}, fmt.Errorf("jev http 401: invalid key"), true
	}
	answers := map[string]json.RawMessage{}
	for name, q := range questions {
		switch q.Type {
		case "noul":
			answers[name] = json.RawMessage(`{"type":"noul","noul":0.12}`)
		case "score":
			answers[name] = json.RawMessage(`{"type":"score","score":1,"confidence":0.9,"legend":{}}`)
		default:
			answers[name] = json.RawMessage(`{"type":"choice","choice":"fast","confidence":0.9,"probabilities":{"fast":0.9}}`)
		}
	}
	return Response{Model: "jev-fake", Answers: answers}, nil, true
}
