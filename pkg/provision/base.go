package provision

import (
	"embed"

	log "github.com/sirupsen/logrus"
)

//go:embed base
var content embed.FS

func ApplyBaseKustomizations(data interface{}) error {
	log.Info("Apply base kustomization...")
	return ApplyKustomizations(&content, "base", data)
}
