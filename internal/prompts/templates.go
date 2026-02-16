package prompts

import (
	"bytes"
	"embed"
	"text/template"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

var funcMap = template.FuncMap{
	"deref": func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	},
	"sub": func(a, b int) int {
		return a - b
	},
}

var tmpl = template.Must(
	template.New("").Funcs(funcMap).ParseFS(templatesFS, "templates/*.tmpl"),
)

func executeTemplate(name string, data any) string {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		panic("template: " + err.Error())
	}
	return buf.String()
}
