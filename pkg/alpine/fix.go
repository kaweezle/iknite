package alpine

import (
	"github.com/bitfield/script"
	log "github.com/sirupsen/logrus"
)

const kubeletConfigurationFilename = "/etc/conf.d/kubelet"

func FixKubeletConfiguration() (err error) {

	log.WithField("file", kubeletConfigurationFilename).Debug("Fixing kubelet configuration bug")

	var content string
	content, err = script.File(kubeletConfigurationFilename).
		Replace("--cni-bin-dir=/usr/libexec/cni ", "").
		String()
	if err != nil {
		return
	}

	log.WithFields(log.Fields{
		"file":    kubeletConfigurationFilename,
		"content": content,
	}).Debug("Writing fixed file")

	_, err = script.Echo(content).
		WriteFile(kubeletConfigurationFilename)
	return
}
