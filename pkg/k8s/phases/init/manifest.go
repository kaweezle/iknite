package init

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/afero"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
)

// writeStaticPodManifest creates filename in manifestDir and writes the rendered manifest to it.
// If rendering fails, the partially-written file is removed.
func writeStaticPodManifest(
	fs afero.Fs,
	manifestDir, filename string,
	config *v1alpha1.IkniteClusterSpec,
	render func(io.Writer, *v1alpha1.IkniteClusterSpec) error,
) (afero.File, error) {
	afs := &afero.Afero{Fs: fs}
	if err := afs.MkdirAll(manifestDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create manifest directory %s: %w", manifestDir, err)
	}
	f, err := afs.Create(filepath.Join(manifestDir, filename))
	if err != nil {
		return f, fmt.Errorf("failed to create %s: %w", filename, err)
	}
	defer func() {
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		} else if closeErr == nil {
			closeErr = afs.Remove(f.Name())
			if closeErr != nil {
				err = errors.Join(err, fmt.Errorf("while removing file %s: %w", f.Name(), closeErr))
			}
		}
	}()

	err = render(f, config)
	return f, err
}
