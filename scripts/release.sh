#!/usr/bin/env bash
# Cross-compile tagged Rock archives for GitHub Releases.
# Usage: scripts/release.sh 1.0.1
set -euo pipefail

VERSION="${1:-}"
VERSION="${VERSION#v}"
if [[ -z "$VERSION" ]]; then
  echo "usage: scripts/release.sh 1.0.1" >&2
  exit 1
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="${ROOT}/dist"
LDFLAGS="-s -w -X github.com/StephenSHorton/rock/internal/version.Version=${VERSION} -X github.com/StephenSHorton/rock/internal/version.Source=release"
mkdir -p "$OUT"
rm -f "$OUT"/rock_* "$OUT"/checksums.txt

build() {
  local goos="$1" goarch="$2"
  local tmp
  tmp="$(mktemp -d)"
  local bin="rock"
  if [[ "$goos" == windows ]]; then
    bin="rock.exe"
  fi
  echo "building ${goos}/${goarch}" >&2
  (
    cd "$ROOT"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
      go build -trimpath -ldflags "$LDFLAGS" -o "${tmp}/${bin}" ./cmd/rock
  )
  if [[ "$goos" == windows ]]; then
    local name="rock_${VERSION}_${goos}_${goarch}.zip"
    (cd "$tmp" && zip -q "$OUT/$name" "$bin")
  else
    local name="rock_${VERSION}_${goos}_${goarch}.tar.gz"
    tar -C "$tmp" -czf "$OUT/$name" "$bin"
  fi
  rm -rf "$tmp"
}

build darwin amd64
build darwin arm64
build linux amd64
build linux arm64
build windows amd64

(
  cd "$OUT"
  sha256sum rock_*.tar.gz rock_*.zip > checksums.txt
)

echo "wrote $OUT" >&2
ls -l "$OUT" >&2
