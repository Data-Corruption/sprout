//go:build linux

package update

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sprout/go/app"
	"sprout/go/platform/database/config"
	"sprout/go/platform/git"
	"sync"
	"time"

	"github.com/Data-Corruption/stdx/xlog"
	"golang.org/x/mod/semver"
)

// Template variables ---------------------------------------------------------

const (
	RepoURL          = "https://github.com/Data-Corruption/sprout.git"
	InstallScriptURL = "https://raw.githubusercontent.com/Data-Corruption/sprout/main/scripts/install.sh"
)

// ----------------------------------------------------------------------------

const UpdateTimeout = 10 * time.Minute // max time for update process

var (
	ExitFunc func() error = nil
	once     sync.Once
)

// Check checks if there is a newer version of the application available and updates the config accordingly.
// It returns true if an update is available, false otherwise.
// When running a dev build (e.g. with `vX.X.X`), it returns false without checking.
func Check(ctx context.Context) (bool, error) {
	appInfo, ok := app.FromContext(ctx)
	if !ok {
		return false, fmt.Errorf("app info not found in context")
	}

	if appInfo.Version == "" {
		return false, fmt.Errorf("failed to get appVersion from context")
	}
	if appInfo.Version == "vX.X.X" {
		return false, nil // No version set, no update check needed
	}

	lCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	latest, err := git.LatestGitHubReleaseTag(lCtx, RepoURL)
	if err != nil {
		return false, err
	}

	updateAvailable := semver.Compare(latest, appInfo.Version) > 0
	xlog.Debugf(ctx, "Latest version: %s, Current version: %s, Update available: %t", latest, appInfo.Version, updateAvailable)

	// update config
	if err := config.Set(ctx, "updateAvailable", updateAvailable); err != nil {
		return false, err
	}

	return updateAvailable, nil
}

// Update checks for available updates and prepares the update to be run on exit.
// Exit after calling this function. Calling more than once has no effect.
func Update(ctx context.Context, detached bool) error {
	var returnErr error = nil

	once.Do(func() {
		appInfo, ok := app.FromContext(ctx)
		if !ok {
			returnErr = fmt.Errorf("app info not found in context")
			return
		}
		if appInfo.Version == "vX.X.X" {
			fmt.Println("Dev build detected, skipping update.")
			return
		}

		lCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		latest, err := git.LatestGitHubReleaseTag(lCtx, RepoURL)
		if err != nil {
			returnErr = err
			return
		}

		updateAvailable := semver.Compare(latest, appInfo.Version) > 0
		if !updateAvailable {
			fmt.Println("No updates available.")
			return
		}
		fmt.Println("New version available:", latest)

		// update config. Treat updates as lazy and not super critical. Fine to set here and
		// have the update fail and user go a day without it. Just a notification after all.
		if err := config.Set(ctx, "updateAvailable", false); err != nil {
			returnErr = fmt.Errorf("failed to set updateAvailable in config: %w", err)
			return
		}

		// prepare update command
		pipeline := fmt.Sprintf("curl -sSfL %s | sh", InstallScriptURL)
		xlog.Debugf(ctx, "Prepared update, detached: %t, command: %s", detached, pipeline)

		ExitFunc = func() error {
			var cmd *exec.Cmd

			if detached {
				// run as transient systemd service, like a service but one-off and configured via cmdline args.
				// we need this to survive the parent process (service) exiting, which will kill the c group,
				// including any child processes. Even those started using `cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}`.

				launchCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()

				unitName := fmt.Sprintf("%s-update-%s", appInfo.Name, time.Now().Format("20060102-150405"))
				runtime := fmt.Sprintf("RuntimeMaxSec=%ds", int(UpdateTimeout.Seconds()))
				syslogIdent := fmt.Sprintf("SyslogIdentifier=%s-update", appInfo.Name)

				cmd = exec.CommandContext(
					launchCtx,
					"systemd-run",
					"--user",
					"--unit="+unitName,
					"--quiet",
					"-p", "StandardOutput=journal",
					"-p", "StandardError=journal",
					"-p", syslogIdent,
					"-p", runtime, // apply timeout
					"-p", "KillSignal=SIGINT",
					"-p", "TimeoutStopSec=30s", // graceful shutdown time
					"/bin/sh", "-c", pipeline,
				)
				// how to check logs (last 100 lines of updates):
				// `journalctl --user -u APPNAME-update* -n 100 -f` - add to docs or service cheat sheet?
			} else {
				runCtx, cancel := context.WithTimeout(context.Background(), UpdateTimeout)
				defer cancel()

				cmd = exec.CommandContext(runCtx, "sh", "-c", pipeline)
				cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			}

			return cmd.Run()
		}
	})

	return returnErr
}
