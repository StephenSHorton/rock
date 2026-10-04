package perms

import "testing"

func TestPlanBlocksEditsEvenUnderRules(t *testing.T) {
	p := Policy{Mode: ModePlan, Allow: []string{"edit_file", "shell", "*"}}
	d, _ := p.Decide("edit_file", "main.go")
	if d != Deny {
		t.Fatalf("edit: %s", d)
	}
	d, _ = p.Decide("edit_file", "plan.md")
	if d != Allow {
		t.Fatalf("plan.md: %s", d)
	}
	d, _ = p.Decide("shell", "ls")
	if d != Deny {
		t.Fatalf("shell: %s", d)
	}
	d, _ = p.Decide("spawn_subagent", "explore")
	if d != Allow {
		t.Fatalf("explore: %s", d)
	}
	d, _ = p.Decide("spawn_subagent", "general")
	if d != Deny {
		t.Fatalf("general: %s", d)
	}
}

func TestYoloDoesNotOverridePlan(t *testing.T) {
	p := Policy{Mode: ModePlan}
	d, why := p.Decide("write_file", "a.go")
	if d != Deny || why == "" {
		t.Fatalf("%s %s", d, why)
	}
}

func TestYoloSkipsAsk(t *testing.T) {
	p := Policy{Mode: ModeYolo}
	d, _ := p.Decide("shell", "ls")
	if d != Allow {
		t.Fatal(d)
	}
}

func TestDenyRuleWins(t *testing.T) {
	p := Policy{Mode: ModeYolo, Deny: []string{"shell:rm *"}}
	d, _ := p.Decide("shell", "rm -rf /")
	if d != Deny {
		t.Fatal(d)
	}
	d, _ = p.Decide("shell", "ls")
	if d != Allow {
		t.Fatal(d)
	}
}

func TestAskJevAllowedInPlanMode(t *testing.T) {
	p := Policy{Mode: ModePlan}
	d, _ := p.Decide("ask_jev", "boolean  resolved?")
	if d != Allow {
		t.Fatalf("ask_jev in plan: %s", d)
	}
	d, _ = p.Decide("jev_decide", "old")
	if d != Ask && d != Deny {
		t.Fatalf("retired jev_decide should not be a read-only default, got %s", d)
	}
}

func TestAskJevNeverPrompts(t *testing.T) {
	if Mutates("ask_jev") {
		t.Fatal("ask_jev is read-only")
	}
	for _, mode := range []Mode{ModeDefault, ModePlan} {
		p := Policy{Mode: mode, Ask: []string{"ask_jev", "*"}}
		d, why := p.Decide("ask_jev", "boolean  ok?")
		if d != Allow {
			t.Fatalf("mode %s prompted: %s %s", mode, d, why)
		}
	}
	p := Policy{Mode: ModeDefault, Deny: []string{"ask_jev"}}
	if d, _ := p.Decide("ask_jev", "ask"); d != Deny {
		t.Fatalf("explicit deny should still win: %s", d)
	}
}

func TestReviewOnly(t *testing.T) {
	p := Policy{Mode: ModeYolo, ReviewOnly: true}
	d, _ := p.Decide("edit_file", "a.go")
	if d != Deny {
		t.Fatal(d)
	}
	d, _ = p.Decide("read_file", "a.go")
	if d != Allow {
		t.Fatal(d)
	}
}
