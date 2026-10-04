package jev

import (
	"context"
	"strings"
	"testing"
)

func TestFakeDecideAcceptAndReject(t *testing.T) {
	t.Setenv("ROCK_TEST_FAKE_JEV", "1")
	c := &Client{APIKey: "rock-test"}
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("accept: %v", err)
	}
	out, err := c.Decide(context.Background(), map[string]string{"rock": "probe"}, map[string]Question{
		"ok": NoulQ("probe"),
	})
	if err != nil || out.Model != "jev-fake" {
		t.Fatalf("decide %v %+v", err, out)
	}

	t.Setenv("ROCK_TEST_FAKE_JEV", "reject")
	if err := c.Ping(context.Background()); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("reject: %v", err)
	}
}

func TestFakeDoesNotInterceptExplicitBaseURL(t *testing.T) {
	t.Setenv("ROCK_TEST_FAKE_JEV", "1")
	c := &Client{APIKey: "rock-test", BaseURL: "http://127.0.0.1:9/nope"}
	if _, err, ok := fakeDecide(c, map[string]Question{"ok": NoulQ("x")}); ok || err != nil {
		t.Fatalf("httptest path must win: ok=%v err=%v", ok, err)
	}
}
