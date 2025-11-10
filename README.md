# 🌱 Sprout [![Build / Release](https://github.com/Data-Corruption/sprout/actions/workflows/build.yml/badge.svg)](https://github.com/Data-Corruption/sprout/actions/workflows/build.yml) ![License](https://img.shields.io/github/license/Data-Corruption/sprout)

Minimal starter for Go CLI apps with an optional daemon, changelog‑driven GitHub Actions CI/CD, and self‑updating installs.

## Features

- Scaffold for CLI apps using \[[urfave/cli/v3](https://github.com/urfave/cli)].
- Daemon (webserver default) **subcommand** (systemd-managed via installer).
- CLI instances and daemon share the same atomic database/config (lmdb).
- Changelog-driven release automation (GitHub Actions).
- Self-update support with daily version checks.
- One-liner install scripts for Linux and Windows (via WSL).

## Platform Support

### Operating System

**Linux** Any `systemd` based distro (most of them) should work.  
**Windows via WSL** (same distro support as native linux).  
Systemd startup support may need to be enabled via config setting.

> Runit support with behavior parity is planned but very low priority.  

### Architecture

x86-64 only.

> ARM / RISC-V is possible but would require updating LMDB cgo bindings and type defs for non-x86_64 ABIs. Seemingly reasonable, not sure how long it would take. Like Runit support this is very low priority.

## For Developers (Using This Template)

### Setup After Cloning

1. Enable GitHub Actions to write releases.
2. Edit template variables (clearly marked near the top of):
   * `scripts/*`
   * `go/main/main.go`
   * `go/update/update.go`
   * `readme.md` (Build / Release badge url)
3. Build:
   ```sh
   ./scripts/build.sh
   ```
4. Test run:
   ```sh
   ./bin/linux-amd64 -h
   ```

Add CLI subcommands in `go/main/main.go`, extend the daemon server, etc.

### Release & Update Flow

1. Add a new entry in `CHANGELOG.md`:
   ```markdown
   ## [v0.0.2] - 2025-07-10
   Yo. whazaap, just adding a bunch of new shizle, peep it.
   ```
2. Push → GitHub Actions drafts a release (body and version come from changelog).
3. Publish → Repo is tagged, installer scripts will download attached build.
4. Clients auto-check daily and can update with a single command.

When building locally, version is set to `vX.X.X` and update logic is skipped.

### Daemon Management

- Daemon is a **subcommand** that runs an HTTP server (default port: `8080`).
- Installer script sets up a **systemd** service that runs this subcommand.
- Service/instances share the same database/logs, so commands and daemon can interop.
- Installer is idempotent: "updating" = simply rerunning it, restarting the service if needed.

This allows the tool to be both a general-purpose CLI and a running service.

## For End Users (Example Install Instructions)

These are example installation commands for the kind of app you can build with this template. When you publish your own project, adapt these to your repo. Otherwise people will install this example template app - *surprised pikachu face*.

### Linux

```sh
curl -sSfL https://raw.githubusercontent.com/Data-Corruption/sprout/main/scripts/install.sh | sh
```

With version override:
```sh
curl -sSfL https://raw.githubusercontent.com/Data-Corruption/sprout/main/scripts/install.sh | sh -s -- v0.1.2
```

### Windows (WSL)

PowerShell:

```powershell
Set-ExecutionPolicy Bypass -Scope Process -Force; iex "& { $(irm https://raw.githubusercontent.com/Data-Corruption/sprout/main/scripts/install.ps1) }"
```

With version override:
```powershell
Set-ExecutionPolicy Bypass -Scope Process -Force; iex "& { $(irm https://raw.githubusercontent.com/Data-Corruption/sprout/main/install.ps1) } -Version v0.1.2"
```

This bridges PowerShell and WSL, adds the binary to PATH, and lets you run the tool directly from PowerShell.

After install, run:

```sh
sprout -h
```

> Replacing sprout with your app name of course.

## Notes & Internals

### Why LMDB for config? Lemme tall ya

- Atomic, safe across multiple instances.
- Single lightweight dependency.
- Easy, high performance IPC for go <-> c/cpp.
- Thin wrapper for extending with DBIs (`go/database/database.go`).

## License / Contributing

[Apache 2.0](./LICENSE.md) PRs welcome.

<sub>
<3 xoxo :3 <- that last bit is a cat, his name is sebastian and he is ultra fancy. Like, i'm not kidding, more than you initially imagined while reading that. Pinky up, drinks tea... you have no idea. Crazy.
</sub>
