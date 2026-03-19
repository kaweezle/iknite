package secrets

import (
	"embed"
	"fmt"
	"strings"
	"text/template"

	sprig "github.com/go-task/slim-sprig/v3"
)

//go:embed templates
var templates embed.FS

const (
	SecretsTemplateName    = "secrets.sops.yaml.gotmpl" //nolint:gosec // filename, not a secret
	SopsConfigTemplateName = "sops.yaml.gotmpl"
)

type TemplateData struct {
	Values map[string]any
}

func renderTemplate(name string, data *TemplateData) (string, error) {
	t, err := template.New(name).Funcs(sprig.TxtFuncMap()).ParseFS(templates, "templates/"+name)
	if err != nil {
		return "", fmt.Errorf("while parsing template %s: %w", name, err)
	}
	var output strings.Builder
	err = t.Execute(&output, data)
	if err != nil {
		return "", fmt.Errorf("while executing template %s: %w", name, err)
	}
	return output.String(), nil
}
