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
package iknitectl

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/kustomize"
)

// CreateKustomizeCmd creates the kustomize command.
func CreateKustomizeCmd(fs afero.Fs, out io.Writer) *cobra.Command {
	kustomizeCmd := &cobra.Command{
		Use:   "kustomize <directory> [destination]",
		Short: "Run kustomize on a directory",
		Long: `Run kustomize on a directory with plugins and exec enabled.

If a destination directory is provided, the produced resources will be split
into individual files named <kind>-<name>.yaml.

Examples:
  # Print kustomization to stdout
  iknitectl kustomize /path/to/kustomization

  # Split resources into individual files
  iknitectl kustomize /path/to/kustomization /path/to/output`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			return runKustomize(fs, out, args)
		},
	}

	return kustomizeCmd
}

// runKustomize validates the kustomization directory and delegates to the
// kustomize package: it either writes all resources to out or splits them
// into individual files under destDir.
func runKustomize(fs afero.Fs, out io.Writer, args []string) error {
	kustomizationDir := args[0]
	var destDir string
	if len(args) > 1 {
		destDir = args[1]
	}

	exists, err := afero.DirExists(fs, kustomizationDir)
	if err != nil {
		return fmt.Errorf("failed to check kustomization directory: %w", err)
	}
	if !exists {
		return fmt.Errorf("kustomization directory does not exist: %s", kustomizationDir)
	}

	kustomizationFile := filepath.Join(kustomizationDir, "kustomization.yaml")
	kustomizationExists, err := afero.Exists(fs, kustomizationFile)
	if err != nil {
		return fmt.Errorf("failed to check kustomization.yaml: %w", err)
	}
	if !kustomizationExists {
		return fmt.Errorf("kustomization.yaml not found in: %s", kustomizationDir)
	}

	resources, err := kustomize.Build(kustomizationDir)
	if err != nil {
		return fmt.Errorf("while building kustomization in %s kustomize: %w", kustomizationDir, err)
	}

	if destDir == "" {
		err = kustomize.WriteToWriter(resources, out)
		if err != nil {
			return fmt.Errorf("failed to write kustomization output: %w", err)
		}
		return nil
	}
	err = kustomize.SplitResMapToDir(fs, resources, destDir)
	if err != nil {
		return fmt.Errorf("failed to split kustomization output to directory: %w", err)
	}
	return nil
}
