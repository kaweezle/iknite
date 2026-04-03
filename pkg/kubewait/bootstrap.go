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

// Package kubewait implements the kubewait command.
// It waits for Kubernetes resources in specified namespaces to become ready
// using kstatus (one goroutine per namespace), then optionally clones and runs
// a bootstrap repository script.
package kubewait

// cSpell: words godotenv clientcmd apimachinery kstatus errorf sirupsen joho metav1

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	defaultBootstrapScript = "iknite-bootstrap.sh"
	defaultBootstrapDir    = "/workspace"
	defaultEnvFile         = ".env"
	bootstrapRepoDirname   = "bootstrap-repo"
)

// Options holds the configuration for the kubewait command.
type BootstrapOptions struct {
	BootstrapDir    string
	BootstrapScript string
	RepoURL         string
	RepoRef         string
	EnvFile         string
}

func NewBootstrapOptions() BootstrapOptions {
	return BootstrapOptions{
		BootstrapDir:    defaultBootstrapDir,
		BootstrapScript: defaultBootstrapScript,
	}
}

func AddBootstrapFlags(flags *pflag.FlagSet, opts *BootstrapOptions) {
	flags.StringVar(&opts.BootstrapDir, "bootstrap-dir", defaultBootstrapDir,
		"Directory to clone the bootstrap repository into (also used as the script working directory)")
	flags.StringVar(&opts.BootstrapScript, "bootstrap-script", defaultBootstrapScript,
		"Name of the bootstrap script to run inside --bootstrap-dir")
	flags.StringVar(&opts.RepoURL, "bootstrap-repo-url", "",
		"URL of the bootstrap git repository to clone (requires --bootstrap-repo-ref)")
	flags.StringVar(&opts.RepoRef, "bootstrap-repo-ref", "",
		"Git branch or tag to checkout when cloning the bootstrap repository")
	flags.StringVar(&opts.EnvFile, "env-file", "",
		"Path to an env file to load before running the bootstrap script (default: .env inside --bootstrap-dir)")
}

func (opts *BootstrapOptions) ReadEnvFile() (bool, error) {
	// Determine the env file path.
	envFile := opts.EnvFile
	if envFile == "" {
		envFile = filepath.Join(opts.BootstrapDir, defaultEnvFile)
	}

	if info, err := os.Stat(envFile); err == nil && !info.IsDir() {
		log.Infof("Loading environment from %s", envFile)
		if loadErr := godotenv.Load(envFile); loadErr != nil {
			return false, fmt.Errorf("failed to load env file %s: %w", envFile, loadErr)
		}
	}

	return true, nil
}

// runBootstrap clones the bootstrap repository (if URL and ref are provided), loads the env
// file (if present), and executes the bootstrap script.
func runBootstrap(ctx context.Context, opts *Options) error {
	// Clone the repository when a ref is also supplied; if no ref is given the clone is skipped.

	baseDir := opts.BootstrapDir
	if opts.RepoURL != "" && opts.RepoRef != "" {
		if err := cloneBootstrapRepo(ctx, opts); err != nil {
			return fmt.Errorf("error during bootstrap: %w", err)
		}
		baseDir = filepath.Join(opts.BootstrapDir, bootstrapRepoDirname)
	} else {
		log.Info("Bootstrap repo URL or ref not provided, skipping clone")
	}

	// Locate and execute the bootstrap script.
	scriptPath := filepath.Join(baseDir, opts.BootstrapScript)
	if _, err := os.Stat(scriptPath); err != nil {
		log.Infof(
			"Bootstrap script %s not found in %s with error %v, skipping",
			opts.BootstrapScript,
			baseDir,
			err,
		)
		return nil
	}

	//nolint:gosec // ensure executable, matching bootstrap.sh chmod +x
	if err := os.Chmod(scriptPath, 0o755); err != nil {
		return fmt.Errorf("failed to make bootstrap script executable: %w", err)
	}

	log.Infof("Running bootstrap script: %s", scriptPath)
	//nolint:gosec // scriptPath is controlled by the user via --bootstrap-dir / --bootstrap-script flags
	cmd := exec.CommandContext(ctx, scriptPath)
	cmd.Dir = opts.BootstrapDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bootstrap script %s failed: %w", scriptPath, err)
	}

	return nil
}

// cloneBootstrapRepo performs a shallow git clone of the bootstrap repository.
func cloneBootstrapRepo(ctx context.Context, opts *Options) error {
	repoPath := filepath.Join(opts.BootstrapDir, bootstrapRepoDirname)
	log.Infof("Cloning bootstrap repository %s (ref: %s) to %s", opts.RepoURL, opts.RepoRef, repoPath)

	if err := ensureSSHKnownHost(ctx, opts.RepoURL); err != nil {
		return err
	}

	// Remove any existing target directory so the clone is clean.
	if err := os.RemoveAll(repoPath); err != nil {
		return fmt.Errorf("failed to remove existing bootstrap dir %s: %w", repoPath, err)
	}

	//nolint:gosec // arguments come from CLI flags under user control
	cmd := exec.CommandContext(
		ctx,
		"git",
		"clone",
		"--depth",
		"1",
		"--branch",
		opts.RepoRef,
		opts.RepoURL,
		repoPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone of %s failed: %w", opts.RepoURL, err)
	}

	return nil
}

func ensureSSHKnownHost(ctx context.Context, repoURL string) error {
	sshServer := extractDomain(repoURL)
	if sshServer == "" {
		log.Warnf(
			"Failed to extract SSH server from repo URL %s, SSH host key will not be added to known_hosts",
			repoURL,
		)
		return nil
	}

	log.Infof("Waiting for SSH server %s to be resolvable...", sshServer)
	if err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		addrs, err := net.DefaultResolver.LookupHost(ctx, sshServer)
		if err != nil {
			log.WithError(err).Debugf("SSH server %s not yet resolvable, retrying...", sshServer)
			return false, nil
		}
		if len(addrs) == 0 {
			log.Debugf("SSH server %s resolved without addresses, retrying...", sshServer)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return fmt.Errorf("SSH server %s did not become resolvable: %w", sshServer, err)
	}

	log.Infof("Adding SSH server %s to known_hosts", sshServer)
	//nolint:gosec // sshServer is derived from repoURL and only used for ssh-keyscan host lookup.
	sshKeyscanCmd := exec.CommandContext(ctx, "ssh-keyscan", "-t", "rsa", sshServer)
	keyscanOutput, err := sshKeyscanCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to scan SSH key for %s: %w", sshServer, err)
	}

	// Ensure $HOME/.ssh directory exists so that ssh-keyscan can write to known_hosts without permission issues.
	sshDir := filepath.Join(os.Getenv("HOME"), ".ssh")

	//nolint:gosec // path is constrained to user-local HOME and mode is intentionally restrictive.
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	knownHostsPath := filepath.Join(sshDir, "known_hosts")

	//nolint:gosec // path is constrained to user-local HOME and mode is intentionally restrictive.
	if err := os.WriteFile(knownHostsPath, keyscanOutput, 0o600); err != nil {
		return fmt.Errorf("failed to write known_hosts file: %w", err)
	}

	return nil
}

func extractDomain(repoURL string) string {
	// This is a very naive extraction that assumes the repo URL is in the form "git@domain:owner/repo.git".
	// In a real implementation, you would want to handle more cases and validate the input properly.
	parts := strings.Split(repoURL, "@")
	if len(parts) != 2 {
		return ""
	}
	domainParts := strings.Split(parts[1], ":")
	if len(domainParts) != 2 {
		return ""
	}
	return domainParts[0]
}
