package provision

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	c "github.com/antoinemartin/k8wsl/pkg/constants"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

var fs = afero.NewOsFs()
var afs = &afero.Afero{Fs: fs}

func deleteTempDir(d string) error {
	return os.RemoveAll(d)
}

func createTempKustomizeDirectory(content *embed.FS, tmpdir string, dirname string, data interface{}) error {
	log.WithFields(log.Fields{
		"tmpdir":  tmpdir,
		"dirname": dirname,
		"data":    data,
	}).Trace("Start creating directory")

	files, err := content.ReadDir(dirname)
	if err != nil {
		return errors.Wrapf(err, "While reading files of %s", dirname)
	}
	for _, entry := range files {
		if !entry.IsDir() {
			inPath := fmt.Sprintf("%s/%s", dirname, entry.Name())
			outPath := fmt.Sprintf("%s/%s", tmpdir, entry.Name())

			log.WithField("path", inPath).Trace("Reading file")
			payload, err := content.ReadFile(inPath)
			if err != nil {
				return errors.Wrapf(err, "While reading embedded file %s", entry.Name())
			}

			if filepath.Ext(entry.Name()) == ".tmpl" {
				log.WithField("path", inPath).Trace("Is template")
				t, err := template.New("tmp").Parse(string(payload))
				if err != nil {
					return errors.Wrapf(err, "While reading template %s", entry.Name())
				}
				buf := new(bytes.Buffer)
				log.WithField("path", inPath).
					WithField("data", data).
					Trace("Rendering")
				err = t.Execute(buf, data)
				if err != nil {
					return errors.Wrap(err, "failed to create a manifest file")
				}
				payload = buf.Bytes()
				outPath = strings.TrimSuffix(outPath, ".tmpl")
			}
			log.WithField("outPath", outPath).Trace("Writing content")
			err = afs.WriteFile(outPath, payload, 0644)
			if err != nil {
				return errors.Wrapf(err, "While writing %s to temp dir %s", entry.Name(), tmpdir)
			}
		}
	}
	return nil
}

func ApplyKustomizations(content *embed.FS, dirname string, data interface{}) error {
	tmpdir, err := afs.TempDir("", dirname)
	if err != nil {
		return errors.Wrap(err, "While creating temp directory")
	}
	log.WithField("tmpdir", tmpdir).Trace("Temp dir created")
	defer deleteTempDir(dirname)

	err = createTempKustomizeDirectory(content, tmpdir, dirname, data)
	if err != nil {
		return err
	}

	log.WithField("tmpdir", tmpdir).Trace("Applying kustomization")
	out, err := exec.Command(c.KubectlCmd, "apply", "-k", tmpdir).CombinedOutput()
	log.WithField("data", data).Trace(string(out))
	if err != nil {
		return errors.Wrap(err, "While applying templates")
	}
	return nil
}
