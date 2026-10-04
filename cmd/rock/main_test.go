package main

import (
	"strings"
	"testing"
)

func TestHelpReturnsNil(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}, {"help"}} {
		if err := run(args); err != nil {
			t.Fatalf("run(%q) = %v", args, err)
		}
	}
}

func TestLoginLogoutNeedChatGPT(t *testing.T) {
	if err := run([]string{"login"}); err == nil || !strings.Contains(err.Error(), "rock login chatgpt") {
		t.Fatalf("login: %v", err)
	}
	if err := run([]string{"logout", "codex"}); err == nil || !strings.Contains(err.Error(), "rock logout chatgpt") {
		t.Fatalf("logout: %v", err)
	}
}
