// Package ui provides embedded frontend templates and assets.
package ui

import (
	"embed"
	"fmt"
	"html/template"
	"io"

	"sprout/pkg/asset"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed assets/css/output.css
var cssFS embed.FS

//go:embed assets/js/output.js
var jsFS embed.FS

// UI holds parsed templates and static assets.
// Create once at app startup via New().
type UI struct {
	templates *template.Template
	CSS, JS   *asset.Asset
}

// New parses all embedded templates and loads static assets.
func New() (*UI, error) {
	t, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	css, err := asset.New(cssFS, "assets/css/output.css", ".css", "text/css; charset=utf-8")
	if err != nil {
		return nil, fmt.Errorf("failed to load CSS: %w", err)
	}

	js, err := asset.New(jsFS, "assets/js/output.js", ".js", "application/javascript; charset=utf-8")
	if err != nil {
		return nil, fmt.Errorf("failed to load JS: %w", err)
	}

	return &UI{
		templates: t,
		CSS:       css,
		JS:        js,
	}, nil
}

// Execute renders a template by name to the writer.
func (u *UI) Execute(w io.Writer, name string, data any) error {
	return u.templates.ExecuteTemplate(w, name, data)
}
