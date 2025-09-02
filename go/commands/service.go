package commands

import (
	"context"
	"fmt"
	"net/http"
	"sprout/go/database/datapath"
	"sprout/go/server"
	"sprout/go/update"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/Data-Corruption/stdx/xnet"
	"github.com/urfave/cli/v3"
)

type AppNameKey struct{}

var Service = &cli.Command{
	Name:  "service",
	Usage: "service management commands",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		// get service name
		appName := ctx.Value(AppNameKey{}).(string)
		if appName == "" {
			return fmt.Errorf("app name not found")
		}
		serviceName := appName + ".service"

		// get env file path
		dataPath := datapath.FromContext(ctx)
		envFilePath := fmt.Sprintf("%s/%s.env", dataPath, appName)

		// print service management commands
		fmt.Printf("🖧 Service Cheat Sheet\n")
		fmt.Printf("    Start:   systemctl --user start %s\n", serviceName)
		fmt.Printf("    Stop:    systemctl --user stop %s\n", serviceName)
		fmt.Printf("    Status:  systemctl --user status %s\n", serviceName)
		fmt.Printf("    Restart: systemctl --user restart %s\n", serviceName)
		fmt.Printf("    Reset:   systemctl --user reset-failed %s\n", serviceName)
		fmt.Printf("    Logs:    journalctl --user -u %s -n 200 --no-pager\n", serviceName)
		fmt.Printf("    Env:     edit %s then restart the service\n", envFilePath)

		fmt.Println("\nIf you've manually edited the unit file, you'll need to reload the systemd")
		fmt.Println("manager configuration 'systemctl --user daemon-reload'. Keep in mind updating")
		fmt.Println("will overwrite your changes, so keep a backup.")

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
