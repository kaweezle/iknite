/*
Copyright © 2025 Antoine Martin <antoine@openance.com>

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
package kustomize

// cSpell: words filesys kyaml Bplo kustomizer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"

	"github.com/kaweezle/iknite/pkg/host"
)

// EnablePlugins configures kustomize options with plugins and exec enabled.
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

// BuildOnFileSystem runs kustomize on the given directory using the provided file system and returns the resulting
// resource map.
func BuildOnFileSystem(fs filesys.FileSystem, dir string) (resmap.ResMap, error) {
	opts := EnablePlugins(krusty.MakeDefaultOptions())
	k := krusty.MakeKustomizer(opts)
	resources, err := k.Run(fs, dir)
	if err != nil {
		return nil, fmt.Errorf("failed to run kustomize on %s: %w", dir, err)
	}
	return resources, nil
}

// Build runs kustomize on the given directory with plugins enabled and returns
// the resulting resource map.
func Build(dir string) (resmap.ResMap, error) {
	return BuildOnFileSystem(filesys.MakeFsOnDisk(), dir)
}

// WriteToWriter converts a resource map to YAML and writes it to the given writer.
func WriteToWriter(resources resmap.ResMap, out io.Writer) error {
	yamlData, err := resources.AsYaml()
	if err != nil {
		return fmt.Errorf("failed to convert resources to YAML: %w", err)
	}
	if _, err = out.Write(yamlData); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

// SplitResMapToDir splits a resource map into individual <Kind>-<name>.yaml files
// in the given directory. The directory is created if it does not exist.

func SplitResMapToDir(fs host.FileSystem, resources resmap.ResMap, destDir string) error {
	if err := fs.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}
	for _, resource := range resources.Resources() {
		kind := resource.GetKind()
		name := resource.GetName()

		yamlData, err := yaml.Marshal(resource)
		if err != nil {
			return fmt.Errorf("failed to marshal resource %s/%s: %w", kind, name, err)
		}

		safeName := strings.ReplaceAll(name, ":", "_")
		filename := fmt.Sprintf("%s-%s.yaml", kind, safeName)
		path := filepath.Join(destDir, filename)

		if err := fs.WriteFile(path, yamlData, 0o644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", path, err)
		}
		_, _ = fmt.Fprintf(os.Stderr, "Created: %s\n", path)
	}
	return nil
}

// SplitYAMLToDir splits a multi-document YAML byte payload into individual
// <Kind>-<name>.yaml files in the given directory.
// The directory is created if it does not exist.
// Returns an error for documents that are missing required kind or metadata fields.
func SplitYAMLToDir(fs afero.Fs, yamlData []byte, destDir string) error {
	if err := fs.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	docs := strings.Split(string(yamlData), "\n---\n")
	for i, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" || doc == "---" {
			continue
		}

		var resource map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &resource); err != nil {
			return fmt.Errorf("failed to parse resource %d: %w", i, err)
		}

		kind, ok := resource["kind"].(string)
		if !ok {
			return fmt.Errorf("resource %d missing 'kind' field", i)
		}

		metadata, ok := resource["metadata"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("resource %d missing 'metadata' field", i)
		}

		name, ok := metadata["name"].(string)
		if !ok {
			return fmt.Errorf("resource %d missing 'metadata.name' field", i)
		}

		safeName := strings.ReplaceAll(name, ":", "_")
		filename := fmt.Sprintf("%s-%s.yaml", kind, safeName)
		path := filepath.Join(destDir, filename)

		if err := afero.WriteFile(fs, path, []byte(doc), 0o644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", path, err)
		}
		fmt.Fprintf(os.Stderr, "Created: %s\n", path)
	}
	return nil
}
