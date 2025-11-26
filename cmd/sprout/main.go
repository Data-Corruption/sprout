package main

import (
	"context"
	"fmt"
	"os"

	"sprout/internal/app"
	"sprout/internal/app/commands"
	"sprout/internal/platform/git"

	"github.com/urfave/cli/v3"
)

const defaultLogLevel = "warn"

// set by build script
var (
	name             string
	version          string
	repoURL          string
	installScriptURL string
	serviceEnabled   string
)

func main() {
	app := &app.App{
		Name:             name,
		Version:          version,
		RepoURL:          repoURL,
		InstallScriptURL: installScriptURL,
		ServiceEnabled:   serviceEnabled == "true",
		ReleaseSource:    &git.GitHubReleaseSource{},
	}
	defer app.Close()

	var subCommands []*cli.Command
	for _, regFunc := range commands.Registry {
		subCommands = append(subCommands, regFunc(app))
	}

	rootCommand := &cli.Command{
		Name:    name,
		Version: version,
		Usage:   "Sprout is a template for building Go services/apps.",
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
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Usage:   "temporarily override port in config",
			},
			&cli.BoolFlag{
				Name:    "migrate",
				Aliases: []string{"m"},
				Hidden:  true,
				Usage:   "skip migration guard (for the migrator)",
			},
			&cli.BoolFlag{
				Name:   "build-vars",
				Hidden: true,
				Usage:  "print build variables and exit",
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			if cmd.Bool("build-vars") {
				fmt.Printf(`{"name":"%s","version":"%s","repoURL":"%s","installScriptURL":"%s","serviceEnabled":"%s"}`+"\n",
					name, version, repoURL, installScriptURL, serviceEnabled)
				os.Exit(0)
			}
			return app.Init(ctx, cmd)
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			app.Log.Info("Ran with no arguments.")
			fmt.Printf("%s version %s\n", name, version)
			fmt.Printf("Use '%s help' to see available commands.\n", name)
			return nil
		},
		Commands: subCommands,
	}

	if err := rootCommand.Run(context.Background(), os.Args); err != nil {
		fmt.Println(err)
	}
}
