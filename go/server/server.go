package server

import (
	"context"
	"fmt"
	"net/http"
	"sprout/go/database/config"
	"sprout/go/sdnotify"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/Data-Corruption/stdx/xlog"
)

type ctxKey struct{}

func IntoContext(ctx context.Context, srv *xhttp.Server) context.Context {
	return context.WithValue(ctx, ctxKey{}, srv)
}

func FromContext(ctx context.Context) *xhttp.Server {
	if srv, ok := ctx.Value(ctxKey{}).(*xhttp.Server); ok {
		return srv
	}
	return nil
}

func New(ctx context.Context, handler http.Handler) (*xhttp.Server, error) {
	// get http server related stuff from config
	port, err := config.Get[int](ctx, "port")
	if err != nil {
		return nil, fmt.Errorf("failed to get port from config: %w", err)
	}
	useTLS, err := config.Get[bool](ctx, "useTLS")
	if err != nil {
		return nil, fmt.Errorf("failed to get useTLS from config: %w", err)
	}
	tlsKeyPath, err := config.Get[string](ctx, "tlsKeyPath")
	if err != nil {
		return nil, fmt.Errorf("failed to get tlsKeyPath from config: %w", err)
	}
	tlsCertPath, err := config.Get[string](ctx, "tlsCertPath")
	if err != nil {
		return nil, fmt.Errorf("failed to get tlsCertPath from config: %w", err)
	}

	// create http server
	var srv *xhttp.Server
	srv, err = xhttp.NewServer(&xhttp.ServerConfig{
		Addr:        fmt.Sprintf(":%d", port),
		UseTLS:      useTLS,
		TLSKeyPath:  tlsKeyPath,
		TLSCertPath: tlsCertPath,
		Handler:     handler,
		AfterListen: func() {
			// tell systemd we're ready
			status := fmt.Sprintf("Listening on %s", srv.Addr())
			if err := sdnotify.Ready(status); err != nil {
				xlog.Warnf(ctx, "sd_notify READY failed: %v", err)
			}
			fmt.Printf("Server is listening on http://localhost%s\n", srv.Addr())
		},
		OnShutdown: func() {
			// tell systemd we’re stopping
			if err := sdnotify.Stopping("Shutting down"); err != nil {
				xlog.Debugf(ctx, "sd_notify STOPPING failed: %v", err)
			}
			fmt.Println("shutting down, cleaning up resources ...")
		},
	})
	return srv, err
}
