package tui

import "testing"

func TestToolHeadingAskJev(t *testing.T) {
	verb, detail := toolHeading("ask_jev", `{"mode":"boolean","question":"ok?"}`, false)
	if verb != "Ask Jev" {
		t.Fatalf("title %q", verb)
	}
	if detail != "boolean  ok?" {
		t.Fatalf("detail %q", detail)
	}
	verb, _ = toolHeading("ask_jev", `{"paths":["a.go"]}`, true)
	if verb != "Asking Jev" {
		t.Fatalf("running title %q", verb)
	}
}
