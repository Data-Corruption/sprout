package router

import (
	"net/http"
	"sprout/internal/app"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/go-chi/chi/v5"
)

const HashParam = "h"

func New(a *app.App) *chi.Mux {
	r := chi.NewRouter()

	// inject logger into request context for xhttp.Error calls
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(xlog.IntoContext(r.Context(), a.Log)))
		})
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("sprout\n"))
	})

	r.Post("/update", func(w http.ResponseWriter, r *http.Request) {
		if err := a.Update(true); err != nil {
			xhttp.Error(r.Context(), w, &xhttp.Err{
				Code: http.StatusInternalServerError,
				Msg:  "Failed to start update",
				Err:  err,
			})
			return
		}
		if err := a.Net.Server.Shutdown(nil); err != nil {
			a.Log.Errorf("Failed to shutdown server: %v", err)
		}
		w.Write([]byte("Starting update...\n"))
	})

	return r
}

//lint:file-ignore SA1012 nil is intentional
