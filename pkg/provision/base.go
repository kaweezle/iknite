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
// cSpell: words filesys kyaml wrapcheck
package provision

import (
	"embed"
	"fmt"
	"log/slog"
	"net/url"
	"path"

	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/kustomize"
)

//go:embed base
var content embed.FS

func createTempKustomizeDirectory(
	content *embed.FS,
	fs filesys.FileSystem,
	outDir, inDir string,
	logger *slog.Logger,
) error {
	logger.Debug("Start creating directory", "outDir", outDir, "inDir", inDir)

	files, err := content.ReadDir(inDir)
	if err != nil {
		return fmt.Errorf("while reading files of %s: %w", inDir, err)
	}
	for _, entry := range files {
		if entry.IsDir() {
			continue
		}

		inPath := fmt.Sprintf("%s/%s", inDir, entry.Name())
		outPath := fmt.Sprintf("%s/%s", outDir, entry.Name())

		logger.Debug("Reading file", "path", inPath)
		payload, err := content.ReadFile(inPath)
		if err != nil {
			return fmt.Errorf("while reading embedded file %s: %w", entry.Name(), err)
		}

		logger.Debug("Writing content", "outPath", outPath)
		err = fs.WriteFile(outPath, payload)
		if err != nil {
			return fmt.Errorf("while writing %s to temp dir %s: %w", entry.Name(), outDir, err)
		}
	}
	return nil
}

// isBaseKustomizationAvailable checks if a kustomization.yaml file is available
// in the specified directory or if the directory is a URL.
func isBaseKustomizationAvailable(fs host.FileSystem, dirname string) (bool, error) {
	var exists bool
	kustomizationURl, err := url.Parse(dirname)
	if err == nil && kustomizationURl.Scheme != "" && kustomizationURl.Host != "" {
		exists = true
	} else {
		exists, err = fs.Exists(path.Join(dirname, "kustomization.yaml"))
		if err != nil {
			return false, fmt.Errorf("while testing for directory: %w", err)
		}
	}
	return exists, nil
}

// GetBaseKustomizationResources applies the kustomizations located in the specified
// directory if available, otherwise returns the embedded kustomizations.
func GetBaseKustomizationResources(
	fs host.FileSystem,
	dirname string,
	forceEmbedded bool,
	logger *slog.Logger,
) (resmap.ResMap, error) {
	ok, err := isBaseKustomizationAvailable(fs, dirname)
	if err != nil {
		return nil, fmt.Errorf("while checking for base kustomization: %w", err)
	}
	kustomizeFs := host.NewKustomizeFSWrapper(fs)
	if !ok || forceEmbedded {
		logger.Debug("Using embedded kustomization.", "directory", dirname, "force_embedded", forceEmbedded,
			"exists", ok)
		kustomizeFs = filesys.MakeFsInMemory()
		dirname = "base"
		err = createTempKustomizeDirectory(&content, kustomizeFs, dirname, dirname, logger)
		if err != nil {
			return nil, fmt.Errorf("while creating temporary kustomization directory: %w", err)
		}
	} else {
		logger.Debug("Base kustomization found, applying it...", "directory", dirname)
	}
	return kustomize.BuildOnFileSystem(kustomizeFs, dirname) //nolint:wrapcheck // No need to wrap here.
}
