//go:build linux

package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sprout/internal/platform/database"
	"syscall"
	"time"

	"github.com/Data-Corruption/stdx/xhttp"
	"golang.org/x/mod/semver"
	"golang.org/x/time/rate"
)

const UpdateTimeout = 10 * time.Minute // max time for update process

var uLimiter = rate.NewLimiter(rate.Every(3*time.Second), 1) // extra little padding for checks/updating

var ErrDevBuild = &xhttp.Err{
	Code: http.StatusNotImplemented,
	Msg:  "development build detected, skipping...",
	Err:  fmt.Errorf("development build detected, skipping"),
}

// Notify runs on app start to notify user of available updates if enabled in config.
// It checks for updates once a day.
func (a *App) Notify() error {
	// check if update notifications are enabled
	cfg, err := database.ViewConfig(a.DB)
	if err != nil {
		return fmt.Errorf("failed to view config: %w", err)
	}

	if cfg.UpdateNotifications {
		// once a day, very lightweight check, trying to be polite to github
		if time.Since(cfg.LastUpdateCheck) > 24*time.Hour {
			a.Log.Debug("Checking for updates...")
			// update check time in config
			if err := database.UpdateConfig(a.DB, func(cfg *database.Configuration) error {
				cfg.LastUpdateCheck = time.Now()
				return nil
			}); err != nil {
				return fmt.Errorf("failed to update lastUpdateCheck in config: %w", err)
			}
			updateAvailable, err := a.UpdateCheck()
			if err != nil && !errors.Is(err, ErrDevBuild) {
				a.Log.Errorf("Update check failed: %v", err) // just log since might not be online
			}
			if updateAvailable {
				fmt.Println("Update available! Run 'sprout update' to update to the latest version.")
			}
		}
	}
	return nil
}

// Check checks if there is a newer version of the application available and updates the config accordingly.
// It returns true if an update is available, false otherwise.
// When running a dev build (e.g. with `vX.X.X`), it returns false without checking.
func (a *App) UpdateCheck() (bool, error) {
	// rate limit
	rCtx, rCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer rCancel()
	if err := uLimiter.Wait(rCtx); err != nil {
		// could be err, timeout, or burst exceeded
		return false, fmt.Errorf("update check rate limited: %w", err)
	}

	if a.Version == "" {
		return false, fmt.Errorf("failed to get appVersion from context")
	}
	if a.Version == "vX.X.X" {
		return false, ErrDevBuild
	}

	lCtx, lCancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer lCancel()

	latest, err := a.ReleaseSource.GetLatest(lCtx, a.RepoURL)
	if err != nil {
		return false, err
	}

	updateAvailable := semver.Compare(latest, a.Version) > 0
	a.Log.Debugf("Latest version: %s, Current version: %s, Update available: %t", latest, a.Version, updateAvailable)

	// update config
	if err := database.UpdateConfig(a.DB, func(cfg *database.Configuration) error {
		cfg.UpdateAvailable = updateAvailable
		return nil
	}); err != nil {
		return false, fmt.Errorf("failed to update updateAvailable in config: %w", err)
	}

	return updateAvailable, nil
}

// Update prepares the update to be run on exit. Exit soon after calling this
// function. Calling more than once just keeps setting UpdateAvailable to false.
// This will prep the update regardless of if an update is available or not.
// Aside from doing unnecessary work, updating when already on the latest version
// is basically idempotent.
func (a *App) Update(detached bool) error {
	// rate limit
	rCtx, rCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer rCancel()
	if err := uLimiter.Wait(rCtx); err != nil {
		// could be err, timeout, or burst exceeded
		return fmt.Errorf("update rate limited: %w", err)
	}

	// double check version string
	if a.Version == "" {
		return fmt.Errorf("failed to get appVersion")
	}
	if a.Version == "vX.X.X" {
		return ErrDevBuild
	}

	// set updateAvailable to false since we're updating
	if err := database.UpdateConfig(a.DB, func(cfg *database.Configuration) error {
		cfg.UpdateAvailable = false
		cfg.UpdateFollowup = a.Version
		return nil
	}); err != nil {
		return fmt.Errorf("failed to update updateAvailable in config: %w", err)
	}

	// prepare update command
	name := a.Name
	pipeline := fmt.Sprintf("curl -sSfL %s | sh", a.InstallScriptURL)
	logPath := filepath.Join(a.Paths.Storage, "update.log")
	a.Log.Debugf("Prepared update, detached: %t, command: %s, logPath: %s", detached, pipeline, logPath)

	a.SetPostCleanup(func() error {
		if detached {
			return runUpdateDetached(a.ServiceEnabled, name, pipeline, logPath)
		} else {
			rCtx, rCancel := context.WithTimeout(context.Background(), UpdateTimeout)
			defer rCancel()

			cmd := exec.CommandContext(rCtx, "sh", "-c", pipeline)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			return cmd.Run()
		}
	})

	return nil
}

func runUpdateDetached(serviceEnabled bool, name, pipeline, logPath string) error {
	var cmd *exec.Cmd

	if serviceEnabled {
		// Run as transient systemd service (like a service but one-off and
		// configured via cmdline args). Assuming this is run from in the daemon,
		// we need this to survive the parent process (service) exiting, which
		// will kill the c group, including any child processes. Even those started
		// using `cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}`. The service
		// needs to exit because the install script updates the unit file, etc.

		lCtx, lCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer lCancel()

		unitName := fmt.Sprintf("%s-update-%s", name, time.Now().Format("20060102-150405"))
		runtime := fmt.Sprintf("RuntimeMaxSec=%ds", int(UpdateTimeout.Seconds()))
		syslogIdent := fmt.Sprintf("SyslogIdentifier=%s-update", name)

		cmd = exec.CommandContext(
			lCtx,
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
	} else {
		// Not under threat of c group being killed, so just use setsid
		// with shell-managed logging. escape logPath to be safe.
		pipelineWithLogging := fmt.Sprintf("( %s ) >> %q 2>&1", pipeline, logPath)
		cmd := exec.Command("sh", "-c", pipelineWithLogging)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start detached update: %w", err)
		}
		// release resources so the parent doesn't track the child (prevents zombies)
		if err := cmd.Process.Release(); err != nil {
			return fmt.Errorf("failed to release process: %w", err)
		}
		return nil
	}

	return cmd.Run()
}
