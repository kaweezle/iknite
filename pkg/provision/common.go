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
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

func createTempKustomizeDirectory(content *embed.FS, fs filesys.FileSystem, tmpdir string, dirname string, data interface{}) error {
	log.WithFields(log.Fields{
		"tmpdir":  tmpdir,
		"dirname": dirname,
		"data":    data,
	}).Trace("Start creating directory")

	files, err := content.ReadDir(dirname)
	if err != nil {
		return errors.Wrapf(err, "While reading files of %s", dirname)
	}
	for _, entry := range files {
		if !entry.IsDir() {
			inPath := fmt.Sprintf("%s/%s", dirname, entry.Name())
			outPath := fmt.Sprintf("%s/%s", tmpdir, entry.Name())

			log.WithField("path", inPath).Trace("Reading file")
			payload, err := content.ReadFile(inPath)
			if err != nil {
				return errors.Wrapf(err, "While reading embedded file %s", entry.Name())
			}

			if filepath.Ext(entry.Name()) == ".tmpl" {
				log.WithField("path", inPath).Trace("Is template")
				t, err := template.New("tmp").Parse(string(payload))
				if err != nil {
					return errors.Wrapf(err, "While reading template %s", entry.Name())
				}
				buf := new(bytes.Buffer)
				log.WithField("path", inPath).
					WithField("data", data).
					Trace("Rendering")
				err = t.Execute(buf, data)
				if err != nil {
					return errors.Wrap(err, "failed to create a manifest file")
				}
				payload = buf.Bytes()
				outPath = strings.TrimSuffix(outPath, ".tmpl")
			}
			log.WithField("outPath", outPath).Trace("Writing content")
			err = fs.WriteFile(outPath, payload)
			if err != nil {
				return errors.Wrapf(err, "While writing %s to temp dir %s", entry.Name(), tmpdir)
			}
		}
	}
	return nil
}

func ApplyKustomizations(fs filesys.FileSystem, dirname string) error {
	out, err := RunKustomizations(fs, dirname)
	if err != nil {
		return errors.Wrap(err, "While applying templates")
	}
	buffer := bytes.Buffer{}
	buffer.Write(out)

	cmd := exec.Command(constants.KubectlCmd, "apply", "-f", "-")
	cmd.Stdin = &buffer
	out, err = cmd.CombinedOutput()
	log.Trace(string(out))
	if err != nil {
		return errors.Wrap(err, "While applying templates")
	}
	return nil
}

func ApplyLocalKustomizations(dirname string) error {
	return ApplyKustomizations(filesys.MakeFsOnDisk(), dirname)
}

func ApplyEmbeddedKustomizations(content *embed.FS, dirname string, data interface{}) error {
	fs := filesys.MakeFsInMemory()
	if err := fs.MkdirAll(dirname); err != nil {
		return err
	}

	if err := createTempKustomizeDirectory(content, fs, dirname, dirname, data); err != nil {
		return err
	}
	return ApplyKustomizations(fs, dirname)
}

func EnablePlugins(opts *krusty.Options) *krusty.Options {
	opts.PluginConfig = types.EnabledPluginConfig(types.BploUseStaticallyLinked)
	opts.PluginConfig.FnpLoadingOptions.EnableExec = true
	opts.PluginConfig.FnpLoadingOptions.AsCurrentUser = true
	opts.PluginConfig.HelmConfig.Command = "helm"
	return opts
}

func RunKustomizations(fs filesys.FileSystem, dirname string) (yaml []byte, err error) {

	opts := EnablePlugins(krusty.MakeDefaultOptions())
	k := krusty.MakeKustomizer(opts)
	var resources resmap.ResMap
	if resources, err = k.Run(fs, dirname); err != nil {
		return
	}

	yaml, err = resources.AsYaml()
	return
}
