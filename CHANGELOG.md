# Changelog

example release below. (without commenting the headers)
IMPORTANT - Remove example before you add any real releases!

## [v0.4.0] - 2025-11-23

Minimal starter for Go CLI apps with an optional webserver daemon, changelog‑driven GitHub Actions CI/CD, and self‑updating installs.

Added
- CLI scaffold using urfave/cli v3 with common flags and subcommands.
- Daemon subcommand running an HTTP server (default :8080); installer provisions a systemd service.
- Shared atomic data/config directory backed by LMDB, safely used by both CLI and daemon.
- Structured, rotatable logging via stdx/xlog under the per-user data path.
- Changelog-driven release automation; daily lightweight version checks and an update command with opt-in notifications.
- Cross-platform installers:
  - Linux installer with optional version pinning.
  - Windows PowerShell (WSL) installer.
- Build scripts for reproducible, versioned artifacts.
- Apache-2.0 license and template documentation.
- Refactored project structure to standard Go layout (`cmd`, `internal`, `pkg`).

Changed
- Configuration variables are now injected via build script (LDFLAGS) instead of requiring manual Go file edits.

Notes
- Local builds use a placeholder version (vX.X.X) and skips update logic.