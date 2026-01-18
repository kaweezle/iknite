/*
Copyright © 2021 Antoine Martin <antoine@openance.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package provision

// cSpell: disable
import (
	"embed"
	"fmt"
	"net/url"
	"path"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/kustomize/kyaml/resid"

	"github.com/kaweezle/iknite/pkg/utils"
)

// cSpell: enable

//go:embed base
var content embed.FS

// IsBaseKustomizationAvailable checks if a kustomization.yaml file is available
// in the specified directory or if the directory is a URL.
func IsBaseKustomizationAvailable(dirname string) (bool, error) {
	var exists bool
	var err error
	_, err = url.Parse(dirname)
	if err == nil {
		exists = true
	} else {
		exists, err = utils.Exists(path.Join(dirname, "kustomization.yaml"))
		if err != nil {
			return false, fmt.Errorf("while testing for directory: %w", err)
		}
	}
	return exists, nil
}

// ApplyBaseKustomizations applies the kustomizations located in the specified
// directory if available, otherwise applies the embedded kustomizations.
func ApplyBaseKustomizations(dirname string, data any) ([]resid.ResId, error) {
	if ok, _ := IsBaseKustomizationAvailable(dirname); ok { //nolint:errcheck // ignore error here
		log.WithField("directory", dirname).Info("Applying base kustomization...")
		return ApplyLocalKustomizations(dirname)
	} else {
		log.Info("Apply embedded kustomization...")

		return ApplyEmbeddedKustomizations(&content, "base", data)
	}
}
