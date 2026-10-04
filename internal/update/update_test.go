package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"v1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.2", "1.0.1", 1},
		{"dev", "1.0.0", -1},
		{"1.0.0", "dev", 1},
		{"1.0", "1.0.0", 0},
		{"1.1.0", "1.0.9", 1},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Fatalf("Compare(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestAssetNameAndSelect(t *testing.T) {
	if got := AssetName("v1.0.2", "linux", "amd64"); got != "rock_1.0.2_linux_amd64.tar.gz" {
		t.Fatal(got)
	}
	if got := AssetName("1.0.2", "windows", "amd64"); got != "rock_1.0.2_windows_amd64.zip" {
		t.Fatal(got)
	}
	assets := []Asset{
		{Name: "checksums.txt", URL: "http://x/checksums.txt"},
		{Name: "rock_1.0.2_linux_arm64.tar.gz", URL: "http://x/arm"},
		{Name: "rock_1.0.2_linux_amd64.tar.gz", URL: "http://x/amd"},
	}
	got, err := SelectAsset(assets, "1.0.2", "linux", "amd64")
	if err != nil || got.URL != "http://x/amd" {
		t.Fatalf("%#v %v", got, err)
	}
	if _, err := SelectAsset(assets, "1.0.2", "darwin", "arm64"); err == nil {
		t.Fatal("expected missing asset")
	}
}

func TestParseAndVerifyChecksum(t *testing.T) {
	sums, err := ParseChecksums(strings.NewReader("abc123  rock_1.0.2_linux_amd64.tar.gz\n# skip\n"))
	if err != nil || sums["rock_1.0.2_linux_amd64.tar.gz"] != "abc123" {
		t.Fatalf("%#v %v", sums, err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "blob")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256([]byte("hello"))
	want := hex.EncodeToString(h[:])
	if err := VerifySHA256(path, want); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256(path, "deadbeef"); err == nil {
		t.Fatal("expected mismatch")
	}
}

func TestApplyDownloadsAndReplaces(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	payload := []byte("rock-bin-v1.0.2")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "rock", Mode: 0755, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	archive := buf.Bytes()
	sum := sha256.Sum256(archive)
	name := "rock_1.0.2_linux_amd64.tar.gz"
	checksums := hex.EncodeToString(sum[:]) + "  " + name + "\n"

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/StephenSHorton/rock/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name":"v1.0.2","assets":[
			{"name":%q,"browser_download_url":%q},
			{"name":"checksums.txt","browser_download_url":%q}
		]}`, name, "http://"+r.Host+"/files/"+name, "http://"+r.Host+"/files/checksums.txt")
	})
	mux.HandleFunc("/files/"+name, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/files/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(checksums))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	dest := filepath.Join(t.TempDir(), "rock")
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	c := &Client{API: srv.URL, HTTP: srv.Client(), GOOS: "linux", GOARCH: "amd64"}
	got, err := c.Apply(context.Background(), dest)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.0.2" {
		t.Fatalf("version %s", got)
	}
	raw, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(payload) {
		t.Fatalf("dest %q", raw)
	}
}

func TestApplyRejectsBadChecksum(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	name := "rock_1.0.2_linux_amd64.tar.gz"
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/StephenSHorton/rock/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name":"v1.0.2","assets":[
			{"name":%q,"browser_download_url":%q},
			{"name":"checksums.txt","browser_download_url":%q}
		]}`, name, "http://"+r.Host+"/files/"+name, "http://"+r.Host+"/files/checksums.txt")
	})
	mux.HandleFunc("/files/"+name, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-a-tar"))
	})
	mux.HandleFunc("/files/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("0000  " + name + "\n"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := &Client{API: srv.URL, HTTP: srv.Client(), GOOS: "linux", GOARCH: "amd64"}
	if _, err := c.Apply(context.Background(), filepath.Join(t.TempDir(), "rock")); err == nil {
		t.Fatal("expected checksum failure")
	}
}

func TestDisabledReason(t *testing.T) {
	t.Setenv("ROCK_NO_UPDATE", "")
	t.Setenv("ROCK_DISABLE_AUTOUPDATE", "")
	t.Setenv("CI", "")
	t.Setenv("ROCK_TEST_FAKE_JEV", "")
	if DisabledReason() != "" {
		t.Fatalf("%q", DisabledReason())
	}
	t.Setenv("ROCK_NO_UPDATE", "1")
	if DisabledReason() != "ROCK_NO_UPDATE" {
		t.Fatal(DisabledReason())
	}
	t.Setenv("ROCK_NO_UPDATE", "")
	t.Setenv("CI", "true")
	if DisabledReason() != "CI" {
		t.Fatal(DisabledReason())
	}
	t.Setenv("CI", "")
	t.Setenv("ROCK_TEST_FAKE_JEV", "1")
	if DisabledReason() != "ROCK_TEST_FAKE_JEV" {
		t.Fatal(DisabledReason())
	}
}

func TestCheckUsesCache(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	writeCache("1.0.0", "1.0.2")
	n, ok := CachedNotice("1.0.0")
	if !ok || !n.Newer || n.Line() != "Rock v1.0.2 available, run rock update" {
		t.Fatalf("%#v %v", n, ok)
	}
	if AutoCheck() {
		t.Fatal("tests must not auto-check")
	}
}

func TestRefreshHitsAPI(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/StephenSHorton/rock/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v1.0.3","assets":[]}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := &Client{API: srv.URL, HTTP: srv.Client()}
	n, err := Refresh(context.Background(), "1.0.0", c)
	if err != nil {
		t.Fatal(err)
	}
	if !n.Newer || n.Latest != "1.0.3" || n.Line() != "Rock v1.0.3 available, run rock update" {
		t.Fatalf("%#v", n)
	}
	cached, ok := CachedNotice("1.0.0")
	if !ok || cached.Latest != "1.0.3" {
		t.Fatalf("cache %#v %v", cached, ok)
	}
}

func TestGoInstallHint(t *testing.T) {
	if !strings.Contains(GoInstallHint(), "github.com/StephenSHorton/rock/cmd/rock@latest") {
		t.Fatal(GoInstallHint())
	}
	t.Setenv("GOPATH", t.TempDir())
	bin := filepath.Join(os.Getenv("GOPATH"), "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(bin, "rock")
	if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !InstalledByGo(exe) {
		t.Fatal("GOPATH/bin should count as go install")
	}
}
