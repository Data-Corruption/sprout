package router

import (
	"net/http"
	"sprout/internal/app"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/go-chi/chi/v5"
)

func New(a *app.App) *chi.Mux {
	r := chi.NewRouter()

	// inject logger into request context for xhttp.Error calls
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(xlog.IntoContext(r.Context(), a.Log)))
		})
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("sprout " + a.Version))
	})

	r.Get("/update", func(w http.ResponseWriter, r *http.Request) {
		if updateAvailable, err := a.UpdateCheck(); err != nil {
			xhttp.Error(r.Context(), w, &xhttp.Err{
				Code: http.StatusInternalServerError,
				Msg:  "Failed to check for updates",
				Err:  err,
			})
			return
		} else if !updateAvailable {
			w.Write([]byte("Already up to date.\n"))
			return
		}

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
