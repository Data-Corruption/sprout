package commands

import (
	"context"
	"fmt"
	"sprout/internal/app"
	"sprout/internal/platform/database"
	"sprout/internal/platform/http/server"
	"sprout/internal/platform/http/server/router"

	"github.com/Data-Corruption/stdx/xnet"
	"github.com/urfave/cli/v3"
)

var Service = register(func(a *app.App) *cli.Command {
	if !a.ServiceEnabled {
		return nil
	}
	return &cli.Command{
		Name:  "service",
		Usage: "service management commands",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			// get service name / env file path
			if a.Name == "" || a.StorageDir == "" {
				return fmt.Errorf("app name or storage path not found")
			}
			serviceName := a.Name + ".service"
			envFilePath := fmt.Sprintf("%s/%s.env", a.StorageDir, a.Name)

			// print service management commands
			fmt.Printf("🖧 Service Cheat Sheet\n\n")
			fmt.Printf("    Status:  systemctl --user status %s\n", serviceName)
			fmt.Printf("    Enable:  systemctl --user enable %s\n", serviceName)
			fmt.Printf("    Disable: systemctl --user disable %s\n\n", serviceName)
			fmt.Printf("    Start:   systemctl --user start %s\n", serviceName)
			fmt.Printf("    Stop:    systemctl --user stop %s\n", serviceName)
			fmt.Printf("    Restart: systemctl --user restart %s\n\n", serviceName)
			fmt.Printf("    Reset:   systemctl --user reset-failed %s\n\n", serviceName)
			fmt.Printf("    Env:     edit %s then restart the service\n\n", envFilePath)
			fmt.Printf("    Logs:        journalctl --user -u %s -n 200 --no-pager\n", serviceName)
			fmt.Printf("    Update Logs: journalctl --user -u %s-update* -n 200 -f\n", a.Name)

			return nil
		},
		Commands: []*cli.Command{
			{
				Name:        "run",
				Description: "Runs service in foreground. Typically called by systemd. If you need to run it manually/unmanaged, use this command.",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					// wait for network (systemd user mode Wants/After is unreliable)
					if err := xnet.Wait(ctx, 0); err != nil {
						return fmt.Errorf("failed to wait for network: %w", err)
					}

					// get port, handle override
					port := cmd.Int("port")
					if port == 0 {
						cfg, err := database.ViewConfig(a.DB)
						if err != nil {
							return fmt.Errorf("failed to get configuration from database: %w", err)
						}
						port = cfg.Port
					}

					// create server
					mux := router.New(a)
					if err := server.New(a, port, mux); err != nil {
						return fmt.Errorf("failed to create server: %w", err)
					}

					// start http server
					if err := a.Server.Listen(); err != nil { // blocks until server stops or shutdown signal received
						return fmt.Errorf("server stopped with error: %w", err)
					} else {
						fmt.Println("server stopped gracefully")
					}

					return nil
				},
			},
		},
	}
})
