package update

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// AssetName is the release archive name for one OS/arch.
func AssetName(version, goos, goarch string) string {
	v := Normalize(version)
	if goos == "windows" {
		return fmt.Sprintf("rock_%s_%s_%s.zip", v, goos, goarch)
	}
	return fmt.Sprintf("rock_%s_%s_%s.tar.gz", v, goos, goarch)
}

// SelectAsset picks the archive for goos/goarch from a release asset list.
func SelectAsset(assets []Asset, version, goos, goarch string) (Asset, error) {
	want := AssetName(version, goos, goarch)
	for _, a := range assets {
		if a.Name == want {
			return a, nil
		}
	}
	return Asset{}, fmt.Errorf("no release asset for %s/%s (want %s)", goos, goarch, want)
}

// ChecksumsFile is the name of the SHA-256 list on a release.
const ChecksumsFile = "checksums.txt"

// ParseChecksums reads `hex  filename` lines.
func ParseChecksums(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sum := strings.ToLower(fields[0])
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		out[name] = sum
	}
	return out, sc.Err()
}

// VerifySHA256 checks the file against a lowercase hex digest.
func VerifySHA256(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, strings.TrimSpace(want)) {
		return fmt.Errorf("checksum mismatch: got %s want %s", got, want)
	}
	return nil
}
