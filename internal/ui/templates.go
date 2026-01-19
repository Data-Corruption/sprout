package ui

import (
	"embed"
	"fmt"
	"html/template"
	"io"
)

//go:embed templates/*.html
var templateFS embed.FS

// Templates manages parsed HTML templates.
type Templates struct {
	parsed *template.Template
}

// NewTemplates parses all embedded templates.
// Call this once at app startup.
func NewTemplates() (*Templates, error) {
	t, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}
	return &Templates{parsed: t}, nil
}

// Execute renders a template by name to the writer.
func (t *Templates) Execute(w io.Writer, name string, data any) error {
	return t.parsed.ExecuteTemplate(w, name, data)
}
