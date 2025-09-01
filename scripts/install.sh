#!/usr/bin/env bash

# Generic install / update script, for self-contained apps targeting linux x86_64/amd64.
# App's daemon, if present, is just a sub-command.
#
# Usage:
#   curl -sSfL https://raw.githubusercontent.com/OWNER/REPO/main/scripts/install.sh | bash -s -- [VERSION]
#
# Arguments:
#   [VERSION] Optional tag (e.g. v1.2.3). Default = latest

# Template variables ----------------------------------------------------------

REPO_OWNER="Data-Corruption"
REPO_NAME="sprout"
APP_NAME="sprout"

SERVICE="true"
SERVICE_DESC="web server daemon for CLI application sprout"
SERVICE_ARGS="service run"

# Startup ---------------------------------------------------------------------

set -euo pipefail
umask 077 # lock that shit down frfr

INSTALL_PATH="$HOME/.local/bin/$APP_NAME"
DATA_PATH="$HOME/.$APP_NAME"

SERVICE_NAME="$APP_NAME.service"
SERVICE_PATH="$HOME/.config/systemd/user/$SERVICE_NAME"
ACTIVE_TIMEOUT=10 # seconds to wait until service to become active
HEALTH_TIMEOUT=30 # seconds to wait until service writes healthy file signal

HEALTH_PATH="$DATA_PATH/.health"
ENV_PATH="$DATA_PATH/$APP_NAME.env"

VERSION="${1:-latest}"
BIN_ASSET_NAME="linux-amd64.gz"

temp_dir=""
cleanup() {
  if [[ -d "$temp_dir" ]]; then
    rm -rf "$temp_dir"
  fi
}

old_bin=""
was_enabled=0
was_active=0
unit_known=0
rollback() {
  if [[ "$SERVICE" == "true" && "$unit_known" == "1" ]]; then
    systemctl --user stop "$SERVICE_NAME" >/dev/null 2>&1 || true
    systemctl --user reset-failed "$SERVICE_NAME" >/dev/null 2>&1 || true
  fi

  if [[ -f "$old_bin" ]]; then
    echo "Restoring previous binary from $old_bin ..."
    mv -f "$old_bin" "$INSTALL_PATH" || echo "   Warning: Failed to restore old binary"
  fi

  if [[ "$SERVICE" == "true" && "$unit_known" == "1" ]]; then
    systemctl --user daemon-reload >/dev/null 2>&1 || true

    if [[ "$was_enabled" == "1" ]]; then
      systemctl --user enable "$SERVICE_NAME"  >/dev/null 2>&1 || true
    else
      systemctl --user disable "$SERVICE_NAME" >/dev/null 2>&1 || true
    fi

    if [[ "$was_active" == "1" ]]; then
      systemctl --user start "$SERVICE_NAME"   >/dev/null 2>&1 || true
    fi
  fi
}

trap '
  status=$?
  if [[ $status -ne 0 ]]; then
    rollback
  fi
  cleanup
  exit $status
' EXIT

# detect platform
uname_s=$(uname -s) # OS
uname_m=$(uname -m) # Architecture

# if not linux, exit
if [[ "$uname_s" != "Linux" ]]; then
  echo "🔴 This application is only supported on Linux. Detected OS: $uname_s" >&2
  exit 1
fi

# if not x86_64 or amd64 (some distros return this), exit
if [[ "$uname_m" != "x86_64" && "$uname_m" != "amd64" ]]; then
  echo "🔴 This application is only supported on x86_64/amd64. Detected architecture: $uname_m" >&2
  exit 1
fi

# must not be root
if [[ $EUID -eq 0 ]]; then
  echo "🔴 This script must not be run as root. Please run as a non-root user." >&2
  exit 1
fi

# dep check for bare bone distros
required_bins=(curl gzip install awk)
for bin in "${required_bins[@]}"; do
  if ! command -v "$bin" >/dev/null 2>&1; then
    echo "🔴 Missing required tool: $bin. Please install it and re-run." >&2
    exit 1
  fi
done

# create necessary directories
mkdir -p "$(dirname "$SERVICE_PATH")" "$DATA_PATH"

# if service, check systemd version and track prior state
if [[ "$SERVICE" == "true" ]]; then
  # require systemd ≥ 245
  systemdVersion=$(systemctl --user --version | head -n1 | awk '{print $2}')
  if (( systemdVersion < 245 )); then
    echo "Error: systemd ≥ 245 required, found $systemdVersion" >&2
    exit 1
  fi
  # track prior systemd state (for rollback)
  if systemctl --user cat "$SERVICE_NAME" >/dev/null 2>&1 || [[ -f "$SERVICE_PATH" ]]; then
    unit_known=1
    systemctl --user is-enabled --quiet "$SERVICE_NAME" && was_enabled=1 || true
    systemctl --user is-active  --quiet "$SERVICE_NAME" && was_active=1  || true
  fi
fi

# looks good, print info
echo "📦 Installing $APP_NAME $VERSION ..."

# Download the binary ---------------------------------------------------------

bin_url=""
if [[ "$VERSION" == "latest" ]]; then
  bin_url="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/latest/download/${BIN_ASSET_NAME}"
else
  bin_url="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}/${BIN_ASSET_NAME}"
fi

temp_dir=$(mktemp -d)
dwld_out="${temp_dir}/${BIN_ASSET_NAME}"
gzip_out="${dwld_out%.gz}"

# curl time! curl moment!
echo "Downloading binary from $bin_url"
curl --max-time 300 --retry 3 --retry-all-errors --retry-delay 1 --fail --show-error --location --progress-bar -o "$dwld_out" "$bin_url"

echo "Unzipping..."
gzip -dc "$dwld_out" > "$gzip_out" || { echo "🔴 Failed to unzip"; exit 1; }

# backup existing install in case of failure
if [[ -f "$INSTALL_PATH" ]]; then
  old_bin="$temp_dir/$APP_NAME.old"
  echo "Backing up current binary to $old_bin (will restore on failure) ..."
  mv -f "$INSTALL_PATH" "$old_bin"
fi

# install the binary
echo "Installing binary ..."
install -Dm755 "$gzip_out" "$INSTALL_PATH" || { echo "🔴 Failed to install binary."; exit 1; }

# verify install / get version
out="$("$INSTALL_PATH" -v 2>/dev/null || true)"
EFFECTIVE_VER="${out%%$'\n'*}"
[[ -n "$EFFECTIVE_VER" ]] || { echo "🔴 Failed to verify..."; exit 1; }

# Service ---------------------------------------------------------------------

if [[ "$SERVICE" == "true" ]]; then
  # escape percent in service args
  SAFE_ARGS="${SERVICE_ARGS//%/%%}"

  # write unit file, (overwrite is ok, this file is not advertized to users, they shouldn't have edited it)
  cat >"$SERVICE_PATH" <<EOF
[Unit]
Description=${SERVICE_DESC}
StartLimitIntervalSec=600
StartLimitBurst=5
# FYI: network-online.target is kinda fucked in the user manager for some reason.
# Using in cases where it works. App will probs need to handle unready net starts gracefully with retries.
Wants=network-online.target
After=network-online.target

[Service]
ExecStart=${INSTALL_PATH} ${SAFE_ARGS}
WorkingDirectory=${DATA_PATH}
Restart=always
RestartSec=3
LimitNOFILE=65535
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK

Environment=PATH=%h/.local/bin:/usr/local/bin:/usr/bin
EnvironmentFile=-${ENV_PATH}

[Install]
WantedBy=default.target
EOF

  # delete health file if exists
  rm -f "$HEALTH_PATH"

  # enable and start/restart service
  systemctl --user daemon-reload
  systemctl --user enable "$SERVICE_NAME"

  # helper for getting restart count / parsing to integer
  get_restarts() {
    local unit="$1"
    local v
    v="$(systemctl --user show -p NRestarts --value -- "$unit" 2>/dev/null || true)"
    # trim whitespace/newlines just in case
    v="${v//$'\n'/}"
    v="${v//[$' \t\r']/}"
    # coerce to 0 if empty or non-numeric
    [[ "$v" =~ ^[0-9]+$ ]] || v=0
    printf '%s' "$v"
  }

  n_res_before="$(get_restarts "$SERVICE_NAME")"

  # start/restart
  if systemctl --user is-active --quiet "$SERVICE_NAME"; then
    echo "Restarting service..."
    systemctl --user restart "$SERVICE_NAME"
  else
    echo "Starting service..."
    systemctl --user start "$SERVICE_NAME"
  fi

  # health gate

  # wait for service to become active
  deadline=$(( SECONDS + ${ACTIVE_TIMEOUT} ))
  until systemctl --user is-active --quiet "$SERVICE_NAME"; do
    if (( SECONDS >= deadline )); then
      echo "🔴 Service failed to reach active state within timeout." >&2
      exit 1
    fi
    sleep 1
  done

  # wait for health file creation or HEALTH_TIMEOUT
  deadline=$(( SECONDS + ${HEALTH_TIMEOUT} ))
  until [[ -f "$HEALTH_PATH" ]]; do
    if (( SECONDS >= deadline )); then
      echo "🔴 Service failed to create health file within timeout." >&2
      exit 1
    fi
    sleep 1
  done

  # final check (is active + no unexpected restarts)
  if ! systemctl --user is-active --quiet "$SERVICE_NAME"; then
    echo "🔴 Service failed to reach healthy active state." >&2
    exit 1
  fi
  n_res_after="$(get_restarts "$SERVICE_NAME")"
  if (( n_res_after > n_res_before )); then
    echo "🔴 Unexpected restart(s) detected." >&2
    exit 1
  fi
fi

echo ""
echo "🟢 Installed: $APP_NAME (${EFFECTIVE_VER:-$VERSION}) → $INSTALL_PATH"
echo "    Run:       '$APP_NAME -v' to verify (you may need to open a new terminal)"
if [[ "$SERVICE" == "true" ]]; then
echo "    Run:       '$APP_NAME service' for service management cheat sheet"
fi