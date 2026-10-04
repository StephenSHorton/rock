package suzuri

import "testing"

func TestSequence(t *testing.T) {
	got := Sequence(Fork{
		NewSession: true,
		SessionID:  "abc-123",
		Bin:        "/usr/bin/rock",
		Prompt:     "try this",
		Title:      "review",
		CWD:        "/work",
	})
	want := "\x1b]7880;new=1;session=abc-123;bin=%2Fusr%2Fbin%2Frock;cwd=%2Fwork;prompt=try%20this;title=review;brand=rock\a"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestForkResume(t *testing.T) {
	got := Sequence(Fork{SessionID: "s", Bin: "/bin/rock"})
	if got != "\x1b]7880;fork=1;session=s;bin=%2Fbin%2Frock;brand=rock\a" {
		t.Fatal(got)
	}
}
