// Package jev calls the Jev decision API. choice, score, and noul come back
// typed. Official keys go to api.typesafe.ai. Hosted jv_live keys go to
// jevtypesafeai.com. Rock does not start without a working key.
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Question struct {
	Type         string          `json:"type"`
	Instructions string          `json:"instructions"`
	Criteria     json.RawMessage `json:"criteria,omitempty"`
}

type Choice struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
}

type Score struct {
	Type          string             `json:"type"`
	Score         float64            `json:"score"`
	Confidence    float64            `json:"confidence"`
	Legend        map[string]string  `json:"legend"`
	Probabilities map[string]float64 `json:"probabilities"`
}

type Noul struct {
	Type string  `json:"type"`
	Noul float64 `json:"noul"`
}

type Response struct {
	Model   string                     `json:"model"`
	Answers map[string]json.RawMessage `json:"answers"`
	Usage   *Usage                     `json:"usage,omitempty"`
}

type Client struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

func (c *Client) Live() bool { return c != nil && strings.TrimSpace(c.APIKey) != "" }

func EndpointFor(key string) string {
	if strings.HasPrefix(key, "jv_live_") {
		return "https://jevtypesafeai.com/api/v1/decide"
	}
	return "https://api.typesafe.ai/v1/systemone"
}

func (c *Client) Decide(ctx context.Context, state any, questions map[string]Question) (Response, error) {
	if !c.Live() {
		return Response{}, fmt.Errorf("jev key is not set")
	}
	if out, err, ok := fakeDecide(c, questions); ok {
		return out, err
	}
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 30 * time.Second}
	}
	base := c.BaseURL
	if base == "" {
		base = EndpointFor(c.APIKey)
	}
	model := c.Model
	if model == "" {
		model = "jev-latest"
	}
	body, err := json.Marshal(map[string]any{
		"model":     model,
		"state":     state,
		"questions": questions,
	})
	if err != nil {
		return Response{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base, bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return Response{}, err
	}
	if res.StatusCode >= 300 {
		return Response{}, fmt.Errorf("jev http %d: %s", res.StatusCode, truncate(string(raw), 300))
	}
	var out Response
	if err := json.Unmarshal(raw, &out); err != nil {
		return Response{}, err
	}
	return out, nil
}

func DecodeChoice(raw json.RawMessage) (Choice, error) {
	var c Choice
	err := json.Unmarshal(raw, &c)
	return c, err
}

func DecodeScore(raw json.RawMessage) (Score, error) {
	var s Score
	err := json.Unmarshal(raw, &s)
	return s, err
}

func DecodeNoul(raw json.RawMessage) (Noul, error) {
	var n Noul
	err := json.Unmarshal(raw, &n)
	return n, err
}

func ChoiceQ(instructions string, criteria map[string]string) Question {
	raw, _ := json.Marshal(criteria)
	return Question{Type: "choice", Instructions: instructions, Criteria: raw}
}

func ScoreQ(instructions string, levels []string) Question {
	raw, _ := json.Marshal(levels)
	return Question{Type: "score", Instructions: instructions, Criteria: raw}
}

func NoulQ(instructions string) Question {
	return Question{Type: "noul", Instructions: instructions}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
