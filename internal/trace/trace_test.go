package trace

import (
	"strings"
	"testing"
	"time"
)

func TestInfoMarksStageAndLogs(t *testing.T) {
	var got []string
	SetLogger(func(level, msg string, kv ...any) { got = append(got, level+" "+msg) })
	defer SetLogger(nil)
	before := time.Now()
	Info("turn jev route start")
	name, at := Last()
	if name != "turn jev route start" || at.Before(before) {
		t.Fatalf("stage %q at %v", name, at)
	}
	Warn("w")
	if name2, _ := Last(); name2 != name {
		t.Fatal("warn must not move the stage")
	}
	Mark("grok chunk")
	if n, _ := Last(); n != "grok chunk" {
		t.Fatal(n)
	}
	if strings.Join(got, ",") != "info turn jev route start,warn w" {
		t.Fatal(got)
	}
}
