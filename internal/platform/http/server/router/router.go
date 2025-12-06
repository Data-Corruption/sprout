package router

import (
	"embed"
	"encoding/json"
	"html/template"
	"net/http"
	"sprout/internal/app"
	"sprout/internal/platform/database"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/index.html
var tmplFS embed.FS

var tmpl = template.Must(template.ParseFS(tmplFS, "templates/index.html"))

func New(a *app.App) *chi.Mux {
	r := chi.NewRouter()

	// inject logger into request context for xhttp.Error calls
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(xlog.IntoContext(r.Context(), a.Log)))
		})
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		cfg, err := database.ViewConfig(a.DB)
		if err != nil {
			xhttp.Error(r.Context(), w, err)
			return
		}

		data := map[string]any{
			"Favicon":         template.URL(`data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text x='50%' y='.9em' font-size='90' text-anchor='middle'>🌱</text></svg>`),
			"Title":           "sprout",
			"Version":         a.Version,
			"UpdateAvailable": cfg.UpdateAvailable,
		}
		if err := tmpl.Execute(w, data); err != nil {
			xhttp.Error(r.Context(), w, err)
			return
		}
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Get("/update", func(w http.ResponseWriter, r *http.Request) {
		if err := a.DetachUpdate(); err != nil {
			xhttp.Error(r.Context(), w, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})

	r.Get("/update-status", func(w http.ResponseWriter, r *http.Request) {
		cfg, err := database.ViewConfig(a.DB)
		if err != nil {
			xhttp.Error(r.Context(), w, err)
			return
		}

		updating := cfg.UpdateFollowup != "" && cfg.UpdateFollowup == a.Version
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]bool{"updating": updating}); err != nil {
			xhttp.Error(r.Context(), w, err)
		}
	})

	return r
}
