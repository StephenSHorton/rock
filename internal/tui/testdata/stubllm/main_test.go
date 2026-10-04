package main

import (
	"strings"
	"testing"
)

func TestRespondRoutesPlanSpawnAndChildText(t *testing.T) {
	tool := func(msgs []message) string {
		reply := respond(msgs)
		calls, _ := reply["tool_calls"].([]any)
		if len(calls) == 0 {
			return ""
		}
		fn := calls[0].(map[string]any)["function"].(map[string]any)
		return fn["name"].(string)
	}
	if name := tool([]message{{Role: "user", Content: "write a plan to fix the typo"}}); name != "update_plan" {
		t.Fatalf("plan prompt: %s", name)
	}
	if name := tool([]message{{Role: "user", Content: "use a subagent to explore greeting.txt"}}); name != "spawn_subagent" {
		t.Fatalf("subagent prompt: %s", name)
	}
	if name := tool([]message{{Role: "user", Content: "fix the typo in greeting.txt"}}); name != "edit_file" {
		t.Fatalf("edit prompt: %s", name)
	}
	child := respond([]message{{Role: "user", Content: childTask}})
	if child["content"] == "" || child["tool_calls"] != nil {
		t.Fatalf("child should answer in text: %#v", child)
	}
	recap := afterTool("updated plan")
	if !strings.Contains(recap, "Plan written") {
		t.Fatal(recap)
	}
	if !strings.Contains(afterTool("[explore] greeting.txt is one line"), "Subagent finished") {
		t.Fatal("spawn recap")
	}
}
