package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"debug/buildinfo"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/StephenSHorton/rock/internal/version"
)

const modulePath = "github.com/StephenSHorton/rock/cmd/rock"

// InstalledByGo reports a `go install` binary: GOPATH/bin, or build info
// that names a module version (not a local devel build).
func InstalledByGo(exe string) bool {
	// Release archives stamp Source=release. Go 1.24+ also stamps the VCS
	// tag into build info, so without this a release binary looks go-installed.
	if version.Source == "release" {
		return false
	}
	abs, err := filepath.Abs(exe)
	if err == nil {
		if dir := goBin(); dir != "" {
			if filepath.Clean(filepath.Dir(abs)) == filepath.Clean(dir) {
				return true
			}
		}
	}
	if info, err := buildinfo.ReadFile(exe); err == nil && moduleVersion(info.Main.Version) {
		return true
	}
	if info, ok := debug.ReadBuildInfo(); ok && moduleVersion(info.Main.Version) {
		return true
	}
	return false
}

func moduleVersion(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" || v == "(devel)" || v == "devel" || v == "dev" {
		return false
	}
	return strings.HasPrefix(v, "v") || numbered(Normalize(v))
}

func goBin() string {
	gopath := strings.TrimSpace(os.Getenv("GOPATH"))
	if gopath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		gopath = filepath.Join(home, "go")
	}
	return filepath.Join(strings.Split(gopath, string(os.PathListSeparator))[0], "bin")
}

// GoInstallHint is the command for a go-install tree.
func GoInstallHint() string {
	return "go install " + modulePath + "@latest"
}

// Apply downloads the latest matching asset, verifies its checksum, and
// replaces dest. dest is usually the running executable.
func (c *Client) Apply(ctx context.Context, dest string) (string, error) {
	rel, err := c.Latest(ctx)
	if err != nil {
		return "", err
	}
	asset, err := SelectAsset(rel.Assets, rel.Version(), c.goos(), c.goarch())
	if err != nil {
		return "", err
	}
	sumsURL := ""
	for _, a := range rel.Assets {
		if a.Name == ChecksumsFile {
			sumsURL = a.URL
			break
		}
	}
	if sumsURL == "" {
		return "", fmt.Errorf("release %s has no %s", rel.Tag, ChecksumsFile)
	}

	dir, err := os.MkdirTemp("", "rock-update-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)

	archive := filepath.Join(dir, asset.Name)
	sumPath := filepath.Join(dir, ChecksumsFile)
	if err := c.Fetch(ctx, asset.URL, archive); err != nil {
		return "", err
	}
	if err := c.Fetch(ctx, sumsURL, sumPath); err != nil {
		return "", err
	}
	f, err := os.Open(sumPath)
	if err != nil {
		return "", err
	}
	sums, err := ParseChecksums(f)
	f.Close()
	if err != nil {
		return "", err
	}
	want, ok := sums[asset.Name]
	if !ok {
		return "", fmt.Errorf("%s is missing from %s", asset.Name, ChecksumsFile)
	}
	if err := VerifySHA256(archive, want); err != nil {
		return "", err
	}
	bin, err := extractBinary(archive, dir)
	if err != nil {
		return "", err
	}
	if err := os.Chmod(bin, 0o755); err != nil {
		return "", err
	}
	if dest == "" {
		dest, err = os.Executable()
		if err != nil {
			return "", err
		}
		if resolved, err := filepath.EvalSymlinks(dest); err == nil {
			dest = resolved
		}
	}
	staged := dest + ".new"
	if err := copyFile(bin, staged); err != nil {
		return "", err
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		_ = os.Remove(staged)
		return "", err
	}
	if err := replaceFile(staged, dest); err != nil {
		_ = os.Remove(staged)
		return "", err
	}
	return rel.Version(), nil
}

func extractBinary(archive, destDir string) (string, error) {
	name := archive
	if strings.HasSuffix(strings.ToLower(archive), ".zip") {
		return unzipRock(archive, destDir)
	}
	if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") {
		return untarRock(archive, destDir)
	}
	return "", fmt.Errorf("unknown archive %s", filepath.Base(archive))
}

func untarRock(path, destDir string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		base := filepath.Base(hdr.Name)
		if hdr.Typeflag != tar.TypeReg || (base != "rock" && base != "rock.exe") {
			continue
		}
		out := filepath.Join(destDir, base)
		if err := writeFile(out, tr, 0o755); err != nil {
			return "", err
		}
		return out, nil
	}
	return "", fmt.Errorf("archive has no rock binary")
}

func unzipRock(path, destDir string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	for _, f := range zr.File {
		base := filepath.Base(f.Name)
		if f.FileInfo().IsDir() || (base != "rock" && base != "rock.exe") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		out := filepath.Join(destDir, base)
		err = writeFile(out, rc, 0o755)
		rc.Close()
		if err != nil {
			return "", err
		}
		return out, nil
	}
	return "", fmt.Errorf("archive has no rock binary")
}

func writeFile(path string, r io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	return writeFile(dest, in, 0o755)
}
