//go:build linux

package commands

import (
	"context"
	"fmt"
	"sprout/go/platform/database/config"
	"sprout/go/platform/update"

	"github.com/urfave/cli/v3"
)

var Update = &cli.Command{
	Name:  "update",
	Usage: "update the application",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "notify",
			Usage: "toggle update notification",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		notify := cmd.Bool("notify")
		if notify {
			// get current
			updateNotify, err := config.Get[bool](ctx, "updateNotify")
			if err != nil {
				return fmt.Errorf("failed to get updateNotify from config: %w", err)
			}
			// set opposite
			if err := config.Set(ctx, "updateNotify", !updateNotify); err != nil {
				return fmt.Errorf("failed to set updateNotify in config: %w", err)
			}
			// print status
			if !updateNotify {
				fmt.Println("Update notifications are now enabled.")
			} else {
				fmt.Println("Update notifications are now disabled.")
			}
			return nil
		}
		return update.Update(ctx, false)
	},
}
