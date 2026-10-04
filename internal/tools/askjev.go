package tools

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/StephenSHorton/rock/internal/jev"
)

func askJevSpec(obj func(map[string]any, []string) map[string]any, str func(string) map[string]any) map[string]any {
	question := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":     str("key in the answers map"),
			"question": str("the decision to make"),
			"mode":     str("boolean, choice, or score"),
			"options":  map[string]any{"description": "choice options: a list of labels, or {label: meaning}"},
			"criteria": map[string]any{"description": "alias of options (choice map) or score levels"},
			"levels":   map[string]any{"description": "score levels, 2-10 strings. This is what Jev score takes."},
			"range":    map[string]any{"description": "score [min, max] integers, mapped onto Jev levels"},
			"rubric":   str("score rubric; folded into the question. Jev has no rubric field."),
		},
		"required": []string{"question", "mode"},
	}
	return obj(map[string]any{
		"state":     map[string]any{"description": "facts the questions are about (string or object)"},
		"context":   map[string]any{"description": "extra fields merged into state"},
		"question":  str("single question if not using questions[]"),
		"mode":      str("boolean, choice, or score"),
		"name":      str("answer key for a single question, default q"),
		"options":   map[string]any{"description": "choice options list or map"},
		"criteria":  map[string]any{"description": "alias of options or score levels"},
		"levels":    map[string]any{"description": "score levels, 2-10 strings"},
		"range":     map[string]any{"description": "score [min, max] integers"},
		"rubric":    str("score rubric; folded into the question"),
		"questions": map[string]any{"type": "array", "description": "batch; one Decide call", "items": question},
		"paths":     map[string]any{"description": "workspace files Rock reads into state. Jev does not open files."},
		"filter":    map[string]any{"type": "boolean", "description": "keep only clips Jev marks relevant in the tool result"},
	}, nil)
}

func parseAsk(raw map[string]any) (state any, questions []jev.Query, err error) {
	if raw == nil {
		return nil, nil, fmt.Errorf("ask_jev needs at least one question")
	}
	state = mergeState(raw["state"], raw["context"])
	hasPaths := raw["paths"] != nil
	if list, ok := raw["questions"]; ok && list != nil {
		arr, ok := list.([]any)
		if !ok {
			return nil, nil, fmt.Errorf("questions: want an array")
		}
		if len(arr) == 0 {
			return nil, nil, fmt.Errorf("ask_jev needs at least one question")
		}
		questions = make([]jev.Query, 0, len(arr))
		for i, item := range arr {
			m, ok := asMap(item)
			if !ok {
				return nil, nil, fmt.Errorf("questions[%d]: want an object", i)
			}
			q, err := parseQuery(m, "q"+strconv.Itoa(i+1))
			if err != nil {
				return nil, nil, err
			}
			questions = append(questions, q)
		}
	} else if _, ok := raw["question"]; ok {
		q, err := parseQuery(raw, "q")
		if err != nil {
			return nil, nil, err
		}
		questions = []jev.Query{q}
	} else if !hasPaths {
		return nil, nil, fmt.Errorf("ask_jev needs at least one question")
	}
	if len(questions) > 0 {
		if err := jev.ValidateQueries(questions); err != nil {
			return nil, nil, err
		}
	}
	return state, questions, nil
}

func parseQuery(raw map[string]any, fallback string) (jev.Query, error) {
	q := jev.Query{
		Name:     strings.TrimSpace(asString(raw["name"])),
		Question: strings.TrimSpace(asString(raw["question"])),
		Mode:     strings.TrimSpace(asString(raw["mode"])),
	}
	if q.Name == "" {
		q.Name = fallback
	}
	if rub := strings.TrimSpace(asString(raw["rubric"])); rub != "" {
		if q.Question == "" {
			q.Question = rub
		} else {
			q.Question = q.Question + "\nRubric: " + rub
		}
	}
	mode := queryMode(q)
	if v, ok := first(raw, "options", "criteria"); ok && mode != jev.ModeScore {
		opts, err := parseOptions(v)
		if err != nil {
			return q, err
		}
		q.Options = opts
	}
	if mode == jev.ModeScore {
		if v, ok := raw["levels"]; ok {
			levels, err := parseLevels(v)
			if err != nil {
				return q, err
			}
			q.Levels = levels
		}
		if len(q.Levels) == 0 {
			if v, ok := raw["criteria"]; ok {
				if _, isMap := asMap(v); !isMap {
					levels, err := parseLevels(v)
					if err != nil {
						return q, err
					}
					q.Levels = levels
				}
			}
		}
		if len(q.Levels) == 0 {
			if v, ok := raw["range"]; ok {
				min, max, err := parseRange(v)
				if err != nil {
					return q, err
				}
				q.Levels, err = levelsFromRange(min, max)
				if err != nil {
					return q, err
				}
			}
		}
	}
	if mode == jev.ModeChoice && len(q.Options) == 0 {
		if v, ok := raw["criteria"]; ok {
			opts, err := parseOptions(v)
			if err != nil {
				return q, err
			}
			q.Options = opts
		}
	}
	return q, nil
}

func queryMode(q jev.Query) string {
	switch strings.ToLower(strings.TrimSpace(q.Mode)) {
	case jev.ModeBoolean, "noul", "yes", "yesno", "yes/no":
		return jev.ModeBoolean
	case jev.ModeChoice:
		return jev.ModeChoice
	case jev.ModeScore:
		return jev.ModeScore
	default:
		return strings.ToLower(strings.TrimSpace(q.Mode))
	}
}

func mergeState(state, context any) any {
	if context == nil {
		return state
	}
	ctxMap, ok := asMap(context)
	if !ok {
		ctxMap = map[string]any{"context": context}
	}
	if state == nil {
		return ctxMap
	}
	if sm, ok := asMap(state); ok {
		out := make(map[string]any, len(sm)+len(ctxMap))
		for k, v := range sm {
			out[k] = v
		}
		for k, v := range ctxMap {
			out[k] = v
		}
		return out
	}
	out := map[string]any{"text": state}
	for k, v := range ctxMap {
		out[k] = v
	}
	return out
}

func parseOptions(v any) (map[string]string, error) {
	if m, ok := asMap(v); ok {
		out := make(map[string]string, len(m))
		for k, val := range m {
			out[k] = fmt.Sprint(val)
		}
		return out, nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("options: want a list or map")
	}
	out := make(map[string]string, len(arr))
	for i, item := range arr {
		switch it := item.(type) {
		case string:
			s := strings.TrimSpace(it)
			if s == "" {
				return nil, fmt.Errorf("options[%d] is empty", i)
			}
			out[s] = s
		case map[string]any:
			id := firstString(it, "id", "label", "name", "key")
			meaning := firstString(it, "meaning", "description", "detail")
			if id == "" {
				return nil, fmt.Errorf("options[%d] needs id or label", i)
			}
			if meaning == "" {
				meaning = id
			}
			out[id] = meaning
		default:
			return nil, fmt.Errorf("options[%d]: want string or object", i)
		}
	}
	return out, nil
}

func parseLevels(v any) ([]string, error) {
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("levels: want a list")
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s := strings.TrimSpace(fmt.Sprint(item))
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

func parseRange(v any) (min, max int, err error) {
	switch t := v.(type) {
	case []any:
		if len(t) != 2 {
			return 0, 0, fmt.Errorf("range: want [min, max]")
		}
		a, ok1 := asInt(t[0])
		b, ok2 := asInt(t[1])
		if !ok1 || !ok2 {
			return 0, 0, fmt.Errorf("range: min and max must be integers")
		}
		return a, b, nil
	case map[string]any:
		a, ok1 := asInt(t["min"])
		b, ok2 := asInt(t["max"])
		if !ok1 || !ok2 {
			return 0, 0, fmt.Errorf("range: min and max must be integers")
		}
		return a, b, nil
	default:
		return 0, 0, fmt.Errorf("range: want [min, max] or {min, max}")
	}
}

func levelsFromRange(min, max int) ([]string, error) {
	if max < min {
		return nil, fmt.Errorf("range: max < min")
	}
	n := max - min + 1
	if n < 2 {
		return nil, fmt.Errorf("score needs at least two levels")
	}
	if n > 10 {
		return nil, fmt.Errorf("score range has %d levels; Jev score allows 2-10", n)
	}
	out := make([]string, 0, n)
	for i := min; i <= max; i++ {
		out = append(out, strconv.Itoa(i))
	}
	return out, nil
}

func asMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n == float64(int(n)) {
			return int(n), true
		}
	}
	return 0, false
}

func first(raw map[string]any, keys ...string) (any, bool) {
	for _, k := range keys {
		if v, ok := raw[k]; ok && v != nil {
			return v, true
		}
	}
	return nil, false
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := strings.TrimSpace(asString(m[k])); s != "" {
			return s
		}
	}
	return ""
}

func parsePathList(v any) ([]string, error) {
	if v == nil {
		return nil, nil
	}
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil, nil
		}
		return []string{s}, nil
	case []any:
		out := make([]string, 0, len(t))
		for i, item := range t {
			s := strings.TrimSpace(fmt.Sprint(item))
			if s == "" {
				return nil, fmt.Errorf("paths[%d] is empty", i)
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("paths: want a string or list")
	}
}

func filterFlag(raw map[string]any) bool {
	if raw == nil {
		return false
	}
	switch v := raw["filter"].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true") || v == "1"
	default:
		return false
	}
}

func keepQueriesForClips(clips []jev.FileClip, question string) []jev.Query {
	if question == "" {
		question = "Does this clip help with the task in state?"
	}
	out := make([]jev.Query, len(clips))
	for i, c := range clips {
		out[i] = jev.KeepQuery(jev.KeepName(i), question+" (path "+c.Path+")")
	}
	return out
}

func attachClips(state any, clips []jev.FileClip) any {
	payload := make([]any, len(clips))
	for i, c := range clips {
		payload[i] = map[string]any{"path": c.Path, "text": c.Text}
	}
	return mergeState(state, map[string]any{"clips": payload})
}

func applyClipFilter(res jev.Result, clips []jev.FileClip, stats jev.FilterStats, min float64, includeKept bool) jev.Result {
	stats.ClipsBefore = len(clips)
	stats.BytesBefore = clipBytes(clips)
	if res.Error != "" {
		res.Filter = &stats
		return res
	}
	var kept []jev.FileClip
	matched := 0
	for i, c := range clips {
		keep, ok := jev.KeepValue(res.Answers[jev.KeepName(i)], min)
		if ok {
			matched++
			if keep {
				kept = append(kept, c)
			}
			continue
		}
		if a, ok := res.Answers[c.Path]; ok {
			keep, ok = jev.KeepValue(a, min)
			if ok {
				matched++
				if keep {
					kept = append(kept, c)
				}
			}
		}
	}
	if matched == 0 {
		// Custom questions, not per-clip keep. Do not dump the pile back
		// to the LLM — Jev already saw the clips.
		stats.ClipsAfter = 0
		stats.BytesAfter = 0
		res.Filter = &stats
		return res
	}
	stats.ClipsAfter = len(kept)
	stats.BytesAfter = clipBytes(kept)
	res.Filter = &stats
	if includeKept || matched > 0 {
		res.Clips = kept
	}
	return res
}

func clipBytes(clips []jev.FileClip) int {
	n := 0
	for _, c := range clips {
		n += len(c.Text)
	}
	return n
}

func askSummary(raw map[string]any) string {
	if list, ok := raw["questions"].([]any); ok && len(list) > 0 {
		if m, ok := asMap(list[0]); ok {
			mode := strings.TrimSpace(asString(m["mode"]))
			q := strings.TrimSpace(asString(m["question"]))
			if len(list) == 1 {
				return strings.TrimSpace(mode + "  " + q)
			}
			return fmt.Sprintf("%s  %s  (+%d)", mode, q, len(list)-1)
		}
	}
	return strings.TrimSpace(asString(raw["mode"]) + "  " + asString(raw["question"]))
}
