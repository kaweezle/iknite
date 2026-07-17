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

// cSpell: words crds appsvc

import (
	"context"

	"github.com/spf13/cobra"
	"go.uber.org/dig"

	"github.com/kaweezle/iknite/pkg/cmd/types"
	appsvc "github.com/kaweezle/iknite/pkg/iknitectl/application"
)

// CreateApplicationCmd creates the application command with validate, render, and render-all subcommands.
func CreateApplicationCmd(s *dig.Scope) *cobra.Command {
	cobra.CheckErr(s.Provide(appsvc.NewService))

	appCmd := &cobra.Command{
		Use:     "application",
		Aliases: []string{"app", "a"},
		Short:   "Manage ArgoCD applications",
		Long: `Commands for validating and rendering ArgoCD applications.

The application type is auto-detected from the directory contents:
  - kustomization.yaml     → kustomize (uses Go code)
  - helmfile.yaml(.gotmpl) → helmfile (invokes the helmfile command)
  - Chart.yaml             → helm chart (invokes the helm command)`,
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
			return s.Invoke(func(svc *appsvc.Service, ctx context.Context) error {
				return svc.Validate(ctx, args[0], schemasDir)
			})
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
			return s.Invoke(func(svc *appsvc.Service, ctx context.Context, out types.CmdOut) error {
				return svc.Render(ctx, args[0], renderDestDir, out)
			})
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
			return s.Invoke(func(svc *appsvc.Service, ctx context.Context, out types.CmdOut) error {
				return svc.RenderAll(ctx, args[0], args[1], baseDir, out)
			})
		},
	}
	renderAllCmd.Flags().StringVar(&baseDir, "base-dir", ".",
		"repository root directory for resolving application source paths")
	appCmd.AddCommand(renderAllCmd)

	return appCmd
}
