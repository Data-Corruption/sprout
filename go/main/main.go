package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"sprout/go/app"
	"sprout/go/app/commands"
	"sprout/go/platform/database"
	"sprout/go/platform/database/config"
	"sprout/go/platform/update"
	"sprout/go/platform/x"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/urfave/cli/v3"
)

// Template variables ---------------------------------------------------------

const (
	name            = "sprout" // root command name, must be filepath safe
	defaultLogLevel = "warn"
)

// ----------------------------------------------------------------------------

var (
	version      string // set by build script
	cleanUpFuncs []func() error
	mainContext  context.Context
)

func main() {
	defer cleanup()

	app := &cli.Command{
		Name:    name,
		Version: version,
		Usage:   "example application",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log",
				Aliases: []string{"l"},
				Value:   defaultLogLevel,
				Usage:   "override log level (debug|info|warn|error|none)",
			},
			&cli.BoolFlag{
				Name:    "version",
				Aliases: []string{"v"},
				Usage:   "print version and exit",
			},
			&cli.BoolFlag{
				Name:    "migrate",
				Aliases: []string{"m"},
				Hidden:  true,
				Usage:   "skip migration guard (for the migrator)",
			},
		},
		Before: startup,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			xlog.Info(ctx, "Ran with no command")
			fmt.Printf("%s version %s\n", name, version)
			fmt.Printf("Use '%s help' to see available commands.\n", name)
			return nil
		},
		Commands: []*cli.Command{
			commands.Update,
			commands.Service,
		},
	}

	mainContext = context.Background()
	if err := app.Run(mainContext, os.Args); err != nil {
		fmt.Println(err)
	}
}

func startup(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	if cmd.Bool("version") {
		fmt.Printf("%s %s\n", name, version)
		os.Exit(0)
	}

	appInfo, err := app.NewAppInfo(name, version)
	if err != nil {
		return ctx, fmt.Errorf("failed to create app info: %w", err)
	}
	ctx = app.IntoContext(ctx, appInfo)

	// setup migration guard.
	if !cmd.Bool("migrate") {
		cleanup, err := update.Mguard(appInfo.Runtime)
		if err != nil {
			return ctx, fmt.Errorf("failed to setup migration guard: %w", err)
		}
		cleanUpFuncs = append(cleanUpFuncs, cleanup)
	} else {
		fmt.Printf("%s version %s\n", name, version)
	}

	// init Logger
	initLogLevel := x.Ternary(cmd.String("log") == "debug", "debug", "none")
	log, err := xlog.New(filepath.Join(appInfo.Storage, "logs"), initLogLevel)
	if err != nil {
		return ctx, fmt.Errorf("failed to initialize logger: %w", err)
	}
	ctx = xlog.IntoContext(ctx, log)
	cleanUpFuncs = append(cleanUpFuncs, log.Close)

	xlog.Debugf(ctx, "Starting %s, version: %s, storage path: %s, runtime path: %s",
		appInfo.Name, appInfo.Version, appInfo.Storage, appInfo.Runtime)

	// init Database
	db, err := database.New(ctx)
	if err != nil {
		return ctx, fmt.Errorf("failed to initialize database: %w", err)
	}
	ctx = database.IntoContext(ctx, db)
	dbClose := func() error { db.Close(); return nil }
	cleanUpFuncs = append(cleanUpFuncs, dbClose)
	xlog.Debug(ctx, "Database initialized")

	// init Config
	ctx, err = config.Init(ctx)
	if err != nil {
		return ctx, fmt.Errorf("failed to initialize config: %w", err)
	}
	xlog.Debug(ctx, "Config initialized")

	// calculate BaseURL
	baseURL, err := getBaseURL(ctx)
	if err != nil {
		return ctx, fmt.Errorf("failed to get base URL: %w", err)
	}
	appInfo.BaseURL = baseURL
	ctx = app.IntoContext(ctx, appInfo) // overwrite with updated appInfo
	xlog.Debugf(ctx, "Base URL: %s", appInfo.BaseURL)

	// set log level
	if initLogLevel != "debug" {
		cfgLogLevel, err := config.Get[string](ctx, "logLevel")
		if err != nil {
			return ctx, fmt.Errorf("failed to get log level from config: %w", err)
		}
		if err := log.SetLevel(cfgLogLevel); err != nil {
			return ctx, fmt.Errorf("failed to set log level: %w", err)
		}
	}

	// update check
	updateNotify, err := config.Get[bool](ctx, "updateNotify")
	if err != nil {
		return ctx, fmt.Errorf("failed to get updateNotify from config: %w", err)
	}
	if updateNotify {
		// get last update check time from config
		tStr, err := config.Get[string](ctx, "lastUpdateCheck")
		if err != nil {
			return ctx, fmt.Errorf("failed to get lastUpdateCheck from config: %w", err)
		}
		t, err := time.Parse(time.RFC3339, tStr)
		if err != nil {
			return ctx, fmt.Errorf("failed to parse lastUpdateCheck time: %w", err)
		}
		// once a day, very lightweight check, to be polite to github
		if time.Since(t) > 24*time.Hour {
			xlog.Debug(ctx, "Checking for updates...")
			// update check time in config
			if err := config.Set(ctx, "lastUpdateCheck", time.Now().Format(time.RFC3339)); err != nil {
				return ctx, fmt.Errorf("failed to set lastUpdateCheck in config: %w", err)
			}
			updateAvailable, err := update.Check(ctx)
			if err != nil {
				xlog.Errorf(ctx, "Update check failed: %v", err) // just log since might not be online
			}
			if updateAvailable {
				fmt.Println("Update available! Run 'sprout update' to update to the latest version.")
			}
		}
	}

	// init other components

	return ctx, nil
}

func cleanup() {
	// call clean up funcs in reverse order
	for i := len(cleanUpFuncs) - 1; i >= 0; i-- {
		if err := cleanUpFuncs[i](); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to clean up: %v\n", err)
		}
	}
	// call update exit func if set
	if update.ExitFunc != nil {
		// sleep a bit to allow cleanup to sync to disk
		time.Sleep(1 * time.Second)
		if err := update.ExitFunc(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to update: %v\n", err)
		}
	}
}

func getBaseURL(ctx context.Context) (string, error) {
	port, err := config.Get[int](ctx, "port")
	if err != nil {
		return "", fmt.Errorf("failed to get port from config: %w", err)
	}
	host, err := config.Get[string](ctx, "host")
	if err != nil {
		return "", fmt.Errorf("failed to get host from config: %w", err)
	}
	proxyPort, err := config.Get[int](ctx, "proxyPort")
	if err != nil {
		return "", fmt.Errorf("failed to get proxyPort from config: %w", err)
	}

	host = x.Ternary(host != "", host, "localhost")
	port = x.Ternary(proxyPort != 0, proxyPort, port)
	hidePort := port == 80 || port == 443
	scheme := x.Ternary(port == 443, "https", "http")
	baseURL := fmt.Sprintf("%s://%s%s", scheme, host, x.Ternary(hidePort, "", fmt.Sprintf(":%d", port)))
	return baseURL, nil
}
