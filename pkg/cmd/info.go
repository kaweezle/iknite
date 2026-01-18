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
package cmd

// cSpell: words forbidigo

// cSpell: disable
import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/cmd/options"
	"github.com/kaweezle/iknite/pkg/config"
)

// cSpell: enable

func NewInfoCmd(ikniteConfig *v1alpha1.IkniteClusterSpec) *cobra.Command {
	// infoCmd represents the start command
	infoCmd := &cobra.Command{
		Use:   "info",
		Short: "Creates or starts the cluster",
		Long: `Starts the cluster. Performs the following operations:

- Starts OpenRC,
- Starts containerd,
- If Kubelet has never been started, execute kubeadm init to provision
  the cluster,
- Allows the use of kubectl from the root account,
- Installs flannel, metal-lb and local-path-provisioner.
`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			config.StartPersistentPreRun(cmd, args)
			flags := cmd.Flags()
			_ = viper.BindPFlag( //nolint:errcheck // flag exists
				options.OutputFormat,
				flags.Lookup(options.OutputFormat))
			_ = viper.BindPFlag( //nolint:errcheck // flag exists
				options.OutputDestination,
				flags.Lookup(options.OutputDestination))
		},
		Run: func(_ *cobra.Command, _ []string) { performInfo(ikniteConfig) },
	}
	flags := infoCmd.PersistentFlags()
	config.ConfigureClusterCommand(flags, ikniteConfig)

	flags = infoCmd.Flags()
	flags.StringP(
		options.OutputFormat,
		"o",
		"yaml",
		"Output format. One of: yaml|json",
	)
	flags.String(
		options.OutputDestination,
		"",
		"Output destination file. If not set, output to stdout.",
	)

	infoCmd.AddCommand(NewImagesCmd(ikniteConfig))

	return infoCmd
}

func NewImagesCmd(ikniteConfig *v1alpha1.IkniteClusterSpec) *cobra.Command {
	imagesCmd := &cobra.Command{
		Use:   "images",
		Short: "Lists the container images used by iknite",
		Long:  `Lists the container images used by iknite.`,
		Run: func(_ *cobra.Command, _ []string) {
			performImages(ikniteConfig)
		},
	}
	return imagesCmd
}

func performInfo(ikniteConfig *v1alpha1.IkniteClusterSpec) {
	cobra.CheckErr(config.DecodeIkniteConfig(ikniteConfig))
	// Marshal config into YAML and print it to the output
	outputFormat := viper.GetString(options.OutputFormat)
	outputDestination := viper.GetString(options.OutputDestination)

	// Determine the writer based on output destination
	writer := os.Stdout
	if outputDestination != "" && outputDestination != "stdout" {
		file, err := os.Create(filepath.Clean(outputDestination))
		cobra.CheckErr(err)
		defer file.Close() //nolint:errcheck // best effort
		writer = file
	}

	cobra.CheckErr(config.PrintIkniteConfig(writer, ikniteConfig, outputFormat))
}

func performImages(ikniteConfig *v1alpha1.IkniteClusterSpec) {
	cobra.CheckErr(config.DecodeIkniteConfig(ikniteConfig))

	containerImages, err := config.GetIkniteImages(ikniteConfig)
	cobra.CheckErr(err)

	for _, image := range containerImages {
		fmt.Println(image) //nolint:forbidigo // printing is expected here
	}
}
