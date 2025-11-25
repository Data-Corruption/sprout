# 🌱 Sprout

[![Build Status](https://github.com/Data-Corruption/sprout/actions/workflows/build.yml/badge.svg)](https://github.com/Data-Corruption/sprout/actions/workflows/build.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](./LICENSE.md)
[![Go Report Card](https://goreportcard.com/badge/github.com/Data-Corruption/sprout)](https://goreportcard.com/report/github.com/Data-Corruption/sprout)

**The minimal, self-updating Go CLI starter kit.**

Sprout provides a unified architecture for building production-ready command-line tools and system services. It eliminates the boilerplate of setting up robust CLI applications, offering a solid foundation that scales from simple scripts to complex daemons.

## Features

- **Modern CLI Interface**: Built on `urfave/cli/v3` for a standard, POSIX-compliant user experience.
- **Self-Updating**: Integrated daily version checks and single-command updates.
- **Daemon Mode**: Optional systemd-managed background service capability.
- **Atomic State**: Shared LMDB database for reliable configuration and state management across Daemon and CLI processes.
- **CI/CD Ready**: Automated, changelog-driven release pipeline via GitHub Actions.
- **Cross-Platform**: Easy installation and support for Linux and Windows (WSL).

## Architecture

Sprout is designed around the principle of unified dependency injection, ensuring that your application state is consistent and easily testable. For a deep dive into the system design, see [ARCHITECTURE.md](./ARCHITECTURE.md).

### Why LMDB for config? Lemme tall ya

- **Atomic & Safe**: Writes are fully ACID compliant, ensuring data integrity even across multiple concurrent processes.
- **Lightweight**: A single, small dependency with no external server required.
- **High Performance**: Extremely fast reads and solid writes, serving as an efficient cross-language IPC mechanism if needed.
- **Extensible**: A thin wrapper (`internal/platform/database/database.go`) allows for easy extension with new Database Interfaces (DBIs).

## Get Started

- **[Use this Template](docs/DEVELOPMENT.md)**: How to fork, configure, and build your own application using Sprout.
- **[Installation Guide](docs/INSTALLATION.md)**: A template for your end-user installation instructions.

## License

Apache 2.0 - See [LICENSE.md](./LICENSE.md) for details.

<br>

<sub>
<3 xoxo :3 <- that last bit is a cat, his name is sebastian and he is ultra fancy. Like, i'm not kidding, more than you initially imagined while reading that. Pinky up, drinks tea... you have no idea. Crazy.
</sub>
