package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"sprout/go/commands"
	"sprout/go/database"
	"sprout/go/database/config"
	"sprout/go/database/datapath"
	"sprout/go/update"
	"sprout/go/version"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/urfave/cli/v3"
)

// Template variables ---------------------------------------------------------

const Name = "sprout" // root command name

// ----------------------------------------------------------------------------

const DefaultLogLevel = "warn"

var Version string // set by build script

func main() {
	exitCode, err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	}
	os.Exit(exitCode)
}

func run() (int, error) {
	// base context with interrupt/termination handling
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// insert version for update stuff
	ctx = version.IntoContext(ctx, Version)

	// get data path
	dataPath, err := datapath.Get(Name)
	if err != nil {
		return 1, fmt.Errorf("failed to get data path: %w", err)
	}
	// create data dir if it doesn't exist
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return 1, fmt.Errorf("failed to create data path: %w", err)
	}
	ctx = datapath.IntoContext(ctx, dataPath)

	// get log path
	logPath := filepath.Join(dataPath, "logs")
	if err := os.MkdirAll(logPath, 0755); err != nil {
		return 1, fmt.Errorf("failed to create log path: %w", err)
	}

	// init logger
	log, err := xlog.New(logPath, DefaultLogLevel)
	if err != nil {
		return 1, fmt.Errorf("failed to initialize logger: %w", err)
	}
	ctx = xlog.IntoContext(ctx, log)
	defer log.Close()

	// init database
	db, err := database.New(ctx)
	if err != nil {
		return 1, fmt.Errorf("failed to initialize database: %w", err)
	}
	ctx = database.IntoContext(ctx, db)
	defer db.Close()
	xlog.Debug(ctx, "Database initialized")

	// init config
	ctx, err = config.Init(ctx)
	if err != nil {
		return 1, fmt.Errorf("failed to initialize config: %w", err)
	}
	xlog.Debug(ctx, "Config initialized")

	// set log level
	cfgLogLevel, err := config.Get[string](ctx, "logLevel")
	if err != nil {
		return 1, fmt.Errorf("failed to get log level from config: %w", err)
	}
	if err := log.SetLevel(cfgLogLevel); err != nil {
		return 1, fmt.Errorf("failed to set log level: %w", err)
	}

	// Update check
	updateNotify, err := config.Get[bool](ctx, "updateNotify")
	if err != nil {
		return 1, fmt.Errorf("failed to get updateNotify from config: %w", err)
	}
	if updateNotify {
		// get last update check time from config
		tStr, err := config.Get[string](ctx, "lastUpdateCheck")
		if err != nil {
			return 1, fmt.Errorf("failed to get lastUpdateCheck from config: %w", err)
		}
		t, err := time.Parse(time.RFC3339, tStr)
		if err != nil {
			return 1, fmt.Errorf("failed to parse lastUpdateCheck time: %w", err)
		}

		// once a day, very lightweight check
		if time.Since(t) > 24*time.Hour {
			xlog.Debug(ctx, "Checking for updates...")

			// update check time in config
			if err := config.Set(ctx, "lastUpdateCheck", time.Now().Format(time.RFC3339)); err != nil {
				return 1, fmt.Errorf("failed to set lastUpdateCheck in config: %w", err)
			}

			updateAvailable, err := update.Check(ctx)
			if err != nil {
				return 1, fmt.Errorf("failed to check for updates: %w", err)
			}
			if updateAvailable {
				fmt.Printf("Update available! Run '%s update' to update.", Name)
			}
		}
	}

	// init app
	app := &cli.Command{
		Name:    Name,
		Version: Version,
		Usage:   "example CLI application with web capabilities",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "log",
				Value: DefaultLogLevel,
				Usage: "override log level (debug|info|warn|error|none)",
			},
			&cli.BoolFlag{
				Name:    "yes",
				Aliases: []string{"y"},
				Usage:   "answer yes to all prompts",
			},
		},
		Commands: []*cli.Command{
			commands.Update,
			commands.UpdateToggleNotify,
			commands.Service,
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			// insert app name into context
			ctx = context.WithValue(ctx, commands.AppNameKey{}, Name)
			// handle log level override
			logLevel := cmd.String("log")
			if logLevel != DefaultLogLevel {
				if err := log.SetLevel(logLevel); err != nil {
					return ctx, err
				}
			}
			return ctx, nil
		},
	}

	// run app
	if err := app.Run(ctx, os.Args); err != nil {
		log.Error(err)
		return 1, fmt.Errorf("app run failed: %w", err)
	}
	return 0, nil
}
