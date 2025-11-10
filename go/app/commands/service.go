package commands

import (
	"context"
	"fmt"
	"net/http"
	"sprout/go/app"
	"sprout/go/platform/http/server"
	"sprout/go/platform/update"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/Data-Corruption/stdx/xnet"
	"github.com/urfave/cli/v3"
)

var Service = &cli.Command{
	Name:  "service",
	Usage: "service management commands",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		// get app info
		app, ok := app.FromContext(ctx)
		if !ok {
			return fmt.Errorf("failed to get app info from context")
		}

		// get service name / env file path
		if app.Name == "" || app.Storage == "" {
			return fmt.Errorf("app name or storage path not found")
		}
		serviceName := app.Name + ".service"
		envFilePath := fmt.Sprintf("%s/%s.env", app.Storage, app.Name)

		// print service management commands
		fmt.Printf("🖧 Service Cheat Sheet\n")
		fmt.Printf("    Start:   systemctl --user start %s\n", serviceName)
		fmt.Printf("    Stop:    systemctl --user stop %s\n", serviceName)
		fmt.Printf("    Status:  systemctl --user status %s\n", serviceName)
		fmt.Printf("    Restart: systemctl --user restart %s\n", serviceName)
		fmt.Printf("    Reset:   systemctl --user reset-failed %s\n", serviceName)
		fmt.Printf("    Enable:  systemctl --user enable %s\n", serviceName)
		fmt.Printf("    Disable: systemctl --user disable %s\n", serviceName)
		fmt.Printf("    Logs:    journalctl --user -u %s -n 200 --no-pager\n", serviceName)
		fmt.Printf("    Env:     edit %s then restart the service\n", envFilePath)

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

				var srv *xhttp.Server

				// hello world handler
				mux := http.NewServeMux()
				mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("Hello World 4\n"))
				})
				mux.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
					// daemon update example. add auth ofc, etc
					w.Write([]byte("Starting update...\n"))
					if err := update.Update(ctx, true); err != nil {
						xlog.Errorf(ctx, "/update update start failed: %s", err)
					}
					srv.Shutdown(nil)
				})

				// create server
				var err error
				srv, err = server.New(ctx, mux)
				if err != nil {
					return fmt.Errorf("failed to create server: %w", err)
				}
				server.IntoContext(ctx, srv)

				// start http server
				if err := srv.Listen(); err != nil {
					return fmt.Errorf("server stopped with error: %w", err)
				} else {
					fmt.Println("server stopped gracefully")
				}

				return nil
			},
		},
	},
}
