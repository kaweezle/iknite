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
package crio

import (
	"encoding/json"
	"os/exec"
	"time"

	"github.com/antoinemartin/kaweezle-rootfs/pkg/constants"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

type CRIOCondition struct {
	Type    string `json:"type"`
	Status  bool   `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type CRIOStatus struct {
	Conditions []CRIOCondition `json:"conditions"`
}

type CRIOStatusResponse struct {
	Status CRIOStatus `json:"status"`
}

var fs = afero.NewOsFs()
var afs = &afero.Afero{Fs: fs}

func WaitForCrio() (bool, error) {
	retries := 3
	for retries > 0 {
		exist, err := afs.Exists(constants.CrioSock)
		if err != nil {
			return false, errors.Wrapf(err, "Error while checking crio sock %s", constants.CrioSock)
		}
		if exist {
			out, err := exec.Command("/usr/bin/crictl", "--runtime-endpoint", "unix:///run/crio/crio.sock", "info").Output()
			if err == nil {
				log.Trace(string(out))
				response := &CRIOStatusResponse{}
				err = json.Unmarshal(out, &response)
				if err == nil {
					conditions := 0
					falseConditions := 0
					for _, v := range response.Status.Conditions {
						conditions += 1
						if !v.Status {
							falseConditions += 1
						}
					}
					if conditions >= 2 && falseConditions == 0 {
						break
					}
				} else {
					log.WithError(err).Warn("Error while parsing crio status")
				}
			} else {
				log.WithError(err).Warn("Error while checking crio sock")
			}
		}
		retries = retries - 1

		log.Debug("Waiting 2 seconds...")
		time.Sleep(2 * time.Second)
	}
	return retries > 0, nil
}
