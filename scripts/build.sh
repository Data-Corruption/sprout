#!/bin/bash

# Target: bash Linux x86_64/amd64
# Requires: (go gcc sed awk sha256sum gzip rclone) preinstalled
# Builds and tests app
# If in CI it:
# - uploads install script if R2 is setup
# - if there's a new unused version in CHANGELOG.md:
#   - tags commit, pushes
#   - uploads bin/sum if R2 is setup

set -euo pipefail
umask 022

# Config --------------------------------------------------------------

APP_NAME="sprout"
RELEASE_URL="https://cd.example.com/release/"
CONTACT_URL="https://codeberg.org/DataCorruption/Sprout"

SERVICE="true"
SERVICE_DESC="Sprout daemon"
SERVICE_ARGS="service run"

# -----------------------------------------------------------------------------

TAILWIND_VERSION="${TAILWIND_VERSION:-v4.1.18}"
DAISYUI_VERSION="${DAISYUI_VERSION:-v5.5.14}"

BIN_DIR="bin"
JS_DIR="./internal/ui/assets/js"
CSS_DIR="./internal/ui/assets/css"
GO_MAIN_PATH="./cmd"

NO_CACHE='Cache-Control: no-store, max-age=0, must-revalidate' # unneeded with cache rule but just in case

# Helpers ---------------------------------------------------------------------

# run_step "success_msg" "fail_msg" command [args...]
# Runs a command, prints success or failure message, exits on failure.
run_step() {
  local success_msg="$1"
  local fail_msg="$2"
  shift 2
  local output
  if output="$("$@" 2>&1)"; then
    printf '🟢 %s\n' "$success_msg"
    [[ -n "${VERBOSE:-}" && -n "$output" ]] && printf '%s\n' "$output" || true
  else
    local status=$?
    printf '\n🔴 %s:\n' "$fail_msg"
    printf '%s\n' "$output"
    exit $status
  fi
}

# download_file "output_path" "url"
# Downloads a file, with status output.
download_file() {
  run_step "Downloaded $2" "Failed to download $2" curl -fsSL -o "$1" "$2"
}

# check_var "key" "expected"
# Verifies a build variable matches the expected value.
check_var() {
  local key="$1"
  local expected="$2"
  local actual
  actual=$(echo "$BUILD_VARS" | grep -o "\"$key\":\"[^\"]*\"" | cut -d'"' -f4)
  if [[ "$actual" != "$expected" ]]; then
    echo "🔴 Error: $key mismatch. Expected '$expected', got '$actual'"
    exit 1
  fi
}

# Stages ----------------------------------------------------------------------

dep_check() {
  local required_bins=(go gcc sed awk sha256sum gzip rclone) # gcc for cgo
  for bin in "${required_bins[@]}"; do
    if ! command -v "$bin" >/dev/null 2>&1; then
      printf "error: '$bin' is required but not installed or not in \$PATH\n" >&2
      exit 1
    fi
  done
}

setup() {
  rm -rf "$BIN_DIR" && mkdir -p "$BIN_DIR"
  printf '🟢 Cleaned bin directory\n'

  # CI detection, distribution vars check
  [[ "${CI:-}" == "true" ]] && IN_CI=true || IN_CI=false
  printf "IN_CI: %s\n" "$IN_CI"
  if $IN_CI && [[ -z "${R2_ACCESS_KEY_ID:-}" || -z "${R2_SECRET_ACCESS_KEY:-}" || -z "${R2_ACCOUNT_ID:-}" || -z "${R2_BUCKET:-}" ]]; then
    printf "🔴 Distribution not configured\n" >&2
    exit 1
  fi

  VERSION="vX.X.X" # dev/test version
  DESCRIPTION="hello world"

  if $IN_CI; then
    # R2 setup
    export RCLONE_CONFIG_R2_TYPE=s3
    export RCLONE_CONFIG_R2_PROVIDER=Cloudflare
    export RCLONE_CONFIG_R2_ACCESS_KEY_ID="$R2_ACCESS_KEY_ID"
    export RCLONE_CONFIG_R2_SECRET_ACCESS_KEY="$R2_SECRET_ACCESS_KEY"
    export RCLONE_CONFIG_R2_ENDPOINT="https://${R2_ACCOUNT_ID}.r2.cloudflarestorage.com"
      
    # Process install.sh template
    sed -e "s|<APP_NAME>|$APP_NAME|g" \
        -e "s|<RELEASE_URL>|$RELEASE_URL|g" \
        -e "s|<SERVICE>|$SERVICE|g" \
        -e "s|<SERVICE_DESC>|$SERVICE_DESC|g" \
        -e "s|<SERVICE_ARGS>|$SERVICE_ARGS|g" \
        "./scripts/install.sh" > "$BIN_DIR/install.sh"

    # Upload install.sh
    run_step "Uploaded install.sh" "Failed to upload install.sh" rclone copyto "$BIN_DIR/install.sh" "r2:$R2_BUCKET/release/install.sh" --header-upload "$NO_CACHE" --s3-env-auth --s3-no-check-bucket

    # Process install.ps1 template
    sed -e "s|<APP_NAME>|$APP_NAME|g" \
        -e "s|<RELEASE_URL>|$RELEASE_URL|g" \
        -e "s|<SERVICE>|\$$SERVICE|g" \
        "./scripts/install.ps1" > "$BIN_DIR/install.ps1"

    # Upload install.ps1
    run_step "Uploaded install.ps1" "Failed to upload install.ps1" rclone copyto "$BIN_DIR/install.ps1" "r2:$R2_BUCKET/release/install.ps1" --header-upload "$NO_CACHE" --s3-env-auth --s3-no-check-bucket

    VERSION=$(sed -n 's/^## \[\(.*\)\] - .*/\1/p' CHANGELOG.md | head -n 1)
    if [[ -z "$VERSION" ]]; then
      printf "No version found in CHANGELOG.md\n"
      exit 0
    fi
    if git show-ref --verify --quiet "refs/tags/$VERSION"; then
      printf "Version $VERSION is already tagged.\n"
      exit 0
    fi

    DESCRIPTION=$(awk '/^## \['"$VERSION"'\]/ {flag=1; next} /^## \[/ {flag=0} flag {print}' CHANGELOG.md)
    printf "Version $VERSION is unused, building...\n"
  fi

  # Export for use in other stages
  export IN_CI VERSION DESCRIPTION
}

frontend_build() {
  # Download tools if missing (or refetch if requested in CI)
  $IN_CI && [[ "${REFETCH_TOOLS:-false}" == "true" ]] && rm -f esbuild tailwindcss "$CSS_DIR/daisyui.mjs" "$CSS_DIR/daisyui-theme.mjs"
  [[ -f esbuild ]] || curl -fsSL https://esbuild.github.io/dl/latest | sh
  [[ -f tailwindcss ]] || download_file tailwindcss "https://github.com/tailwindlabs/tailwindcss/releases/download/${TAILWIND_VERSION}/tailwindcss-linux-x64"
  [[ -f "$CSS_DIR/daisyui.mjs" ]] || download_file "$CSS_DIR/daisyui.mjs" "https://github.com/saadeghi/daisyui/releases/download/${DAISYUI_VERSION}/daisyui.mjs"
  [[ -f "$CSS_DIR/daisyui-theme.mjs" ]] || download_file "$CSS_DIR/daisyui-theme.mjs" "https://github.com/saadeghi/daisyui/releases/download/${DAISYUI_VERSION}/daisyui-theme.mjs"

  chmod +x tailwindcss esbuild
  run_step "Tailwind CSS built" "Tailwind CSS failed" ./tailwindcss -i "$CSS_DIR/input.css" -o "$CSS_DIR/output.css" --minify
  run_step "JavaScript bundled" "JavaScript bundling failed" ./esbuild "$JS_DIR/src/main.js" --bundle --minify --outfile="$JS_DIR/output.js"
}

tests() {
  run_step "Tests passed" "Tests failed" go test -race ./...
}

go_build() {
  local ldflags="-X 'main.version=$VERSION' -X 'main.name=$APP_NAME' -X 'main.releaseURL=$RELEASE_URL' -X 'main.contactURL=$CONTACT_URL' -X 'main.serviceEnabled=$SERVICE'"
  BUILD_OUT="$BIN_DIR/linux-amd64"
  
  GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -trimpath -buildvcs=false -ldflags="$ldflags" -o "$BUILD_OUT" "$GO_MAIN_PATH"
  printf "🟢 Built $BUILD_OUT\n"

  # Export for use in other stages
  export BUILD_OUT
}

verify_build() {
  BUILD_VARS=$("$BUILD_OUT" --build-vars)
  export BUILD_VARS

  check_var "name" "$APP_NAME"
  check_var "version" "$VERSION"
  check_var "releaseURL" "$RELEASE_URL"
  check_var "contactURL" "$CONTACT_URL"
  check_var "serviceEnabled" "$SERVICE"

  printf "🟢 Build variables verified\n"
}

distribute() {
  # Gzip binary
  local gzip_out="$BUILD_OUT.gz"
  gzip -c -n -- "$BUILD_OUT" > "$gzip_out"
  printf "🟢 Gzipped $BUILD_OUT\n"

  # Generate checksum
  local sha_out="$gzip_out.sha256"
  (
    cd "$(dirname "$gzip_out")" || exit 1
    sha256sum "$(basename "$gzip_out")" > "$(basename "$sha_out")"
  )
  printf "🟢 Generated checksum $sha_out\n"

  # Tag and push (GIT_TERMINAL_PROMPT=0 ensures failure instead of hang if auth fails)
  run_step "Tagged $VERSION" "Failed to tag $VERSION" git tag "$VERSION"
  run_step "Pushed $VERSION" "Failed to push $VERSION" env GIT_TERMINAL_PROMPT=0 git push origin "$VERSION"

  # Upload to R2 (no-check-bucket needed for Object Read & Write tokens)
  run_step "Uploaded $(basename "$gzip_out")" "Failed to upload $(basename "$gzip_out")" rclone copyto "$gzip_out" "r2:$R2_BUCKET/release/$(basename "$gzip_out")" --header-upload "$NO_CACHE" --s3-env-auth --s3-no-check-bucket
  run_step "Uploaded $(basename "$sha_out")" "Failed to upload $(basename "$sha_out")" rclone copyto "$sha_out" "r2:$R2_BUCKET/release/$(basename "$sha_out")" --header-upload "$NO_CACHE" --s3-env-auth --s3-no-check-bucket

  # Upload version file
  local version_file="$BIN_DIR/version"
  echo "$VERSION" > "$version_file"
  run_step "Uploaded version" "Failed to upload version" rclone copyto "$version_file" "r2:$R2_BUCKET/release/version" --header-upload "$NO_CACHE" --s3-env-auth --s3-no-check-bucket
}

# Main ------------------------------------------------------------------------

main() {
  dep_check
  setup

  # pre-build here (e.g., code generation, linting)

  frontend_build
  tests

  # build here (e.g., additional binaries, platforms)

  go_build
  verify_build

  # post-build here (e.g., packaging, signing)

  if $IN_CI; then
    distribute
  fi
}

main "$@"