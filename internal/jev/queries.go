package jev

// Shared question builders. Agent ask_jev and the Go gates use the same Query
// type and Ask encoder. Risk is still invoked from Go — never optional.

func TurnQueries(skillNames []string) []Query {
	qs := []Query{
		{
			Name:     "model",
			Question: "Which model should complete this coding turn?",
			Mode:     ModeChoice,
			Options: map[string]string{
				"fast":   "small edit, lookup, or a narrow question",
				"strong": "design, multi-file change, or ambiguous debugging",
			},
		},
		{
			Name:     "stuck",
			Question: "Are these recent tool calls repeating without progress?",
			Mode:     ModeBoolean,
		},
		{
			Name:     "weight",
			Question: "How heavy is this transcript?",
			Mode:     ModeScore,
			Levels:   []string{"light", "fine", "tight", "compact now"},
		},
	}
	if len(skillNames) == 0 {
		return qs
	}
	crit := map[string]string{"none": "no skill applies"}
	for _, name := range skillNames {
		if len(crit) >= 12 {
			break
		}
		crit[name] = "a discovered skill"
	}
	qs = append(qs, Query{
		Name:     "skill",
		Question: "Which single skill is most relevant? Prefer none when unsure.",
		Mode:     ModeChoice,
		Options:  crit,
	})
	return qs
}

func RiskQuery() Query {
	return Query{
		Name:     "risk",
		Question: "Is this tool call destructive or hard to undo?",
		Mode:     ModeBoolean,
	}
}

func SubagentQuery() Query {
	return Query{
		Name:     "kind",
		Question: "What kind of subagent should run?",
		Mode:     ModeChoice,
		Options: map[string]string{
			"explore": "read-only search",
			"plan":    "write a plan, do not edit the repo",
			"general": "may edit and run commands",
		},
	}
}

func ReadyQuery() Query {
	return Query{
		Name:     "ready",
		Question: "Is this plan specific enough to implement?",
		Mode:     ModeBoolean,
	}
}
