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
	"net/url"
	"path"

	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/kustomize/kyaml/resid"
)

// cSpell: enable

//go:embed base
var content embed.FS

func ApplyBaseKustomizations(dirname string, data any) ([]resid.ResId, error) {
	exists := false
	var err error
	_, err = url.Parse(dirname)
	if err == nil {
		exists = true
	} else {
		exists, err = utils.Exists(path.Join(dirname, "kustomization.yaml"))
		if err != nil {
			return nil, errors.Wrap(err, "While testing for directory")
		}
	}

	if exists {
		log.WithField("directory", dirname).Info("Applying base kustomization...")
		return ApplyLocalKustomizations(dirname)
	} else {
		log.Info("Apply embedded kustomization...")

		return ApplyEmbeddedKustomizations(&content, "base", data)
	}
}
