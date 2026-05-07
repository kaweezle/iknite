package cmd

// cSpell: words configurer

import (
	"io"
	"os"

	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/checkers"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/utils"
)

// RootDeps groups dependencies used to build the iknite root command tree.
type RootDeps struct {
	Host                   host.Host
	FileSystem             host.FileSystem
	Stdin                  io.Reader
	Stdout                 io.Writer
	CleanOptions           *cleanOptions
	InitOptions            *initOptions
	InitRunner             *workflow.Runner
	KubeletKustomizeOption *utils.KustomizeOptions
	KustomizeOptions       *utils.KustomizeOptions
	KustomizeWaitOptions   *utils.WaitOptions
	ResetOptions           *resetOptions
	ResetRunner            *workflow.Runner
	StartWaitOptions       *utils.WaitOptions
	StatusConfigurer       CheckExecutorConfigurer
	StatusWaitOptions      *utils.WaitOptions
}

// DefaultRootDeps returns production defaults for root command wiring.
func DefaultRootDeps() *RootDeps {
	statusWaitOptions := utils.NewWaitOptions()
	statusWaitOptions.OkResponses = 3

	kustomizeWaitOptions := utils.NewWaitOptions()
	kustomizeWaitOptions.Immediate = false
	kustomizeWaitOptions.Wait = false

	return &RootDeps{
		Host:                   host.NewDefaultHost(),
		FileSystem:             host.NewOsFS(),
		Stdin:                  os.Stdin,
		Stdout:                 os.Stdout,
		CleanOptions:           newCleanOptions(),
		InitOptions:            newInitOptions(),
		InitRunner:             workflow.NewRunner(),
		KubeletKustomizeOption: utils.NewKustomizeOptions(),
		KustomizeOptions:       utils.NewKustomizeOptions(),
		KustomizeWaitOptions:   kustomizeWaitOptions,
		ResetOptions:           newResetOptions(),
		ResetRunner:            workflow.NewRunner(),
		StartWaitOptions:       utils.NewWaitOptions(),
		StatusConfigurer:       CheckExecutorConfigFunc(checkers.ConfigureIkniteClusterChecker),
		StatusWaitOptions:      statusWaitOptions,
	}
}

//nolint:gocyclo // straightforward field-by-field defaulting
func applyRootDepsDefaults(deps *RootDeps) *RootDeps {
	defaults := DefaultRootDeps()
	if deps == nil {
		return defaults
	}
	if deps.Host == nil {
		deps.Host = defaults.Host
	}
	if deps.FileSystem == nil {
		deps.FileSystem = defaults.FileSystem
	}
	if deps.Stdin == nil {
		deps.Stdin = defaults.Stdin
	}
	if deps.Stdout == nil {
		deps.Stdout = defaults.Stdout
	}
	if deps.CleanOptions == nil {
		deps.CleanOptions = defaults.CleanOptions
	}
	if deps.InitOptions == nil {
		deps.InitOptions = defaults.InitOptions
	}
	if deps.InitRunner == nil {
		deps.InitRunner = defaults.InitRunner
	}
	if deps.KubeletKustomizeOption == nil {
		deps.KubeletKustomizeOption = defaults.KubeletKustomizeOption
	}
	if deps.KustomizeOptions == nil {
		deps.KustomizeOptions = defaults.KustomizeOptions
	}
	if deps.KustomizeWaitOptions == nil {
		deps.KustomizeWaitOptions = defaults.KustomizeWaitOptions
	}
	if deps.ResetOptions == nil {
		deps.ResetOptions = defaults.ResetOptions
	}
	if deps.ResetRunner == nil {
		deps.ResetRunner = defaults.ResetRunner
	}
	if deps.StartWaitOptions == nil {
		deps.StartWaitOptions = defaults.StartWaitOptions
	}
	if deps.StatusConfigurer == nil {
		deps.StatusConfigurer = defaults.StatusConfigurer
	}
	if deps.StatusWaitOptions == nil {
		deps.StatusWaitOptions = defaults.StatusWaitOptions
	}
	return deps
}
