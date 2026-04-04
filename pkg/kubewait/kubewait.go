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

// cSpell: words kstatus sirupsen logrus
// Package kubewait implements the kubewait command.
// It waits for Kubernetes resources in specified namespaces to become ready
// using kstatus (one goroutine per namespace), then optionally clones and runs
// a bootstrap repository script.
package kubewait

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"

	"github.com/kaweezle/iknite/pkg/cmd/util"
)

type Options struct {
	Verbosity               log.Level
	JSONLogs                bool
	SkipWaitingForResources bool
	SkipBootstrap           bool
	ResourcesOptions
	BootstrapOptions
}

func NewOptions() *Options {
	opts := &Options{
		ResourcesOptions: NewResourcesOptions(),
		BootstrapOptions: NewBootstrapOptions(),
		Verbosity:        log.InfoLevel,
	}
	return opts
}

func AddKubewaitFlags(flags *pflag.FlagSet, opts *Options) {
	AddResourcesFlags(flags, &opts.ResourcesOptions)
	AddBootstrapFlags(flags, &opts.BootstrapOptions)
	flags.VarP(
		util.NewLogLevelValue(&opts.Verbosity),
		"verbosity",
		"v",
		"Log level (debug, info, warn, error, fatal, panic)",
	)
	flags.BoolVar(&opts.JSONLogs, "json", false, "Emit log messages as JSON")
	flags.BoolVar(
		&opts.SkipWaitingForResources,
		"skip-wait",
		false,
		"Skip waiting for resources to be ready and proceed directly to the optional bootstrap (for testing purposes)",
	)
	flags.BoolVar(
		&opts.SkipBootstrap,
		"skip-bootstrap",
		false,
		"Skip the bootstrap process (for testing purposes)",
	)
}

// RunKubewait is the main logic for the kubewait command.
func RunKubewait(ctx context.Context, opts *Options, namespaces []string) error {
	if !opts.SkipWaitingForResources {
		if err := waitForResources(ctx, opts, namespaces); err != nil {
			return fmt.Errorf("error while waiting for resources: %w", err)
		}
	}

	if !opts.SkipBootstrap {
		if err := runBootstrap(ctx, opts); err != nil {
			return fmt.Errorf("error during bootstrap: %w", err)
		}
	}
	return nil
}
