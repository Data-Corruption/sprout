package server

import (
	"fmt"
	"net/http"
	"sprout/internal/app"
	"sprout/pkg/sdnotify"

	"github.com/Data-Corruption/stdx/xhttp"
)

func New(app *app.App, port int, handler http.Handler) error {
	// create http server
	var err error
	app.Server, err = xhttp.NewServer(&xhttp.ServerConfig{
		Addr:    fmt.Sprintf(":%d", port),
		UseTLS:  false,
		Handler: handler,
		AfterListen: func() {
			// tell systemd we're ready
			fmt.Println("Listening on", app.BaseURL) // for user
			status := fmt.Sprintf("Listening on %s", app.Server.Addr())
			if err := sdnotify.Ready(status); err != nil {
				app.Log.Warnf("sd_notify READY failed: %v", err)
			}
		},
		OnShutdown: func() {
			// tell systemd we’re stopping
			if err := sdnotify.Stopping("Shutting down"); err != nil {
				app.Log.Debugf("sd_notify STOPPING failed: %v", err)
			}
			fmt.Println("shutting down, cleaning up resources ...")
		},
	})
	return err
}
