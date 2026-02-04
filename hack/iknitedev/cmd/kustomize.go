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
package cmd

// cSpell: words filesys kyaml Bplo kustomizer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"
)

// CreateKustomizeCmd creates the kustomize command.
func CreateKustomizeCmd(fs afero.Fs) *cobra.Command {
	kustomizeCmd := &cobra.Command{
		Use:   "kustomize <directory> [destination]",
		Short: "Run kustomize on a directory",
		Long: `Run kustomize on a directory with plugins and exec enabled.

If a destination directory is provided, the produced resources will be split
into individual files named <kind>-<name>.yaml.

Examples:
  # Print kustomization to stdout
  iknitedev kustomize /path/to/kustomization

  # Split resources into individual files
  iknitedev kustomize /path/to/kustomization /path/to/output`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKustomizeWithCmd(cmd, fs, args)
		},
	}

	return kustomizeCmd
}

// enablePlugins configures kustomize options with plugins and exec enabled.
func enablePlugins(opts *krusty.Options) *krusty.Options {
	opts.PluginConfig = types.EnabledPluginConfig(
		types.BploUseStaticallyLinked,
	) // cSpell: disable-line
	opts.PluginConfig.FnpLoadingOptions.EnableExec = true
	opts.PluginConfig.FnpLoadingOptions.AsCurrentUser = true
	opts.PluginConfig.HelmConfig.Command = "helm"
	opts.LoadRestrictions = types.LoadRestrictionsNone
	return opts
}

// runKustomizeWithCmd executes the kustomize operation with command output.
func runKustomizeWithCmd(cmd *cobra.Command, fs afero.Fs, args []string) error {
	kustomizationDir := args[0]
	var destDir string
	if len(args) > 1 {
		destDir = args[1]
	}

	// Check if kustomization directory exists
	exists, err := afero.DirExists(fs, kustomizationDir)
	if err != nil {
		return fmt.Errorf("failed to check kustomization directory: %w", err)
	}
	if !exists {
		return fmt.Errorf("kustomization directory does not exist: %s", kustomizationDir)
	}

	// Check if kustomization.yaml exists
	kustomizationFile := filepath.Join(kustomizationDir, "kustomization.yaml")
	kustomizationExists, err := afero.Exists(fs, kustomizationFile)
	if err != nil {
		return fmt.Errorf("failed to check kustomization.yaml: %w", err)
	}
	if !kustomizationExists {
		return fmt.Errorf("kustomization.yaml not found in: %s", kustomizationDir)
	}

	// Run kustomize
	opts := enablePlugins(krusty.MakeDefaultOptions())
	k := krusty.MakeKustomizer(opts)
	resources, err := k.Run(filesys.MakeFsOnDisk(), kustomizationDir)
	if err != nil {
		return fmt.Errorf("failed to run kustomize: %w", err)
	}

	// If no destination directory, print to stdout
	if destDir == "" {
		out, err := resources.AsYaml()
		if err != nil {
			return fmt.Errorf("failed to convert resources to YAML: %w", err)
		}
		_, err = cmd.OutOrStdout().Write(out)
		return err
	}

	// Create destination directory if it doesn't exist
	if err := fs.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Split resources directly from resmap (avoids double marshalling)
	return splitResourcesFromResMap(fs, resources, destDir)
}

// runKustomize executes the kustomize operation (for backward compatibility).
func runKustomize(fs afero.Fs, args []string) error {
	kustomizationDir := args[0]
	var destDir string
	if len(args) > 1 {
		destDir = args[1]
	}

	// Check if kustomization directory exists
	exists, err := afero.DirExists(fs, kustomizationDir)
	if err != nil {
		return fmt.Errorf("failed to check kustomization directory: %w", err)
	}
	if !exists {
		return fmt.Errorf("kustomization directory does not exist: %s", kustomizationDir)
	}

	// Check if kustomization.yaml exists
	kustomizationFile := filepath.Join(kustomizationDir, "kustomization.yaml")
	kustomizationExists, err := afero.Exists(fs, kustomizationFile)
	if err != nil {
		return fmt.Errorf("failed to check kustomization.yaml: %w", err)
	}
	if !kustomizationExists {
		return fmt.Errorf("kustomization.yaml not found in: %s", kustomizationDir)
	}

	// Run kustomize
	opts := enablePlugins(krusty.MakeDefaultOptions())
	k := krusty.MakeKustomizer(opts)
	resources, err := k.Run(filesys.MakeFsOnDisk(), kustomizationDir)
	if err != nil {
		return fmt.Errorf("failed to run kustomize: %w", err)
	}

	// If no destination directory, print to stdout
	if destDir == "" {
		out, err := resources.AsYaml()
		if err != nil {
			return fmt.Errorf("failed to convert resources to YAML: %w", err)
		}
		fmt.Print(string(out))
		return nil
	}

	// Create destination directory if it doesn't exist
	if err := fs.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Split resources into individual files
	return splitResourcesFromResMap(fs, resources, destDir)
}

// splitResourcesFromResMap splits resources from resmap into individual files.
// This avoids double marshalling by working directly with the ResMap.
func splitResourcesFromResMap(fs afero.Fs, resources resmap.ResMap, destDir string) error {
	// Iterate over resources in the ResMap
	for _, resource := range resources.Resources() {
		// Extract kind and name from ResId (kind is already in CamelCase)
		kind := resource.GetKind()
		name := resource.GetName()

		// Marshal the resource to YAML
		yamlData, err := yaml.Marshal(resource)
		if err != nil {
			return fmt.Errorf("failed to marshal resource %s/%s: %w", kind, name, err)
		}

		// Create filename with CamelCase kind and underscore replacing colons
		safeName := strings.ReplaceAll(name, ":", "_")
		filename := fmt.Sprintf("%s-%s.yaml", kind, safeName)
		filepath := filepath.Join(destDir, filename)

		// Write file
		if err := afero.WriteFile(fs, filepath, yamlData, 0o644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", filepath, err)
		}

		fmt.Fprintf(os.Stderr, "Created: %s\n", filepath)
	}

	return nil
}

// splitResources splits YAML resources into individual files (deprecated, kept for backward compatibility).
func splitResources(fs afero.Fs, yamlData []byte, destDir string) error {
	// Split by "---" separator
	docs := strings.Split(string(yamlData), "\n---\n")

	for i, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" || doc == "---" {
			continue
		}

		// Parse the document to extract kind and name
		var resource map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &resource); err != nil {
			return fmt.Errorf("failed to parse resource %d: %w", i, err)
		}

		// Extract kind and name
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

		// Create filename with CamelCase kind and underscore replacing colons
		safeName := strings.ReplaceAll(name, ":", "_")
		filename := fmt.Sprintf("%s-%s.yaml", kind, safeName)
		filepath := filepath.Join(destDir, filename)

		// Write file
		if err := afero.WriteFile(fs, filepath, []byte(doc), 0o644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", filepath, err)
		}

		fmt.Fprintf(os.Stderr, "Created: %s\n", filepath)
	}

	return nil
}
