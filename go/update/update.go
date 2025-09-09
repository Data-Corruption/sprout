//go:build linux

package update

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sprout/go/database/config"
	"sprout/go/database/datapath"
	"sprout/go/system/git"
	"sprout/go/version"
	"sync"
	"syscall"
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

const DetachUpdateDelay = 30 * time.Second // delay between daemon initiated update attempts

var (
	updateMu   sync.Mutex
	lastDetach time.Time = time.Now().Add(-DetachUpdateDelay)
)

// Check checks if there is a newer version of the application available and updates the config accordingly.
// It returns true if an update is available, false otherwise.
// When running a dev build (e.g. with `vX.X.X`), it returns false without checking.
func Check(ctx context.Context) (bool, error) {
	currentVersion := version.FromContext(ctx)
	if currentVersion == "" {
		return false, fmt.Errorf("failed to get appVersion from context")
	}
	if currentVersion == "vX.X.X" {
		return false, nil // No version set, no update check needed
	}

	lCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	latest, err := git.LatestGitHubReleaseTag(lCtx, RepoURL)
	if err != nil {
		return false, err
	}

	updateAvailable := semver.Compare(latest, currentVersion) > 0
	xlog.Debugf(ctx, "Latest version: %s, Current version: %s, Update available: %t", latest, currentVersion, updateAvailable)

	// update config
	if err := config.Set(ctx, "updateAvailable", updateAvailable); err != nil {
		return false, err
	}

	return updateAvailable, nil
}

// Update checks for available updates and applies them if necessary.
// detach is for when this is called within the app daemon (install script will shut down the daemon)
func Update(ctx context.Context, detach bool) error {
	updateMu.Lock()
	defer updateMu.Unlock()

	if detach && time.Since(lastDetach) < DetachUpdateDelay {
		return fmt.Errorf("update already initiated recently, please wait a bit before trying again")
	}

	currentVersion := version.FromContext(ctx)
	if currentVersion == "" {
		return fmt.Errorf("current version not found")
	}
	if currentVersion == "vX.X.X" {
		fmt.Println("Dev build detected, skipping update.")
		return nil
	}

	lCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	latest, err := git.LatestGitHubReleaseTag(lCtx, RepoURL)
	if err != nil {
		return err
	}

	updateAvailable := semver.Compare(latest, currentVersion) > 0
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
	xlog.Debugf(ctx, "Running update command: %s", pipeline)
	if detach {
		lastDetach = time.Now()

		// get update log path
		uLogPath := filepath.Join(datapath.FromContext(ctx), "update.log")

		uLogF, err := os.OpenFile(uLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("open log: %w", err)
		}
		defer uLogF.Close()

		cmd := exec.Command("sh", "-c", pipeline)
		cmd.Stdout, cmd.Stderr = uLogF, uLogF
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start update: %w", err)
		}
	} else {
		iCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(iCtx, "sh", "-c", pipeline)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
	}
	return nil
}
