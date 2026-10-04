package jev

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Agent-facing modes. boolean is noul on the wire — Jev has no boolean type.
// The noul float is P(yes): the calibrated probability that the answer to
// the question is yes (https://jevtypesafeai.com/jev/noul). Display may
// say "yes"/"no"; the value itself stays the raw float.
const (
	ModeBoolean = "boolean"
	ModeChoice  = "choice"
	ModeScore   = "score"

	// FailedDetail is set on every answer when Decide does not return one.
	// Never invent a value to go with it.
	FailedDetail = "Jev call failed, question not answered"
)

// Query is one agent question. Ask batches them in a single Decide call.
type Query struct {
	Name     string
	Question string
	Mode     string
	Options  map[string]string // choice labels → meanings
	Levels   []string          // score scale, 2–10
}

// Answer is one typed result the model can act on.
type Answer struct {
	Mode          string             `json:"mode"`
	Question      string             `json:"question,omitempty"`
	Value         any                `json:"value,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Legend        map[string]string  `json:"legend,omitempty"`
	Reason        string             `json:"reason,omitempty"`
	Detail        string             `json:"detail,omitempty"`
}

// Result is the ask_jev tool payload.
type Result struct {
	Source  string            `json:"source"`
	Model   string            `json:"model,omitempty"`
	Answers map[string]Answer `json:"answers"`
	Error   string            `json:"error,omitempty"`
	Usage   *Usage            `json:"usage,omitempty"`
	Clips   []FileClip        `json:"clips,omitempty"`
	Filter  *FilterStats      `json:"filter,omitempty"`
}

// Usage is optional. The public API documents it; a missing field is not an error.
type Usage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

func (q Query) agentMode() string {
	switch strings.ToLower(strings.TrimSpace(q.Mode)) {
	case ModeBoolean, "noul", "yes", "yesno", "yes/no":
		return ModeBoolean
	case ModeChoice:
		return ModeChoice
	case ModeScore:
		return ModeScore
	default:
		return strings.ToLower(strings.TrimSpace(q.Mode))
	}
}

// ValidateQuery checks one question before any HTTP call.
func ValidateQuery(q Query) error {
	if strings.TrimSpace(q.Question) == "" {
		return fmt.Errorf("question is required")
	}
	switch q.agentMode() {
	case ModeBoolean:
		return nil
	case ModeChoice:
		if len(q.Options) == 0 {
			return fmt.Errorf("choice %q needs an options list", q.Name)
		}
		if len(q.Options) > 255 {
			return fmt.Errorf("choice %q has %d options; Jev choice allows at most 255", q.Name, len(q.Options))
		}
		return nil
	case ModeScore:
		if len(q.Levels) < 2 {
			return fmt.Errorf("score %q needs a range or at least two levels", q.Name)
		}
		if len(q.Levels) > 10 {
			return fmt.Errorf("score %q has %d levels; Jev score allows 2-10", q.Name, len(q.Levels))
		}
		return nil
	default:
		return fmt.Errorf("unknown mode %q (want boolean, choice, or score)", q.Mode)
	}
}

func ValidateQueries(questions []Query) error {
	if len(questions) == 0 {
		return fmt.Errorf("ask_jev needs at least one question")
	}
	seen := map[string]bool{}
	for i, q := range questions {
		if strings.TrimSpace(q.Name) == "" {
			return fmt.Errorf("questions[%d] needs a name", i)
		}
		if seen[q.Name] {
			return fmt.Errorf("duplicate question name %q", q.Name)
		}
		seen[q.Name] = true
		if err := ValidateQuery(q); err != nil {
			return err
		}
	}
	return nil
}

func (q Query) wireQuestion() Question {
	switch q.agentMode() {
	case ModeChoice:
		return ChoiceQ(q.Question, q.Options)
	case ModeScore:
		return ScoreQ(q.Question, q.Levels)
	default:
		return NoulQ(q.Question)
	}
}

// Ask runs questions in one Decide call. A failed call sets Error and
// leaves every value empty — never a fabricated answer. Key-less process
// startup is not this function's job; if we are called without a live
// client, that is a runtime error like any other Decide failure.
func (g Gates) Ask(ctx context.Context, state any, questions []Query) Result {
	qs := make(map[string]Question, len(questions))
	for _, q := range questions {
		qs[q.Name] = q.wireQuestion()
	}
	if g.Client == nil {
		return askFailed(questions, "jev client is not wired")
	}
	res, err := g.Client.Decide(ctx, state, qs)
	if err != nil {
		return askFailed(questions, err.Error())
	}
	out := Result{Source: "live", Model: res.Model, Answers: make(map[string]Answer, len(questions))}
	if res.Usage != nil && (res.Usage.InputTokens != 0 || res.Usage.OutputTokens != 0) {
		out.Usage = res.Usage
	}
	for _, q := range questions {
		raw, ok := res.Answers[q.Name]
		if !ok {
			out.Answers[q.Name] = Answer{Mode: q.agentMode(), Question: q.Question, Detail: "Jev returned no answer"}
			continue
		}
		out.Answers[q.Name] = decodeAsk(q, raw)
	}
	return out
}

func askFailed(questions []Query, err string) Result {
	out := Result{Error: err, Answers: make(map[string]Answer, len(questions))}
	for _, q := range questions {
		out.Answers[q.Name] = Answer{Mode: q.agentMode(), Question: q.Question, Detail: FailedDetail}
	}
	return out
}

func decodeAsk(q Query, raw json.RawMessage) Answer {
	a := Answer{Mode: q.agentMode(), Question: q.Question, Reason: extraReason(raw)}
	switch q.agentMode() {
	case ModeChoice:
		ch, err := DecodeChoice(raw)
		if err != nil {
			a.Detail = "decode: " + err.Error()
			return a
		}
		a.Value = ch.Choice
		a.Confidence = ch.Confidence
		a.Probabilities = ch.Probabilities
	case ModeScore:
		s, err := DecodeScore(raw)
		if err != nil {
			a.Detail = "decode: " + err.Error()
			return a
		}
		a.Value = s.Score
		a.Confidence = s.Confidence
		a.Probabilities = s.Probabilities
		a.Legend = s.Legend
	default:
		n, err := DecodeNoul(raw)
		if err != nil {
			a.Detail = "decode: " + err.Error()
			return a
		}
		a.Value = n.Noul
	}
	return a
}

// Line is the shared EvJev text. One call is one block: a source line,
// then each question on its own row (name, mode, question, answer).
// Failed calls show error= and no name=value tokens.
func (r Result) Line() string {
	var b strings.Builder
	if r.Source != "" {
		b.WriteString(r.Source)
	} else {
		b.WriteString("jev")
	}
	if r.Error != "" {
		b.WriteString(" error=")
		b.WriteString(r.Error)
	}
	names := make([]string, 0, len(r.Answers))
	for name := range r.Answers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		a := r.Answers[name]
		b.WriteByte('\n')
		if a.Mode != "" {
			b.WriteString(a.Mode)
			b.WriteByte(' ')
		}
		b.WriteString(name)
		if a.Value != nil && r.Error == "" {
			b.WriteByte('=')
			fmt.Fprint(&b, a.Value)
			if a.Mode == ModeChoice && a.Confidence > 0 {
				fmt.Fprintf(&b, " %.2f", a.Confidence)
			}
			if a.Mode == ModeScore {
				if label := scoreLegend(a); label != "" {
					b.WriteByte(' ')
					b.WriteString(label)
				}
			}
		} else if a.Detail != "" {
			b.WriteString(": ")
			b.WriteString(a.Detail)
		}
		if q := strings.TrimSpace(a.Question); q != "" {
			b.WriteString(" · ")
			b.WriteString(oneLine(q))
		}
	}
	return b.String()
}

func scoreLegend(a Answer) string {
	if len(a.Legend) == 0 {
		return ""
	}
	keys := []string{fmt.Sprint(a.Value)}
	if n, ok := AnswerFloat(a); ok {
		keys = append(keys, fmt.Sprintf("%.0f", n), fmt.Sprintf("%d", int(n)))
	}
	for _, k := range keys {
		if s := strings.TrimSpace(a.Legend[k]); s != "" {
			return s
		}
	}
	names := make([]string, 0, len(a.Legend))
	for k := range a.Legend {
		names = append(names, k)
	}
	sort.Strings(names)
	var parts []string
	seen := map[string]bool{}
	for _, k := range names {
		s := strings.TrimSpace(a.Legend[k])
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		parts = append(parts, s)
	}
	return strings.Join(parts, "/")
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}

func AnswerString(a Answer) (string, bool) {
	if a.Value == nil {
		return "", false
	}
	s, ok := a.Value.(string)
	if !ok {
		return fmt.Sprint(a.Value), true
	}
	return s, true
}

func AnswerFloat(a Answer) (float64, bool) {
	if a.Value == nil {
		return 0, false
	}
	switch v := a.Value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	default:
		return 0, false
	}
}

func extraReason(raw json.RawMessage) string {
	var extra struct {
		Reason      string `json:"reason"`
		Explanation string `json:"explanation"`
	}
	if json.Unmarshal(raw, &extra) != nil {
		return ""
	}
	if extra.Reason != "" {
		return extra.Reason
	}
	return extra.Explanation
}
