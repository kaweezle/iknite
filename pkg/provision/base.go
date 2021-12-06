package provision

import (
	"embed"
)

//go:embed base
var content embed.FS

func ApplyBaseKustomizations(data interface{}) error {
	return ApplyKustomizations(&content, "base", data)
}
