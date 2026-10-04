package main

import "testing"

func TestHelpReturnsNil(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}, {"help"}} {
		if err := run(args); err != nil {
			t.Fatalf("run(%q) = %v", args, err)
		}
	}
}
