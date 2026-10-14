#!/usr/bin/env bash
set -euo pipefail

if ! command -v go >/dev/null 2>&1; then
  printf 'Go is required in this environment. Install Linux Go for WSL; Windows Go is separate.\n' >&2
  exit 1
fi

# Invoke with bash; executable-bit preservation is not required on NTFS.
repo_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
mkdir -p "$repo_dir/dist"
cd "$repo_dir/Pinny.Go"

GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags='-s -w -H windowsgui' -o "$repo_dir/dist/Pinny.exe" .

printf 'Built %s/dist/Pinny.exe (development build, without embedded icon)\n' "$repo_dir"
