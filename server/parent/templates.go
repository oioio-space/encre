package parent

import (
	"embed"
	"fmt"
	"html/template"
)

//go:embed templates/*.html
var templateFS embed.FS

// parseTemplates parses every embedded template into one [html/template.Template]
// set, sharing the "base" layout and partials across pages. html/template is
// mandatory here, never text/template: every page interpolates
// parent-supplied text (a child's pseudo, first and foremost) that must be
// HTML-escaped before it reaches the response.
func parseTemplates() (*template.Template, error) {
	tmpl, err := template.New("").ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	return tmpl, nil
}
