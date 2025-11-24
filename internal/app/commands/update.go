package commands

import (
	"context"
	"fmt"
	"sprout/internal/app"
	"sprout/internal/platform/database"

	"github.com/urfave/cli/v3"
)

var Update = register(func(a *app.App) *cli.Command {
	return &cli.Command{
		Name:  "update",
		Usage: "update the app",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "notify",
				Usage: "toggle update notification",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			notify := cmd.Bool("notify")
			if notify {
				var updateNotifications bool
				if err := database.UpdateConfig(a.DB, func(cfg *database.Configuration) error {
					cfg.UpdateNotifications = !cfg.UpdateNotifications
					updateNotifications = cfg.UpdateNotifications
					return nil
				}); err != nil {
					return fmt.Errorf("failed to update notification setting in config: %w", err)
				}
				// print status
				if !updateNotifications {
					fmt.Println("Update notifications are now enabled.")
				} else {
					fmt.Println("Update notifications are now disabled.")
				}
				return nil
			}
			return a.Update(false)
		},
	}
})
