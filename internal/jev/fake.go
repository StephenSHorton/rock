package jev

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
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
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "fail", "broken", "error":
			// Omit so Ask records a failed answer with no value.
			// The rest of the batch can still succeed — test-only.
			continue
		}
		switch q.Type {
		case "noul":
			answers[name] = json.RawMessage(`{"type":"noul","noul":0.12}`)
		case "score":
			answers[name] = json.RawMessage(`{"type":"score","score":1,"confidence":0.9,"legend":{"0":"safe","1":"caution","2":"dangerous"}}`)
		default:
			label := fakeChoice(q)
			raw, _ := json.Marshal(map[string]any{
				"type": "choice", "choice": label, "confidence": 0.9,
				"probabilities": map[string]float64{label: 0.9},
			})
			answers[name] = raw
		}
	}
	return Response{Model: "jev-fake", Answers: answers}, nil, true
}

func fakeChoice(q Question) string {
	var crit map[string]string
	if json.Unmarshal(q.Criteria, &crit) == nil && len(crit) > 0 {
		names := make([]string, 0, len(crit))
		for k := range crit {
			names = append(names, k)
		}
		sort.Strings(names)
		return names[0]
	}
	var levels []string
	if json.Unmarshal(q.Criteria, &levels) == nil && len(levels) > 0 {
		return levels[0]
	}
	return "fast"
}
