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

// cSpell: words crds

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/yaml"

	"github.com/kaweezle/iknite/pkg/kustomize"
)

// appType represents the type of an application directory.
type appType int

const (
	appTypeKustomize appType = iota
	appTypeHelmfile
	appTypeHelm
	appTypeUnknown
)

// argoApplication is a minimal ArgoCD Application struct for YAML parsing.
type argoApplication struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Source struct {
			Path string `yaml:"path"`
		} `yaml:"source"`
	} `yaml:"spec"`
}

// detectAppType auto-detects the application type from a directory.
// It returns the type, the resolved path (helmfile path or dir), and any error.
func detectAppType(fs afero.Fs, dir string) (appType, string, error) {
	kustomizationFile := filepath.Join(dir, "kustomization.yaml")
	exists, err := afero.Exists(fs, kustomizationFile)
	if err != nil {
		return appTypeUnknown, "", fmt.Errorf("failed to check kustomization.yaml: %w", err)
	}
	if exists {
		return appTypeKustomize, dir, nil
	}

	for _, name := range []string{"helmfile.yaml", "helmfile.yaml.gotmpl"} {
		helmfileFile := filepath.Join(dir, name)
		exists, err = afero.Exists(fs, helmfileFile)
		if err != nil {
			return appTypeUnknown, "", fmt.Errorf("failed to check %s: %w", name, err)
		}
		if exists {
			return appTypeHelmfile, helmfileFile, nil
		}
	}

	chartFile := filepath.Join(dir, "Chart.yaml")
	exists, err = afero.Exists(fs, chartFile)
	if err != nil {
		return appTypeUnknown, "", fmt.Errorf("failed to check Chart.yaml: %w", err)
	}
	if exists {
		return appTypeHelm, dir, nil
	}

	return appTypeUnknown, "", nil
}

func runCommandToResmap(cmd *exec.Cmd) (resmap.ResMap, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("command %s failed: %w\n%s", cmd.Path, err, stderr.String())
	}
	result, err := resmap.NewFactory(provider.NewDefaultDepProvider().GetResourceFactory()).
		NewResMapFromBytes(stdout.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to create resmap from %s output: %w", cmd.Path, err)
	}
	return result, nil
}

// renderHelmfile runs helmfile template and returns the YAML output.
func renderHelmfile(helmfileFile string) (resmap.ResMap, error) {
	return runCommandToResmap(
		exec.CommandContext(
			context.Background(),
			"helmfile",
			"template",
			"--skip-tests",
			"--args=--skip-crds",
			"-f",
			helmfileFile,
		),
	)
}

// renderHelm runs helm template and returns the YAML output.
func renderHelm(dir string) (resmap.ResMap, error) {
	releaseName := filepath.Base(dir)
	//nolint:gosec // controlled input for command and args, want to invoke helm CLI
	cmd := exec.CommandContext(context.Background(), "helm", "template", releaseName, dir, "--skip-crds")
	return runCommandToResmap(cmd)
}

// renderApp renders an application directory and returns the YAML output.
// For kustomize apps it uses Go code; for helmfile and helm it invokes the
// respective external commands.
func renderApp(fs afero.Fs, dir string) (resmap.ResMap, error) {
	appT, path, err := detectAppType(fs, dir)
	if err != nil {
		return nil, err
	}

	var resources resmap.ResMap
	var buildErr error
	switch appT {
	case appTypeKustomize:
		resources, buildErr = kustomize.Build(path)
	case appTypeHelmfile:
		resources, buildErr = renderHelmfile(path)
	case appTypeHelm:
		resources, buildErr = renderHelm(path)
	case appTypeUnknown:
		return nil, fmt.Errorf(
			//nolint:lll // long error message
			"directory %s has no recognized app definition (kustomization.yaml, helmfile.yaml, helmfile.yaml.gotmpl, or Chart.yaml)",
			dir,
		)
	}
	if buildErr != nil {
		return nil, buildErr
	}
	return resources, nil
}

// renderAppWithOutput renders an application and writes output to stdout or
// splits into destDir.
func renderAppWithOutput(fs afero.Fs, out io.Writer, dir, destDir string) error {
	resources, err := renderApp(fs, dir)
	if err != nil {
		return fmt.Errorf("while rendering with output: %w", err)
	}
	if destDir != "" {
		err = kustomize.SplitResMapToDir(fs, resources, destDir)
		if err != nil {
			return fmt.Errorf("failed to split resources to directory: %w", err)
		}
		return nil
	}
	var yamlData []byte
	yamlData, err = resources.AsYaml()
	if err != nil {
		return fmt.Errorf("failed to convert resources to YAML: %w", err)
	}
	_, err = out.Write(yamlData)
	if err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

// runKubeconform runs kubeconform validation on the provided YAML data.
// SchemasDir may be empty to skip the local schema location.
func runKubeconform(resources resmap.ResMap, schemasDir string) error {
	args := []string{"-schema-location", "default"}
	if schemasDir != "" {
		args = append(args, "-schema-location",
			schemasDir+"/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json")
	}
	args = append(
		args,
		"-schema-location",
		//nolint:lll // template
		"https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json",
		"-schema-location",
		"https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/master/customresourcedefinition.json",
		"-summary",
	)
	cmd := exec.CommandContext(context.Background(), "kubeconform", args...)
	data, err := resources.AsYaml()
	if err != nil {
		return fmt.Errorf("failed to convert resources to YAML: %w", err)
	}
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubeconform validation failed: %w", err)
	}
	return nil
}

// runValidateApp validates an application directory.
func runValidateApp(fs afero.Fs, _ io.Writer, dir, schemasDir string) error {
	exists, err := afero.DirExists(fs, dir)
	if err != nil {
		return fmt.Errorf("failed to check directory: %w", err)
	}
	if !exists {
		return fmt.Errorf("directory does not exist: %s", dir)
	}

	yamlData, err := renderApp(fs, dir)
	if err != nil {
		return err
	}
	return runKubeconform(yamlData, schemasDir)
}

// runRenderApp renders an application directory to stdout or a destination directory.
func runRenderApp(fs afero.Fs, out io.Writer, dir, destDir string) error {
	exists, err := afero.DirExists(fs, dir)
	if err != nil {
		return fmt.Errorf("failed to check directory: %w", err)
	}
	if !exists {
		return fmt.Errorf("directory does not exist: %s", dir)
	}
	return renderAppWithOutput(fs, out, dir, destDir)
}

// parseApplicationsFromDir parses Application resources from YAML files in a directory.
func parseApplicationsFromDir(fs afero.Fs, manifestsDir string) ([]argoApplication, error) {
	files, err := afero.ReadDir(fs, manifestsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", manifestsDir, err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	var apps []argoApplication
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}
		data, err := afero.ReadFile(fs, filepath.Join(manifestsDir, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", file.Name(), err)
		}
		var app argoApplication
		if unmarshalErr := yaml.Unmarshal(data, &app); unmarshalErr != nil {
			continue
		}
		if app.Kind == "Application" && app.Metadata.Name != "" {
			apps = append(apps, app)
		}
	}
	return apps, nil
}

// runRenderAll renders all appstages in an environment, mirroring render-environment.sh.
//
//nolint:gocyclo // complex function but straightforward flow
func runRenderAll(fs afero.Fs, out io.Writer, appstagesDir, destDir, baseDir string) error {
	exists, err := afero.DirExists(fs, appstagesDir)
	if err != nil {
		return fmt.Errorf("failed to check appstages directory: %w", err)
	}
	if !exists {
		return fmt.Errorf("appstages directory not found: %s", appstagesDir)
	}

	if err = fs.RemoveAll(destDir); err != nil {
		return fmt.Errorf("failed to remove destination directory: %w", err)
	}
	if err = fs.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	entries, err := afero.ReadDir(fs, appstagesDir)
	if err != nil {
		return fmt.Errorf("failed to read appstages directory: %w", err)
	}

	var appstageDirs []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "appstage-") {
			appstageDirs = append(appstageDirs, filepath.Join(appstagesDir, entry.Name()))
		}
	}
	sort.Strings(appstageDirs)

	if len(appstageDirs) == 0 {
		return fmt.Errorf("no appstage- prefixed directories found in %s", appstagesDir)
	}

	for _, appstageDir := range appstageDirs {
		appstageName := filepath.Base(appstageDir)
		manifestsDir := filepath.Join(destDir, appstageName, "manifests")
		applicationsDir := filepath.Join(destDir, appstageName, "applications")

		fmt.Fprintf(out, "Rendering appstage %s\n", appstageName) //nolint:errcheck // best effort logging

		resources, runErr := kustomize.Build(appstageDir)
		if runErr != nil {
			return fmt.Errorf("failed to render appstage %s: %w", appstageName, runErr)
		}
		if err = kustomize.SplitResMapToDir(fs, resources, manifestsDir); err != nil {
			return fmt.Errorf("failed to write manifests for appstage %s: %w", appstageName, err)
		}

		apps, err := parseApplicationsFromDir(fs, manifestsDir)
		if err != nil {
			return fmt.Errorf("failed to parse applications for appstage %s: %w", appstageName, err)
		}

		for _, app := range apps {
			if app.Spec.Source.Path == "" {
				return fmt.Errorf("application %s in appstage %s has no spec.source.path",
					app.Metadata.Name, appstageName)
			}
			appSourceDir := filepath.Join(baseDir, app.Spec.Source.Path)
			appDestDir := filepath.Join(applicationsDir, app.Metadata.Name)

			//nolint:errcheck // best effort logging
			fmt.Fprintf(out, "Rendering application %s from %s\n", app.Metadata.Name, app.Spec.Source.Path)

			if renderErr := renderAppWithOutput(fs, out, appSourceDir, appDestDir); renderErr != nil {
				return fmt.Errorf("failed to render application %s: %w", app.Metadata.Name, renderErr)
			}
		}
	}

	return nil
}

// CreateApplicationCmd creates the application command with validate, render, and render-all subcommands.
func CreateApplicationCmd(fs afero.Fs, out io.Writer) *cobra.Command {
	appCmd := &cobra.Command{
		Use:   "application",
		Short: "Manage ArgoCD applications",
		Long: `Commands for validating and rendering ArgoCD applications.

The application type is auto-detected from the directory contents:
  - kustomization.yaml   → kustomize (uses Go code)
  - helmfile.yaml(.gotmpl) → helmfile (invokes the helmfile command)
  - Chart.yaml           → helm chart (invokes the helm command)`,
	}

	var schemasDir string
	validateCmd := &cobra.Command{
		Use:   "validate <directory>",
		Short: "Validate an application",
		Long: `Validate an application directory using kubeconform.

The application type is auto-detected. Kustomize apps are built with Go code;
helmfile and helm apps invoke the respective external commands. The output is
then validated with kubeconform.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runValidateApp(fs, out, args[0], schemasDir)
		},
	}
	validateCmd.Flags().StringVar(&schemasDir, "schemas-dir", "",
		"directory containing additional kubeconform schemas")
	appCmd.AddCommand(validateCmd)

	var renderDestDir string
	renderCmd := &cobra.Command{
		Use:   "render <directory>",
		Short: "Render an application",
		Long: `Render an application directory.

The application type is auto-detected. Kustomize apps are built with Go code;
helmfile and helm apps invoke the respective external commands.
With --output, resources are split into individual <Kind>-<name>.yaml files.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runRenderApp(fs, out, args[0], renderDestDir)
		},
	}
	renderCmd.Flags().StringVarP(&renderDestDir, "output", "o", "",
		"output directory for split resources (default: stdout)")
	appCmd.AddCommand(renderCmd)

	var baseDir string
	renderAllCmd := &cobra.Command{
		Use:   "render-all <appstages-dir> <destination-dir>",
		Short: "Render all appstages and their applications",
		Long: `Render all appstages in an environment, mirroring render-environment.sh.

Processes each appstage-* directory found in <appstages-dir>, renders its
kustomization manifests, then renders each referenced ArgoCD Application to
<destination-dir>/<appstage>/applications/<app-name>/.`,
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			return runRenderAll(fs, out, args[0], args[1], baseDir)
		},
	}
	renderAllCmd.Flags().StringVar(&baseDir, "base-dir", ".",
		"repository root directory for resolving application source paths")
	appCmd.AddCommand(renderAllCmd)

	return appCmd
}
