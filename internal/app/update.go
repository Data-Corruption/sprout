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
	"sync"
	"syscall"
	"time"

	"github.com/Data-Corruption/stdx/xhttp"
	"golang.org/x/mod/semver"
)

const UpdateTimeout = 10 * time.Minute // max time for update process

var uOnce sync.Once

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

// DeferUpdate prepares the install/update script to be run on exit.
// It will prep the update regardless of if an update is available or not.
// You should exit soon after calling this.
// Calling either DeferUpdate or DetachUpdate more than once does nothing.
// Only the first call will have any effect.
func (a *App) DeferUpdate() error {
	var rErr error
	uOnce.Do(func() {
		if err := a.uPrep(); err != nil {
			rErr = err
			return
		}

		// prepare update command
		pipeline := fmt.Sprintf("curl -sSfL %s | sh", a.InstallScriptURL)
		a.Log.Debugf("Prepared update, command: %s", pipeline)

		a.SetPostCleanup(func() error {
			rCtx, rCancel := context.WithTimeout(context.Background(), UpdateTimeout)
			defer rCancel()

			cmd := exec.CommandContext(rCtx, "sh", "-c", pipeline)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			return cmd.Run()
		})
	})
	return rErr
}

// DetachUpdate starts the install/update script as a detached process.
// It does so regardless of if an update is available or not.
// After calling this, the process will soon be closed externally by the install/update script.
// Calling either DeferUpdate or DetachUpdate more than once does nothing.
// Only the first call will have any effect.
func (a *App) DetachUpdate() error {
	var rErr error
	uOnce.Do(func() {
		if err := a.uPrep(); err != nil {
			rErr = err
			return
		}

		// prepare update command
		name := a.Name
		pipeline := fmt.Sprintf("curl -sSfL %s | sh", a.InstallScriptURL)
		logPath := filepath.Join(a.Paths.Storage, "update.log")
		a.Log.Debugf("Prepared detached update: command: %s, logPath: %s", pipeline, logPath)

		// run update (install/update script will close this process)
		if err := runUpdateDetached(a.ServiceEnabled, name, pipeline, logPath); err != nil {
			rErr = err
			return
		}
	})
	return rErr
}

func (a *App) uPrep() error {
	// double check version string
	if a.Version == "" {
		return fmt.Errorf("failed to get appVersion")
	}
	if a.Version == "vX.X.X" {
		return ErrDevBuild
	}
	// set updateAvailable to false since we're updating, and followup to the current version
	if err := database.UpdateConfig(a.DB, func(cfg *database.Configuration) error {
		cfg.UpdateAvailable = false
		cfg.UpdateFollowup = a.Version
		return nil
	}); err != nil {
		return fmt.Errorf("failed to update updateAvailable in config: %w", err)
	}
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
			"--no-block", // fully detached
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
