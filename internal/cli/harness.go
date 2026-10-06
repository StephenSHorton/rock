package cli

import (
	"context"

	"github.com/StephenSHorton/rock/internal/harness"
	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/session"
	"github.com/StephenSHorton/rock/internal/tools"
)

// harnessEvent is the JSON shape Headless prints. It matches harness.Event.
type harnessEvent = harness.Event

func (a *App) harness(set *tools.Set, ask harness.AskFunc) *harness.Harness {
	return harness.New(harness.Options{
		Provider:    a.Provider,
		FastModel:   a.FastModel(),
		StrongModel: a.StrongModel(),
		Policy:      a.Policy(),
		Gates:       a.Gates,
		Tools:       set,
		Skills:      a.Skills,
		MaxSteps:    a.Loaded.File.MaxSteps,
		Checkpoint:  a.Checkpoint,
		Ask:         ask,
		NudgeEvery:  a.Loaded.File.Jev.NudgeInterval(),
		TriageOff:   !a.Loaded.File.Jev.TriageOn(),
		FilterOff:   !a.Loaded.File.Jev.FilterOn(),
		ClipBytes:   a.Loaded.File.Jev.ClipSize(),
	})
}

// RunTurn is one harness call for the TUI. Policy is read when the turn starts.
func (a *App) RunTurn(ctx context.Context, sess *session.Session, prompt string, ask harness.AskFunc, sink func(harness.Event)) error {
	a.ConnectMCP(ctx)
	set, err := a.NewHarness(sess.Meta.CWD)
	if err != nil {
		return err
	}
	set.Env.PlanPath = sess.PlanPath()
	h := a.harness(set, ask)
	h.Stream = true // the TUI renders thoughts and reply text live
	return h.Run(ctx, sess, prompt, sink)
}

// Factory is the ACP and HTTP entry. Each call gets its own tool set.
func (a *App) Factory(cwd string) (*harness.Harness, error) {
	a.ConnectMCP(context.Background())
	set, err := a.NewHarness(cwd)
	if err != nil {
		return nil, err
	}
	return a.harness(set, func(context.Context, string, string) perms.Decision {
		return perms.Deny
	}), nil
}
