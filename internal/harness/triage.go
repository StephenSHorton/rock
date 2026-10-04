package harness

import (
	"context"
	"strings"

	"github.com/StephenSHorton/rock/internal/jev"
)

const (
	EvFilter    EventKind = "filter"
	EvClassify  EventKind = "classify"
	logClipSize           = 2000
)

func (h *Harness) ask(ctx context.Context, state any, questions []jev.Query) jev.Result {
	if h.Tools != nil && h.Tools.Env.Ask != nil {
		res, err := h.Tools.Env.Ask(ctx, state, questions)
		if err != nil {
			return jev.Result{Error: err.Error(), Answers: map[string]jev.Answer{}}
		}
		return res
	}
	return h.Gates.Ask(ctx, state, questions)
}

func (h *Harness) classifyFailure(ctx context.Context, output string) (hint string, ev Event) {
	if h.TriageOff {
		return "", Event{}
	}
	excerpt := jev.Clip(strings.TrimSpace(output), logClipSize)
	if excerpt == "" {
		return "", Event{}
	}
	res := h.ask(ctx, map[string]any{"log": excerpt}, jev.ClassifyQueries())
	line := jev.CompactClassify(res)
	if line == "" {
		return "", Event{}
	}
	return line, Event{Kind: EvClassify, Name: "classify", Text: line}
}

func filterEvent(name string, stats *jev.FilterStats) Event {
	if stats == nil || stats.Zero() {
		return Event{}
	}
	return Event{
		Kind:        EvFilter,
		Name:        name,
		Text:        stats.Text(),
		ClipsBefore: stats.ClipsBefore,
		ClipsAfter:  stats.ClipsAfter,
		BytesBefore: stats.BytesBefore,
		BytesAfter:  stats.BytesAfter,
	}
}
