//go:build linux

package update

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sprout/go/app"
	"sprout/go/database/config"
	"sprout/go/system/git"
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

// Update checks for available updates and applies them if necessary.
// You should exit after calling this function
func Update(ctx context.Context, logToFile bool) error {
	appInfo, ok := app.FromContext(ctx)
	if !ok {
		return fmt.Errorf("app info not found in context")
	}
	if appInfo.Version == "vX.X.X" {
		fmt.Println("Dev build detected, skipping update.")
		return nil
	}

	lCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	latest, err := git.LatestGitHubReleaseTag(lCtx, RepoURL)
	if err != nil {
		return err
	}

	updateAvailable := semver.Compare(latest, appInfo.Version) > 0
	if !updateAvailable {
		fmt.Println("No updates available.")
		return nil
	}
	fmt.Println("New version available:", latest)

	// update config
	if err := config.Set(ctx, "updateAvailable", false); err != nil {
		return fmt.Errorf("failed to set updateAvailable in config: %w", err)
	}

	// run the install command
	pipeline := fmt.Sprintf("curl -sSfL %s | sh", InstallScriptURL)
	xlog.Debugf(ctx, "Running update, log to file: %t, command: %s", logToFile, pipeline)
	var doErr error
	once.Do(func() {
		ExitFunc = func() error {
			iCtx, cancel := context.WithTimeout(ctx, UpdateTimeout)
			defer cancel()

			cmd := exec.CommandContext(iCtx, "sh", "-c", pipeline)

			if logToFile {
				uLogPath := filepath.Join(appInfo.Storage, "update.log")
				uLogF, err := os.OpenFile(uLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
				if err != nil {
					doErr = fmt.Errorf("open log: %w", err)
				}
				defer uLogF.Close()
				cmd.Stdout, cmd.Stderr = uLogF, uLogF
			} else {
				cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			}

			return cmd.Run()
		}
	})
	return doErr
}
