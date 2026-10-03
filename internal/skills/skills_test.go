package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".agents", "skills", "release")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: release\ndescription: cut a release\n---\n\nTag and push.\n"
	if err := os.WriteFile(filepath.Join(p, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "release" || got[0].Description != "cut a release" || got[0].Body != "Tag and push." {
		t.Fatalf("%#v", got)
	}
}
