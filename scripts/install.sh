#!/bin/sh

# Target: POSIX Linux x86_64/amd64, user-level install, optional systemd --user unit
# Requires: curl, gzip, mktemp, install, sha256sum, sed, awk, (and systemd if SERVICE=true)
# Example: curl -fsSL https://raw.githubusercontent.com/Data-Corruption/sprout/main/scripts/install.sh | sh
#   (add '-s -- [VERSION]' after 'sh' for specific version/tag)

set -u
umask 077

# Template variables ----------------------------------------------------------
REPO_OWNER="Data-Corruption"
REPO_NAME="sprout"

APP_NAME="sprout"

SERVICE="true"
SERVICE_DESC="web server daemon for CLI application sprout"
SERVICE_ARGS="service run"

# Constants -------------------------------------------------------------------
APP_BIN="$HOME/.local/bin/$APP_NAME"
APP_DATA_DIR="$HOME/.$APP_NAME"
APP_ENV_FILE="$APP_DATA_DIR/$APP_NAME.env"

SERVICE_NAME="$APP_NAME.service"
SERVICE_FILE="$HOME/.config/systemd/user/$SERVICE_NAME"
SERVICE_READY_TIMEOUT_SECONDS=90

VERSION="${1:-latest}"
BIN_ASSET_NAME="linux-amd64.gz"
BIN_ASSET_NAME_SHA256="linux-amd64.gz.sha256"

# Globals used by rollback/cleanup --------------------------------------------
temp_dir=""
old_app_bin=""
old_service_file=""
service_exists=0
service_was_enabled=0
service_was_active=0

printfSuccess() { printf '\033[32m%s\033[0m\n' "$@"; }
printfErr() { printf '\033[31m%s\033[0m\n' "$@" >&2; }

fatal() { printfErr "$@"; exit 1; }

rollback() {
    rb=0
    if [ -n "$old_app_bin" ] && [ -s "$old_app_bin" ]; then
        printfErr "Restoring previous installation..."
        mv -f "$old_app_bin" "$APP_BIN" || printfErr "   Warning: Failed to restore old binary"
        rb=1
    fi
    if [ "$SERVICE" = "true" ] && [ "$service_exists" -eq 1 ]; then
        systemctl --user stop "$SERVICE_NAME" >/dev/null 2>&1 || :
        systemctl --user reset-failed "$SERVICE_NAME" >/dev/null 2>&1 || :
        if [ -n "$old_service_file" ] && [ -s "$old_service_file" ]; then
            printfErr "Restoring previous service configuration ..."
            mv -f "$old_service_file" "$SERVICE_FILE" || printfErr "   Warning: Failed to restore old service unit file"
            rb=1
        fi
        systemctl --user daemon-reload >/dev/null 2>&1 || :
        if [ "$service_was_enabled" -eq 1 ]; then
            systemctl --user enable "$SERVICE_NAME" >/dev/null 2>&1 || :
        else
            systemctl --user disable "$SERVICE_NAME" >/dev/null 2>&1 || :
        fi
        if [ "$service_was_active" -eq 1 ]; then
            systemctl --user start "$SERVICE_NAME" >/dev/null 2>&1 || :
        fi
    fi
    if [ "$rb" -eq 1 ]; then printfErr "Rolled back to previous version."; fi
}

on_exit () {
    status=$?
    if [ "$status" -ne 0 ]; then rollback; fi
    if [ -n "$temp_dir" ] && [ -d "$temp_dir" ]; then rm -rf "$temp_dir"; fi
}

trap on_exit EXIT INT TERM HUP QUIT PIPE

# Platform Checks -------------------------------------------------------------
uname_s=$(uname -s)
uname_m=$(uname -m)

# OS
[ "$uname_s" = "Linux" ] || fatal "This application is only supported on Linux. Detected OS: $uname_s"
# Architecture
( [ "$uname_m" = "x86_64" ] || [ "$uname_m" = "amd64" ] ) || fatal "This application is only supported on x86_64/amd64. Detected architecture: $uname_m"
# Disallow root
[ "$(id -u)" -ne 0 ] || fatal "Running as root is unsafe. Please run as a non-root user."
# Dependencies
for bin in curl gzip mktemp install sha256sum sed awk; do
    command -v "$bin" >/dev/null 2>&1 || fatal "Missing required tool: $bin. Please install it and re-run."
done

# Service pre-checks ----------------------------------------------------------
if [ "$SERVICE" = "true" ]; then
    # require systemd >= 246
    systemdVersion=$(systemctl --user --version 2>/dev/null \
        | awk 'NR==1 {print $2}' \
        | sed 's/^\([0-9][0-9]*\).*/\1/')
    [ -n "$systemdVersion" ] || fatal "systemd --user not available (required for SERVICE=true)"
    [ "$systemdVersion" -ge 246 ] || fatal "systemd ≥ 246 required, found $systemdVersion"

    # track prior state
    if systemctl --user cat "$SERVICE_NAME" >/dev/null 2>&1; then
        service_exists=1
        if systemctl --user is-enabled --quiet "$SERVICE_NAME"; then service_was_enabled=1; fi
        if systemctl --user is-active  --quiet "$SERVICE_NAME"; then service_was_active=1; fi
    fi
fi

# Create directories ---------------------------------------------------------
printf "📦 Installing $APP_NAME $VERSION ...\n"
mkdir -p "$(dirname "$SERVICE_FILE")" "$APP_DATA_DIR" || fatal "failed to create install dirs"

# Download -------------------------------------------------------------------
if [ "$VERSION" = "latest" ]; then
    shared_start="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/latest/download/"
else
    shared_start="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}/"
fi
bin_url="${shared_start}${BIN_ASSET_NAME}"
bin_url_sha256="${shared_start}${BIN_ASSET_NAME_SHA256}"

# make temp dir
temp_dir=$(mktemp -d) || fatal "failed to create temp dir"

# output paths
dwld_out="$temp_dir/$BIN_ASSET_NAME"
hash_out="$temp_dir/$BIN_ASSET_NAME_SHA256"
gzip_out=${dwld_out%".gz"}

printf "Downloading $bin_url ...\n"
curl_opts="--fail --location --progress-bar --show-error --connect-timeout 5 --retry-all-errors --retry 3 --retry-delay 1 --max-time 300"
curl $curl_opts -o "$dwld_out" "$bin_url" || fatal "Download of binary failed"

printf "Downloading checksum file %s ...\n" "$bin_url_sha256"
curl $curl_opts -o "$hash_out" "$bin_url_sha256" || fatal "Download of checksum file failed"

# read the first field (the hash)
expected_sum=$(cut -d' ' -f1 "$hash_out" | tr -d '\r\n')
[ ${#expected_sum} -eq 64 ] || fatal "Invalid checksum format"

printf "Verifying checksum ...\n"
actual_sum=$(sha256sum "$dwld_out" | awk '{print $1}' | tr -d '\r\n')
[ -n "$actual_sum" ] || fatal "Failed to compute actual checksum"

[ "$expected_sum" = "$actual_sum" ] || fatal "Checksum mismatch! Expected $expected_sum, got $actual_sum"

printf "Unzipping ...\n"
gzip -dc "$dwld_out" > "$gzip_out" || fatal "Failed to unzip"

# Backup (for rollback) -------------------------------------------------------
if [ -f "$APP_BIN" ] || [ "$service_exists" -eq 1 ]; then
    printf "Backing up current installation ...\n"
fi

if [ -f "$APP_BIN" ]; then
    old_app_bin="$temp_dir/$APP_NAME.old"
    cp -f "$APP_BIN" "$old_app_bin" || fatal "Failed to backup existing binary"
fi

if [ "$SERVICE" = "true" ] && [ "$service_exists" -eq 1 ]; then
    old_service_file="$temp_dir/$SERVICE_NAME.old"
    systemctl --user cat "$SERVICE_NAME" > "$old_service_file" || fatal "Failed to backup existing service unit file"
fi

# Install ---------------------------------------------------------------------
printf "Installing binary ...\n"
install -Dm755 "$gzip_out" "$APP_BIN" || fatal "Failed to install binary"

# Verify install / get version (first line only)
printf "Verifying installation (this may take a few moments if migrating) ...\n"
out=$("$APP_BIN" -v 2>/dev/null) || fatal "$APP_BIN -v exited with an error"
effective_version=$(printf '%s\n' "$out" | awk 'NR==1{print; exit}')
[ -n "$effective_version" ] || fatal "Failed to get effective version"

# Service --------------------------------------------------------------------
if [ "$SERVICE" = "true" ]; then
    [ "$service_exists" -eq 1 ] && printf "Updating service ...\n" || printf "Setting up service ...\n"

    # Escape % -> %% in args (no ${var//%/%%} in POSIX)
    safe_args=$(printf '%s' "$SERVICE_ARGS" | sed 's/%/%%/g') || fatal "Failed to escape service args"

    # Write unit file
    {
        printf '%s\n' "[Unit]"
        printf 'Description=%s\n' "$SERVICE_DESC"
        printf '%s\n' "StartLimitIntervalSec=600"
        printf '%s\n' "StartLimitBurst=5"
        printf '%s\n' "# FYI: network-online.target is kinda fucked in the user manager for some reason."
        printf '%s\n' "# Using in case it works. App will still handle unready net starts gracefully with retries."
        printf '%s\n' "Wants=network-online.target"
        printf '%s\n' "After=network-online.target"
        printf '%s\n' ""
        printf '%s\n' "[Service]"
        printf '%s\n' "Type=notify"
        printf 'ExecStart=%s %s\n' "$APP_BIN" "$safe_args"
        printf 'WorkingDirectory=%s\n' "$APP_DATA_DIR"
        printf '%s\n' "Restart=always"
        printf '%s\n' "RestartSec=3"
        printf '%s\n' "LimitNOFILE=65535"
        printf 'TimeoutStartSec=%ss\n' "$SERVICE_READY_TIMEOUT_SECONDS"
        printf '%s\n' "RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK"
        printf '%s\n' "Environment=PATH=%h/.local/bin:/usr/local/bin:/usr/bin:/bin"
        printf 'EnvironmentFile=-%s\n' "$APP_ENV_FILE"
        printf '%s\n' ""
        printf '%s\n' "[Install]"
        printf '%s\n' "WantedBy=default.target"
    } > "$SERVICE_FILE" || fatal "Failed to write service unit file"

    systemctl --user daemon-reload || fatal "Failed to reload systemd daemon"
    systemctl --user enable "$SERVICE_NAME" || fatal "Failed to enable service"
    systemctl --user reset-failed "$SERVICE_NAME" || :

    # Start/restart service (will block until service reports ready or timeout)
    if systemctl --user is-active --quiet "$SERVICE_NAME"; then
        printf "Restarting service ...\n"
        systemctl --user restart "$SERVICE_NAME" || fatal "Failed to restart service"
    else
        printf "Starting service ...\n"
        systemctl --user start "$SERVICE_NAME" || fatal "Failed to start service"
    fi
fi

# Success! --------------------------------------------------------------------
printfSuccess "Installed: $APP_NAME ($effective_version) → $APP_BIN"
printfSuccess "    Run:       '$APP_NAME -v' to verify (you may need to open a new terminal)"
if [ "$SERVICE" = "true" ]; then
  printfSuccess "    Run:       '$APP_NAME service' for service management cheat sheet"
fi