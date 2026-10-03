#!/bin/sh
# Install the current Rock 1.0 build.
# Usage: curl -fsSL https://stephenshorton.github.io/rock/install.sh | bash
set -eu

if ! command -v go >/dev/null 2>&1; then
  echo "Rock needs Go 1.27 or newer: https://go.dev/dl/" >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required to find the current 1.0 commit." >&2
  exit 1
fi

ref="cursor/rock-1-0-a11b"
# Used only when the GitHub API does not answer. Move this when 1.0 moves.
fallback="09b1e7a060600951c2daf7b745ca61a0488ce877"
echo "Finding the current 1.0 commit..." >&2
sha=""
i=0
while [ "$i" -lt 3 ]; do
  sha=$(curl -fsSL -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/StephenSHorton/rock/commits/${ref}" \
    | sed -n 's/.*"sha": *"\([0-9a-f]\{40\}\)".*/\1/p' | head -n 1) || sha=""
  if [ -n "$sha" ]; then
    break
  fi
  i=$((i + 1))
  sleep 1
done

if [ -z "$sha" ]; then
  echo "GitHub did not answer. Installing the last known 1.0 commit." >&2
  sha="$fallback"
fi

echo "Installing rock ${sha}..." >&2
go install "github.com/StephenSHorton/rock/cmd/rock@${sha}"

bin="$(go env GOPATH)/bin"
echo "Installed ${bin}/rock" >&2
case ":$PATH:" in
  *":${bin}:"*) echo "Run: rock" >&2 ;;
  *) echo "Add ${bin} to PATH, then run: rock" >&2
     echo "  export PATH=\"${bin}:\$PATH\"" >&2 ;;
esac
