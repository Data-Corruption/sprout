# Development Guide

## Prerequisites

- **Go**: Version 1.23 or higher
- **Environment**: Linux or WSL (Windows Subsystem for Linux)
- **Architecture**: `amd64` / `x86_64` (Required due to LMDB dependency)

## Architecture

Before diving into the code, check out [ARCHITECTURE.md](ARCHITECTURE.md) to understand the high-level design, core components, and data flow.

## Quick Start

1. **Use this Template**:  
   Click the "Use this template" button on GitHub to create a new repository based on Sprout.

2. **Enable Github Actions Write Access**:  
   Go to the "Settings" tab of your repository, then **Actions** -> **General** -> **Workflow permissions** -> "Read and write permissions". Then save.

3. **Clone your new repository**:  
   ```sh
   git clone https://github.com/YOUR_USERNAME/YOUR_REPO.git
   cd YOUR_REPO
   ```

4. **Configure the Template**:  
   All three files in `./scripts` have a template section at the top.  
   Here are the variables you see (some repeated) between them:

   - **`APP_NAME` / `$AppName`**: Your application name (binary name).
   - **`REPO_OWNER` / `$Owner`**: Your GitHub username or organization.
   - **`REPO_NAME` / `$Repo`**: Your repository name.
   - **`REPO_URL`**: The full clone URL of your repository.
   - **`SERVICE` / `$Service`**: Set to "true" or "false" to enable/disable the daemon mode.
   - **`SERVICE_DESC`**: Description for the systemd service.
   - **`INSTALL_SCRIPT_URL`**: Raw URL to your `scripts/install.sh`.

5. **Rename cmd Directory**:  
   Rename `cmd/sprout` to `cmd/YOUR_APP_NAME`

6. **Build the project**:  
   ```sh
   ./scripts/build.sh
   ```

7. **Run the binary**:  
   ```sh
   ./bin/linux-amd64 -h
   ```

## Release Workflow

This project uses a changelog-driven release process:

1. Insert an entry to `CHANGELOG.md` under # Changelog, describing your changes. See [CHANGELOG.md](CHANGELOG.md) for example.
2. Push your changes to the `main` branch.
3. GitHub Actions will automatically build the project and draft a release.
4. Publish the release on GitHub to trigger the update for users.   
   They should see the update within a day or so.

After configuring the template sections in the scripts, the repo is a pre-made example project ready to be released. By default, it will be a simple HTTP server with a web UI and update functionality. All the hardest parts, done first and for you, so you can focus on the fun parts.

To see how the update process works in the app, see the [update command](../internal/app/commands/update.go).

To see how the detached update process works, see the [router](../internal/platform/http/server/router/router.go).
To test the detached update process:
- publish a new release
- run `YOUR_APP update --check` to force a check, otherwise it will wait and only check once a day.
- visit/refresh `http://localhost:8080` in your browser.
- you should see a notification with a button to update. Click it, and the app will update, just like magic ✨

## Outgrowing Sprout

Congratulations! Your 🌱 has evolved into a 🌳. Wow and it looks like an ultra rare foil variant!

### Scaling Beyond GitHub Releases

The default update mechanism is fairly lightweight: one GET request per daily check,  
plus two downloads when a user updates. This works well for small to medium user bases.

**When to migrate**: If you reach mid-upper tens of thousands of users, you'll want to  
self-host releases via a CDN to reduce GitHub's load and improve reliability.

**Migration steps**:
1. Add a secondary upload target to your release workflow (CDN, S3, etc.)
2. Update your app to check/use this new endpoint instead of GitHub
3. After most users migrate, remove the GitHub upload target

This migration is outside Sprout's scope, but the process is pretty straightforward.
