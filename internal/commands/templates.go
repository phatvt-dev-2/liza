package commands

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

var cmdFuncMap = template.FuncMap{
	"deref": func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	},
}

var cmdTmpl = template.Must(
	template.New("").Funcs(cmdFuncMap).ParseFS(templatesFS, "templates/*.tmpl"),
)

func executeCommandTemplate(name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := cmdTmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("template %s: %w", name, err)
	}
	return buf.String(), nil
}
