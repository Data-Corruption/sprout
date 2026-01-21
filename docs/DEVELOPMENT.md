# Development Guide

## Prerequisites

- **Go**: Version 1.23 or higher
- **Environment**: Linux or WSL (Windows Subsystem for Linux)
- **Architecture**: `amd64` / `x86_64` (Required due to LMDB dependency)

## Architecture

Before diving into the code, check out [ARCHITECTURE.md](ARCHITECTURE.md) to understand the high-level design, core components, and data flow.

This CI/CD pipeline is built on Forgejo Actions and Cloudflare R2. The Cloudflare R2 bucket is used to store the releases. That part can be swapped out with minimal changes to the build script which gets run by Forgejo Actions. As is, this is written for a self-hosted non-containerized runner with aggressive caching. I'll show how to set that up below

## Personal Rant - Moving from GitHub to Codeberg  

Incentives are everything... America is currently imploding due to unregulated capitalism and populist fascism. Cognitively dissonant rich assholes who got lucky at the casino are reaching grotesque immoral levels of fake monopoly wealth that will slowly poison the world until everything is dead.

Overstimulated and desensitized, It's hard to even think about what to do, let alone understand what's going on. A good start is to stop buying crap. Using an open source dev stack you can host yourself is a solid step in the right direction. Issues born from systemic incentives can probably only be fixed in kind. **Buy less**. Reward market diversity. Delayed>Instant gratification. Avoid free products/services from private companies (you're paying in ways you don't understand). Shame friends who purchase microtransactions. Baby steps <3

The self host runner, aggressive caching, etc is to minimize the load (and cost / dependency) on Codeberg / whatever host you're using and their **products**.  

## Steps

### 1. Use this Template  
Click the "Use this template" button on Codeberg to create a new repository based on Sprout. Or when making a new template, search for it in the template dropdown.

### 2. Enable Actions  
In your new repository, go to **Settings** → **Units** → **Overview** and enable Actions.

### 3. Add a PUSH_TOKEN secret to your repository  
Open your user account settings, if the repo is under an organization, open one of the orgs owner settings.  
Under **Applications** → **Generate new token**:
- Name it something like "YOUR_APP_NAME Push Token"
- Set **repository** to `Read and Write`
- Click **Generate token** and copy the token  

Now in your repository's settings under **Actions** → **Secrets**, add it as `PUSH_TOKEN`.

### 4. Setup your self-hosted runner  
You need an **x86_64** computer/server running **Linux or WSL**, and it will need to be online 24/7 or at least when you want to push a new release. Oh and you don't need to forward ports, it only does outbound requests to Codeberg / whatever Forgejo host you're using.

Get on that machine and do the following:

Ensure you have the following installed:  
- `curl` and `jq` via your package manager
- [Podman](https://podman.io/docs/installation)
- [Go](https://go.dev/doc/install) (download, then follow the install instructions)  
- [rclone](https://rclone.org/downloads/) (rclone uses sudo for its install script, not sure why)
- `systemd` (should already be installed on most distros)

I recommend using user scoped systemd. It's easier and more secure. Although you need to enable lingering for the user that you do all this on, otherwise if you restart but don't login, the runner won't start. That's an issue for a server you don't want to be logged into all the time. Easy fix.
```sh
sudo loginctl enable-linger $USER
```

> [!NOTE]
> If you're using WSL, it can be a little fucky wucky. Double check systemd user works using: `systemctl --user list-units --no-pager >/dev/null`. That should print nothing if it works. If it errors, update your WSL kernel to the latest. In powershell, run `wsl --update`. That usually fixes that kind of bs.

Download latest `forgejo-runner` (amd64)
```sh
RUNNER_VERSION="$(
  curl -fsSL https://data.forgejo.org/api/v1/repos/forgejo/runner/releases/latest \
  | jq -r '.name' | sed 's/^v//'
)"

BASE_URL="https://code.forgejo.org/forgejo/runner/releases/download"
URL="${BASE_URL}/v${RUNNER_VERSION}/forgejo-runner-${RUNNER_VERSION}-linux-amd64"

mkdir -p ~/.local/bin
curl -fL "$URL" -o ~/.local/bin/forgejo-runner
chmod +x ~/.local/bin/forgejo-runner
~/.local/bin/forgejo-runner --version
```

Create a config for the runner. If you plan on setting up multiple runners, from here on you'll need to copy the steps for each one. They'll all share the same binary but have their own homes, configs, tokens, and services. For this example, we'll do an 'org' and 'personal' runner, they'll have access to all of the repos in the account or org they're under.

Create runner homes
```sh
mkdir -p ~/.forgejo-runner/personal ~/.forgejo-runner/org
```

Create runner configs (you may need to restart your shell first)
```sh
forgejo-runner generate-config > ~/.forgejo-runner/personal/config.yml
forgejo-runner generate-config > ~/.forgejo-runner/org/config.yml
```

Edit these configs parts:

`~/.forgejo-runner/personal/config.yml`
```yaml
runner:
  capacity: 1
  file: /home/YOU/.forgejo-runner/personal/.runner
host:
  workdir_parent: /home/YOU/.cache/forgejo-runner/personal
```

`~/.forgejo-runner/org/config.yml`
```yaml
runner:
  capacity: 1
  file: /home/YOU/.forgejo-runner/org/.runner
host:
  workdir_parent: /home/YOU/.cache/forgejo-runner/org
```

Create runner token in Codeberg

I recommend creating a runner in the account or org(if under one) settings, not the repo settings. That way it will be available to your / the org's other repos.

> [!IMPORTANT]
> This shit is not containerized or secure. Do not run untrusted workflows on it OR workflows that invoke untrusted code. It's NOT a secure runner.

Org/User → **Settings → Actions → Runners** → **Create new runner** → copy token.

Register the runner (using org as an example)
```sh
forgejo-runner --config ~/.forgejo-runner/org/config.yml register
```

Use:
- Instance URL: `https://codeberg.org/`
- Token: (paste)
- Labels: `self-hosted:host,<Distro>:host`  
  `<Distro>` being smth like `fedora-43`, `ubuntu-24`, etc  
  First `self-hosted:host` one is important, will bork otherwise.

Repeat token generation and registering for each runner.

Create systemd user services for each:

`~/.config/systemd/user/forgejo-runner-personal.service`
```ini
[Unit]
Description=Forgejo Actions Runner (Personal)
After=network-online.target
Wants=network-online.target

[Service]
Environment=DOCKER_HOST=unix://%t/podman/podman.sock
ExecStart=%h/.local/bin/forgejo-runner --config %h/.forgejo-runner/personal/config.yml daemon
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
```

`~/.config/systemd/user/forgejo-runner-org.service`
```ini
[Unit]
Description=Forgejo Actions Runner (Org)
After=network-online.target
Wants=network-online.target

[Service]
Environment=DOCKER_HOST=unix://%t/podman/podman.sock
ExecStart=%h/.local/bin/forgejo-runner --config %h/.forgejo-runner/org/config.yml daemon
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
```

enable and start
```sh
systemctl --user enable --now podman.socket
systemctl --user daemon-reload
systemctl --user enable --now forgejo-runner-personal forgejo-runner-org
```

For reference, basic controls are (using org as an example):
```sh
systemctl --user status forgejo-runner-org
systemctl --user restart forgejo-runner-org
systemctl --user stop forgejo-runner-org
systemctl --user start forgejo-runner-org
journalctl --user -u forgejo-runner-org -f
```
last one is for tailing logs, pretty handy.

Now when you look in your repo, you should see it online under **Actions** → **Runners**. The build.yml workflow will be dispatched to it, since it's configured as `runs-on: self-hosted`.

### 5. Setup Cloudflare R2

This project is setup so you can swap this part out if you want. This is all handled in `scripts/build.sh` using runner secrets for upload auth. The released binaries / install script look at a release URL (also set in `scripts/build.sh`) and assume a simple flat directory structure:
```
release/
  install.ps1
  install.sh
  linux-amd64.gz.sha256
  linux-amd64.gz
  version
```

Go to Cloudflare dashboard, create an account if you don't have one. Get a domain if you don't have one.

In the dashboard, select **Account home**, then the domain you want to use. Now select **Rules → Overview → Create rule**.
- Name: `Bypass cache for YOUR-APP CD`
- Custom filter expression - When incoming requests match...
  - Field: `Hostname`
  - Operator: `equals`
  - Value: `YOUR-APP-cd.yourdomain.com`
- Then
  - Action: `Bypass cache`

Now back in the main dashboard, select **Storage & databases → R2 object storage** (sign up for free tier, will be fine for small / medium projects. You can switch to self host later easily) → **Overview → Create bucket**.
- Name: `YOUR-APP-cd`
- Region: `Auto`
- Default Storage Class: `Standard`

After creation, **Bucket Settings → Custom Domains → Add**:  
`cd.yourdomain.com`

In **R2 object storage → Overview** on the right under Account Details, click **{}Manage** API Tokens. Kinda easy to miss. **Create User API Token**:
- Token Name: `YOUR-APP CD`
- Permissions: `Object Read & Write`

After creation, copy the:
- Access Key ID
- Secret Access Key

Back in the **R2 object storage → Overview** Account Details
- Copy the Account ID
- Copy the Bucket Name e.g. `YOUR-APP-cd`

Open your repository, **Settings → Actions → Secrets** Add the following secrets:
- `R2_ACCESS_KEY_ID` = paste Access Key ID
- `R2_SECRET_ACCESS_KEY` = paste Secret Access Key
- `R2_ACCOUNT_ID` = paste Account ID
- `R2_BUCKET` = paste Bucket Name

### 6. Clone your new repository  
```sh
  git clone https://codeberg.org/YOUR_USERNAME/YOUR_REPO.git
  cd YOUR_REPO
```

### 7. Configure the Template  
All configuration is done at the top of `scripts/build.sh`:
- `APP_NAME`: Your application name (binary name).
- `RELEASE_URL`: URL to your release bucket, e.g. `https://cd.yourdomain.com/release/`.
- `CONTACT_URL`: This is used in the User-Agent. It's currently unused, but if you start making requests to other services it's a good idea to add it to the request headers. Your apps landing page or repo URL is fine.
- `DEFAULT_LOG_LEVEL`: The default log level (e.g. `debug`, `info`, `warn`, `error`).
- `SERVICE`: Set to "true" or "false" to enable/disable the daemon.
- `SERVICE_DESC`: Description for the systemd service.
- `SERVICE_ARGS`: Arguments to pass to the binary when running as a daemon. Unless you have a specific reason, leave this as `service run`.
- `SERVICE_DEFAULT_PORT`: The default port the service listens on (e.g. `8484`).

### 8. **Build the project**:  
   ```sh
   ./scripts/build.sh
   ```

### 9. **Test it**:  
   ```sh
   ./bin/linux-amd64 service run
   ```

Dev (non CI) builds set the app version to `v.X.X.X` which disabled update related features. This is useful for testing / conditionally enabling things you don't want in dev.

## Release Workflow

This project uses a changelog-driven release process:

1. Insert an entry to `CHANGELOG.md` under # Changelog, describing your changes. See [CHANGELOG.md](CHANGELOG.md) for example.
2. Push your changes to the `main` branch.
3. Forgejo Actions will automatically build the project and upload it to the release bucket. Users should see the update within a day or so.

To see how the update process works, see the [settings page](../internal/platform/http/router/settings/settings.go).  
To test it:
- publish a new release
- run `YOUR_APP update --check` to force a check, otherwise it will wait and only check ~once a day.
- visit/refresh `http://localhost:8484` in your browser.
- you should see a notification about an update. Click **restart → enable update → confirm** and the app will update, just like magic ✨

## Release Host Migration

1. Add a secondary upload target in the build script
2. Update your app to check/use this new endpoint instead of the old one
3. After most users migrate, remove the old upload target

Obvious thing to note is this is a potential attack vector, if you setup a domain and acquire a bunch of users, you'll want to keep that domain forever. Otherwise a haxor could buy it and host malicious files there. This project doesn't deal with this and you should be aware of it. Idk what the solution is, I'm at the edge of my knowledge here.
