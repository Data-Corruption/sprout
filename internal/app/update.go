//go:build linux

package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sprout/internal/platform/database"
	"sync"
	"syscall"
	"time"

	"golang.org/x/mod/semver"
)

const UpdateTimeout = 10 * time.Minute // max time for update process

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
			if err != nil {
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

	if a.Version == "" {
		return false, fmt.Errorf("failed to get appVersion from context")
	}
	if a.Version == "vX.X.X" {
		return false, nil // No version set, no update check needed
	}

	lCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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

var once = new(sync.Once)

// Update checks for available updates and prepares the update to be run on exit.
// Exit soon after calling this function. Calling more than once has no effect.
func (a *App) Update(detached bool) error {
	var returnErr error = nil

	once.Do(func() {
		if a.Version == "" {
			returnErr = fmt.Errorf("failed to get appVersion")
			return
		}
		if a.Version == "vX.X.X" {
			fmt.Println("Dev build detected, skipping update.")
			return
		}

		lCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		latest, err := a.ReleaseSource.GetLatest(lCtx, a.RepoURL)
		if err != nil {
			returnErr = err
			return
		}

		updateAvailable := semver.Compare(latest, a.Version) > 0
		if !updateAvailable {
			fmt.Println("No updates available.")
			return
		}
		fmt.Println("New version available:", latest)

		// update config
		if err := database.UpdateConfig(a.DB, func(cfg *database.Configuration) error {
			cfg.UpdateAvailable = false
			return nil
		}); err != nil {
			returnErr = fmt.Errorf("failed to update updateAvailable in config: %w", err)
			return
		}

		// prepare update command
		name := a.Name
		pipeline := fmt.Sprintf("curl -sSfL %s | sh", a.InstallScriptURL)
		logPath := filepath.Join(a.Paths.Storage, "update.log")
		a.Log.Debugf("Prepared update, detached: %t, command: %s, logPath: %s", detached, pipeline, logPath)

		a.SetPostCleanup(func() error {
			return runUpdate(detached, a.ServiceEnabled, name, pipeline, logPath)
		})
	})

	return returnErr
}

func runUpdate(detached, serviceEnabled bool, name, pipeline, logPath string) error {
	var cmd *exec.Cmd

	if detached {
		if serviceEnabled {
			// Run as transient systemd service (like a service but one-off and
			// configured via cmdline args). Assuming this is run from in the daemon,
			// we need this to survive the parent process (service) exiting, which
			// will kill the c group, including any child processes. Even those started
			// using `cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}`. The service
			// needs to exit because the install script updates the unit file, etc.

			launchCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			unitName := fmt.Sprintf("%s-update-%s", name, time.Now().Format("20060102-150405"))
			runtime := fmt.Sprintf("RuntimeMaxSec=%ds", int(UpdateTimeout.Seconds()))
			syslogIdent := fmt.Sprintf("SyslogIdentifier=%s-update", name)

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
	} else {
		runCtx, cancel := context.WithTimeout(context.Background(), UpdateTimeout)
		defer cancel()

		cmd = exec.CommandContext(runCtx, "sh", "-c", pipeline)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	}

	return cmd.Run()
}
