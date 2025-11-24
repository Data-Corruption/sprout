package main

import (
	"context"
	"fmt"
	"os"

	"sprout/internal/app"
	"sprout/internal/app/commands"

	"github.com/urfave/cli/v3"
)

// Template variables ---------------------------------------------------------

const (
	name            = "sprout" // root command name, must be filepath safe
	defaultLogLevel = "warn"
)

// ----------------------------------------------------------------------------

var version string // set by build script

func main() {
	app := &app.App{}
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
			&cli.BoolFlag{
				Name:    "migrate",
				Aliases: []string{"m"},
				Hidden:  true,
				Usage:   "skip migration guard (for the migrator)",
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			return app.Init(ctx, cmd, name, version)
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
