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

import (
	"embed"
	"path"

	"github.com/kaweezle/iknite/pkg/utils"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/kustomize/kyaml/resid"
)

//go:embed base
var content embed.FS

func ApplyBaseKustomizations(dirname string, data interface{}) ([]resid.ResId, error) {

	exits, err := utils.Exists(path.Join(dirname, "kustomization.yaml"))
	if err != nil {
		return nil, err
	}

	if exits {
		log.WithField("directory", dirname).Info("Applying base kustomization...")
		return ApplyLocalKustomizations(dirname)
	} else {
		log.Info("Apply embedded kustomization...")

		return ApplyEmbeddedKustomizations(&content, "base", data)
	}
}
