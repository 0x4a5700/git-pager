#!/usr/bin/env bash
# Cross-compiles release binaries into dist/. Called by semantic-release with the new tag.
set -euo pipefail

tag="${1:?usage: build-release.sh <tag>}"

targets=(
  "linux/amd64"
  "linux/arm64"
  "darwin/arm64"
  "windows/amd64"
)

rm -rf dist
mkdir -p dist

for target in "${targets[@]}"; do
  os="${target%/*}"
  arch="${target#*/}"
  ext=""
  [ "$os" = "windows" ] && ext=".exe"
  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${tag}" \
    -o "dist/git-pager-${os}-${arch}${ext}" ./cmd/git-pager
done
