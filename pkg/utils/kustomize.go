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
	"fmt"

	flag "github.com/spf13/pflag"

	"github.com/kaweezle/iknite/pkg/cmd/options"
	"github.com/kaweezle/iknite/pkg/constants"
)

type KustomizeOptions struct {
	Kustomization string
	ForceConfig   bool
	ForceEmbedded bool
}

func NewKustomizeOptions() *KustomizeOptions {
	result := &KustomizeOptions{
		Kustomization: constants.DefaultKustomization,
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

	existing := flagSet.Lookup(options.Kustomization)
	if existing == nil {
		flagSet.StringVar(
			&kustomizeConfig.Kustomization,
			options.Kustomization,
			kustomizeConfig.Kustomization,
			"Kustomization location (URL or directory)",
		)
	} else {
		AddStringFlagDestination(existing, &kustomizeConfig.Kustomization)
	}
}

type MultiStringValue struct {
	values []flag.Value
}

// Set implements [pflag.Value].
func (k *MultiStringValue) Set(value string) error {
	for _, v := range k.values {
		err := v.Set(value)
		if err != nil {
			return fmt.Errorf("while setting a multi string value: %w", err)
		}
	}
	return nil
}

// String implements [pflag.Value].
func (k *MultiStringValue) String() string {
	if len(k.values) == 0 {
		return ""
	}
	return k.values[0].String()
}

// Type implements [pflag.Value].
func (k *MultiStringValue) Type() string {
	return "string"
}

var _ flag.Value = (*MultiStringValue)(nil)

func NewMultiStringValue(values ...flag.Value) *MultiStringValue {
	return &MultiStringValue{
		values: values,
	}
}

func AddStringFlagDestination(existing *flag.Flag, destination *string) {
	ev := existing.Value
	fs := flag.NewFlagSet("temp", flag.ContinueOnError)
	fs.StringVar(
		destination,
		existing.Name,
		existing.Value.String(),
		existing.Usage,
	)
	nv := fs.Lookup(options.Kustomization)
	existing.Value = NewMultiStringValue(ev, nv.Value)
}
