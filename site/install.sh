#!/bin/sh
# Install rock from the main branch.
# Usage: curl -fsSL https://stephenshorton.github.io/rock/install.sh | bash
set -eu

if ! command -v go >/dev/null 2>&1; then
  echo "Rock needs Go 1.27 or newer: https://go.dev/dl/" >&2
  exit 1
fi

echo "Installing rock from main..." >&2
go install github.com/StephenSHorton/rock/cmd/rock@main

bin="$(go env GOPATH)/bin"
echo "Installed ${bin}/rock" >&2
case ":$PATH:" in
  *":${bin}:"*) echo "Run: rock" >&2 ;;
  *) echo "Add ${bin} to PATH, then run: rock" >&2
     echo "  export PATH=\"${bin}:\$PATH\"" >&2 ;;
esac
