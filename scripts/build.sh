#!/bin/bash

# Template variables ----------------------------------------------------------

APP_NAME="sprout"

# Script ----------------------------------------------------------------------

set -euo pipefail
umask 022

# quick dep check
required_bins=(go gcc sed awk sha256sum gzip) # gcc for cgo
for bin in "${required_bins[@]}"; do
  if ! command -v "$bin" >/dev/null 2>&1; then
    echo "error: '$bin' is required but not installed or not in \$PATH" >&2
    exit 1
  fi
done

version="vX.X.X" # default / development version
BIN_DIR=bin
RELEASE_BODY_FILE="$BIN_DIR/release_body.md"

# clean bin dir
rm -rf "$BIN_DIR" && mkdir -p "$BIN_DIR"
echo "🟢 Cleaned bin directory"

# if running in CI, extract latest version and description from CHANGELOG.md, if tag already exists, flag and exit.
if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
  echo "Building for CI..."
  version=$(sed -n 's/^## \[\(.*\)\] - .*/\1/p' CHANGELOG.md | head -n 1)
  description=$(awk '/^## \['"$version"'\]/ {flag=1; next} /^## \[/ {flag=0} flag {print}' CHANGELOG.md)
  echo "$description" > "$RELEASE_BODY_FILE"

  if [[ -z "$version" ]]; then
    echo "No version found in CHANGELOG.md"
    echo "DRAFT_RELEASE=false" >> "$GITHUB_ENV"
    exit 0
  fi

  # check if tag already exists
  git fetch --tags
  if git show-ref --verify --quiet "refs/tags/$version"; then
    echo "Version $version is already tagged."
    echo "DRAFT_RELEASE=false" >> "$GITHUB_ENV"
    exit 0
  fi

  echo "Version $version is not tagged yet."
  echo "DRAFT_RELEASE=true" >> "$GITHUB_ENV"
  echo "VERSION=$version" >> "$GITHUB_ENV"
fi

# place any other pre-build steps here e.g.:
# - linting
# - formatting
# - tailwindcss
# - tests
# - etc.

# build
LDFLAGS="-X 'main.version=$version'"
build_out="$BIN_DIR/linux-amd64"
GO_MAIN_PATH="./cmd/$APP_NAME"
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -trimpath -buildvcs=false -ldflags="$LDFLAGS" -o "$build_out" "$GO_MAIN_PATH"
echo "🟢 Built $build_out"

# if in CI, gzip and generate checksum
if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
  gzip_out="$build_out.gz"
  gzip -c -n -- "$build_out" > "$gzip_out"
  echo "🟢 Gzipped $build_out"

  sha_out="$gzip_out.sha256"
  (
    cd "$(dirname "$gzip_out")" || exit 1
    sha256sum "$(basename "$gzip_out")" > "$(basename "$sha_out")"
  )
  echo "🟢 Generated checksum $sha_out"
fi
