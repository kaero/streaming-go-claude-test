package templates

import (
	"embed"
	"html/template"
	"io"
	"log"
)

//go:embed templates/*.gohtml
var templateFS embed.FS

// Templates holds parsed templates
type Templates struct {
	list   *template.Template
	player *template.Template
}

// New creates a new Templates instance
func New() *Templates {
	t := &Templates{}
	
	// Parse templates from embedded filesystem
	var err error
	
	t.list, err = template.ParseFS(templateFS, "templates/list.gohtml")
	if err != nil {
		log.Fatalf("Failed to parse list template: %v", err)
	}
	
	t.player, err = template.ParseFS(templateFS, "templates/player.gohtml")
	if err != nil {
		log.Fatalf("Failed to parse player template: %v", err)
	}
	
	return t
}

// ListTemplate renders the video list template
func (t *Templates) ListTemplate(w io.Writer, data interface{}) error {
	return t.list.Execute(w, data)
}

// PlayerTemplate renders the video player template
func (t *Templates) PlayerTemplate(w io.Writer, data interface{}) error {
	return t.player.Execute(w, data)
}