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

// cSpell: words kustomizer filesys crds tmpl Bplo
// cSpell: disable
import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// cSpell: enable

func createTempKustomizeDirectory(
	content *embed.FS, fs filesys.FileSystem, tempdir string, dirname string, data any,
) error {
	log.WithFields(log.Fields{
		"tempdir": tempdir,
		"dirname": dirname,
		"data":    data,
	}).Trace("Start creating directory")

	files, err := content.ReadDir(dirname)
	if err != nil {
		return fmt.Errorf("while reading files of %s: %w", dirname, err)
	}
	for _, entry := range files {
		if entry.IsDir() {
			continue
		}

		inPath := fmt.Sprintf("%s/%s", dirname, entry.Name())
		outPath := fmt.Sprintf("%s/%s", tempdir, entry.Name())

		log.WithField("path", inPath).Trace("Reading file")
		payload, err := content.ReadFile(inPath)
		if err != nil {
			return fmt.Errorf("while reading embedded file %s: %w", entry.Name(), err)
		}

		if filepath.Ext(entry.Name()) == ".tmpl" {
			log.WithField("path", inPath).Trace("Is template")
			var t *template.Template
			t, err = template.New("tmp").Parse(string(payload))
			if err != nil {
				return fmt.Errorf("while reading template %s: %w", entry.Name(), err)
			}
			buf := new(bytes.Buffer)
			log.WithField("path", inPath).
				WithField("data", data).
				Trace("Rendering")
			err = t.Execute(buf, data)
			if err != nil {
				return fmt.Errorf("failed to create a manifest file: %w", err)
			}
			payload = buf.Bytes()
			outPath = strings.TrimSuffix(outPath, ".tmpl")
		}
		log.WithField("outPath", outPath).Trace("Writing content")
		err = fs.WriteFile(outPath, payload)
		if err != nil {
			return fmt.Errorf("while writing %s to temp dir %s: %w", entry.Name(), tempdir, err)
		}
	}
	return nil
}

func ApplyKustomizations(fs filesys.FileSystem, dirname string) (resmap.ResMap, error) {
	resources, err := RunKustomizations(fs, dirname)
	if err != nil {
		return nil, fmt.Errorf("while building templates: %w", err)
	}

	return resources, nil
}

func ApplyLocalKustomizations(dirname string) (resmap.ResMap, error) {
	return ApplyKustomizations(filesys.MakeFsOnDisk(), dirname)
}

func ApplyEmbeddedKustomizations(
	content *embed.FS,
	dirname string,
	data any,
) (resmap.ResMap, error) {
	fs := filesys.MakeFsInMemory()
	if err := fs.MkdirAll(dirname); err != nil {
		return nil, fmt.Errorf("failed to create directory in memory: %w", err)
	}

	if err := createTempKustomizeDirectory(content, fs, dirname, dirname, data); err != nil {
		return nil, err
	}
	return ApplyKustomizations(fs, dirname)
}

func EnablePlugins(opts *krusty.Options) *krusty.Options {
	opts.PluginConfig = types.EnabledPluginConfig(
		types.BploUseStaticallyLinked,
	) // cSpell: disable-line
	opts.PluginConfig.FnpLoadingOptions.EnableExec = true
	opts.PluginConfig.FnpLoadingOptions.AsCurrentUser = true
	opts.PluginConfig.HelmConfig.Command = "helm"
	opts.LoadRestrictions = types.LoadRestrictionsNone
	return opts
}

func RunKustomizations(fs filesys.FileSystem, dirname string) (resmap.ResMap, error) {
	opts := EnablePlugins(krusty.MakeDefaultOptions())
	k := krusty.MakeKustomizer(opts)
	resources, err := k.Run(fs, dirname)
	if err != nil {
		return resources, fmt.Errorf("failed to run kustomize: %w", err)
	}
	return resources, nil
}
