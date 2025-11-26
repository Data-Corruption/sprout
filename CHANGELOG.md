# Changelog

## [v0.4.9] - 2025-11-26

- Dummy version for testing

## [v0.4.8] - 2025-11-26

- Fix potential update race

## [v0.4.7] - 2025-11-25

- Dummy version for testing

## [v0.4.6] - 2025-11-25

- Fix issue in /update where shutdown was preventing the client from receiving its response.

## [v0.4.5] - 2025-11-25

- Dummy version for testing

## [v0.4.4] - 2025-11-25

- Overhaul detached update process / web UI

## [v0.4.3] - 2025-11-24

Check for updates before starting update in /update endpoint.

## [v0.4.2] - 2025-11-24

Fix test /update endpoint.

## [v0.4.1] - 2025-11-24

Dummy version for testing.

<!-- Replace with your changelog entries. You can start at any version that isn't already tagged.

## [v0.4.0] - 2025-11-24

Minimal starter for Go CLI apps with an optional webserver daemon, changelog‑driven GitHub Actions CI/CD, and self‑updating installs.

Added
- CLI scaffold using urfave/cli v3 with common flags and subcommands.
- Service subcommand running an HTTP server (default :8080); installer provisions a systemd service.
- Shared atomic data/config directory via LMDB, safely used by both CLI and service.
- Intuitive migration system for data/config.
- Structured, rotatable logging via stdx/xlog under the per-user data path.
- Changelog-driven release automation; daily lightweight version checks and an update command with opt-out notifications.
- Cross-platform installers:
  - Linux installer with optional version pinning.
  - Windows PowerShell (WSL) installer.
- Build scripts for reproducible, versioned artifacts.
- Apache-2.0 license and template documentation.
- Build time variable injection via LDFLAGS with verification tests.
- Tests for most of the important parts of the codebase (updating, migrations, etc).

Notes
- Local builds use a placeholder version (vX.X.X) and skips update logic.
- Project structure using standard Go layout (`cmd`, `internal`, `pkg`).

-->