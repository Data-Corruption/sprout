# Development Guide

## Prerequisites

- **Go**: Version 1.23 or higher
- **Environment**: Linux or WSL (Windows Subsystem for Linux)
- **Architecture**: `amd64` / `x86_64` (Required due to LMDB dependency)

## Architecture

Before diving into the code, it's recommended to read [ARCHITECTURE.md](../ARCHITECTURE.md) to understand the high-level design, core components, and data flow.

## Quick Start

1. **Use this Template**:
   Click the "Use this template" button on GitHub to create a new repository based on Sprout.

2. **Clone your new repository**:
   ```sh
   git clone https://github.com/YOUR_USERNAME/YOUR_REPO.git
   cd YOUR_REPO
   ```

3. **Configure the Template**:
   All three files in `./scripts` have a template section at the top. Here are the variables you see (some repeated) between them:

   - **`APP_NAME` / `$AppName`**: Your application name (binary name).
   - **`REPO_OWNER` / `$Owner`**: Your GitHub username or organization.
   - **`REPO_NAME` / `$Repo`**: Your repository name.
   - **`REPO_URL`**: The full clone URL of your repository.
   - **`SERVICE` / `$Service`**: Set to "true" or "false" to enable/disable the daemon mode.
   - **`SERVICE_DESC`**: Description for the systemd service.
   - **`INSTALL_SCRIPT_URL`**: Raw URL to your `scripts/install.sh`.

4. **Build the project**:
   ```sh
   ./scripts/build.sh
   ```

5. **Run the binary**:
   ```sh
   ./bin/linux-amd64 -h
   ```

## Release Workflow

This project uses a changelog-driven release process:

1. Add an entry to `CHANGELOG.md` describing your changes.
2. Push your changes to the `main` branch.
3. GitHub Actions will automatically build the project and draft a release.
4. Publish the release on GitHub to trigger the update for users.   
   They should see the update within a day or so.
