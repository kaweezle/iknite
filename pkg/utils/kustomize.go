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
package utils

import (
	flag "github.com/spf13/pflag"

	"github.com/kaweezle/iknite/pkg/cmd/options"
)

type KustomizeOptions struct {
	ForceConfig   bool
	ForceEmbedded bool
}

func NewKustomizeOptions() *KustomizeOptions {
	result := &KustomizeOptions{
		ForceConfig:   false,
		ForceEmbedded: false,
	}
	return result
}

func AddKustomizeOptionsFlags(flagSet *flag.FlagSet, kustomizeConfig *KustomizeOptions) {
	flagSet.BoolVarP(
		&kustomizeConfig.ForceConfig,
		options.ForceConfig,
		"C",
		kustomizeConfig.ForceConfig,
		"Force configuration even if it has already occurred",
	)
	flagSet.BoolVarP(
		&kustomizeConfig.ForceEmbedded,
		options.ForceEmbedded,
		"E",
		kustomizeConfig.ForceEmbedded,
		"Force use of embedded kustomization even if a custom one is available",
	)
}
