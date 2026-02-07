package web

import (
	"embed"
	"html/template"
	"net/http"
)

type Templates struct {
	fs   embed.FS
	base *template.Template
}

func NewTemplates(fs embed.FS) (*Templates, error) {
	base, err := template.ParseFS(fs, "templates/layout.html", "templates/partials/*.html")
	if err != nil {
		return nil, err
	}
	return &Templates{fs: fs, base: base}, nil
}

func (t *Templates) Render(w http.ResponseWriter, name string, data any) error {
	tmpl, err := t.base.Clone()
	if err != nil {
		return err
	}
	if _, err := tmpl.ParseFS(t.fs, "templates/"+name); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.ExecuteTemplate(w, "layout", data)
}

func (t *Templates) RenderPartial(w http.ResponseWriter, name string, data any) error {
	tmpl, err := t.base.Clone()
	if err != nil {
		return err
	}
	if _, err := tmpl.ParseFS(t.fs, "templates/partials/"+name); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.ExecuteTemplate(w, name, data)
}
