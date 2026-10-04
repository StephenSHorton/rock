#!/bin/sh
# Install the latest Rock release for this OS/arch.
# Usage: curl -fsSL https://stephenshorton.github.io/rock/install.sh | sh
set -eu

REPO="StephenSHorton/rock"
MODULE="github.com/StephenSHorton/rock/cmd/rock"
API="${ROCK_UPDATE_API:-https://api.github.com}"
DL="${ROCK_UPDATE_DOWNLOAD:-https://github.com/${REPO}/releases/download}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) arch="" ;;
esac

go_install() {
  if ! command -v go >/dev/null 2>&1; then
    echo "No matching Rock release and Go is not installed. Install Go 1.27+: https://go.dev/dl/" >&2
    exit 1
  fi
  echo "Falling back to go install..." >&2
  if ! go install "${MODULE}@latest"; then
    go install "${MODULE}@main"
  fi
  bin="$(go env GOPATH)/bin"
  echo "Installed ${bin}/rock" >&2
  echo "This tree is a go install. Later: go install ${MODULE}@latest" >&2
  case ":$PATH:" in
    *":${bin}:"*) echo "Run: rock" >&2 ;;
    *) echo "Add ${bin} to PATH, then run: rock" >&2
       echo "  export PATH=\"${bin}:\$PATH\"" >&2 ;;
  esac
}

fetch() {
  url=$1
  dest=$2
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$dest"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url"
  else
    echo "Need curl or wget to download a release." >&2
    return 1
  fi
}

digest() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    echo "Need sha256sum or shasum to verify the release." >&2
    return 1
  fi
}

if [ -z "$arch" ] || { [ "$os" != linux ] && [ "$os" != darwin ]; }; then
  echo "No release asset for ${os:-unknown}/${arch:-unknown}." >&2
  go_install
  exit 0
fi

json=""
if command -v curl >/dev/null 2>&1; then
  json=$(curl -fsSL -A "rock-install" -H "Accept: application/vnd.github+json" "${API}/repos/${REPO}/releases/latest") || json=""
elif command -v wget >/dev/null 2>&1; then
  json=$(wget -qO- --header="Accept: application/vnd.github+json" "${API}/repos/${REPO}/releases/latest") || json=""
fi

tag=$(printf '%s' "$json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
ver=${tag#v}
name="rock_${ver}_${os}_${arch}.tar.gz"

if [ -z "$tag" ] || [ -z "$ver" ]; then
  echo "Could not read the latest GitHub release." >&2
  go_install
  exit 0
fi

dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT

if ! fetch "${DL}/${tag}/${name}" "${dir}/${name}"; then
  echo "No asset ${name} on ${tag}." >&2
  go_install
  exit 0
fi
if ! fetch "${DL}/${tag}/checksums.txt" "${dir}/checksums.txt"; then
  echo "Release ${tag} has no checksums.txt." >&2
  go_install
  exit 0
fi

want=$(awk -v f="$name" 'NF>=2 && $NF==f {print $1; exit}' "${dir}/checksums.txt")
if [ -z "$want" ]; then
  echo "${name} is missing from checksums.txt." >&2
  go_install
  exit 0
fi
got=$(digest "${dir}/${name}")
want_lc=$(printf '%s' "$want" | tr '[:upper:]' '[:lower:]')
got_lc=$(printf '%s' "$got" | tr '[:upper:]' '[:lower:]')
if [ "$got_lc" != "$want_lc" ]; then
  echo "checksum mismatch for ${name}" >&2
  exit 1
fi

tar -C "$dir" -xzf "${dir}/${name}"
if [ ! -f "${dir}/rock" ]; then
  echo "archive has no rock binary" >&2
  exit 1
fi
chmod 755 "${dir}/rock"

if [ -n "${ROCK_INSTALL_DIR:-}" ]; then
  dest=$ROCK_INSTALL_DIR
elif [ -w /usr/local/bin ]; then
  dest=/usr/local/bin
else
  dest="${HOME}/.local/bin"
fi
mkdir -p "$dest"
cp "${dir}/rock" "${dest}/rock"
echo "Installed ${dest}/rock (${tag})" >&2
case ":$PATH:" in
  *":${dest}:"*) echo "Run: rock   later: rock update" >&2 ;;
  *) echo "Add ${dest} to PATH, then run: rock" >&2
     echo "  export PATH=\"${dest}:\$PATH\"" >&2
     echo "Later: rock update" >&2 ;;
esac
