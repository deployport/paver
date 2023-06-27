package templates

import (
	"strings"
	"text/template"
)

// InstallStandardTemplateFunctions installs the standard template functions
func InstallStandardTemplateFunctions(tmpl *template.Template) {
	tmpl.Funcs(template.FuncMap{
		"lowercase": strings.ToLower,
		"uppercase": strings.ToUpper,
	})
}
