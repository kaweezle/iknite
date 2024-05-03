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
package cri

import (
	"encoding/json"
	"time"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

type CRICondition struct {
	Type    string `json:"type"`
	Status  bool   `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type CRIStatus struct {
	Conditions []CRICondition `json:"conditions"`
}

type CRIStatusResponse struct {
	Status CRIStatus `json:"status"`
}

var fs = afero.NewOsFs()
var afs = &afero.Afero{Fs: fs}

func WaitForContainerService() (bool, error) {
	retries := 3
	for retries > 0 {
		exist, err := afs.Exists(constants.ContainerServiceSock)
		if err != nil {
			return false, errors.Wrapf(err, "Error while checking container service sock %s", constants.ContainerServiceSock)
		}
		if exist {
			out, err := utils.Exec.Run(false, "/usr/bin/crictl", "--runtime-endpoint", "unix://"+constants.ContainerServiceSock, "info")
			if err == nil {
				log.Trace(string(out))
				response := &CRIStatusResponse{}
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
					log.WithError(err).Warn("Error while parsing crictl status")
				}
			} else {
				log.WithError(err).Warn("Error while checking container service sock")
			}
		}
		retries = retries - 1

		log.Debug("Waiting 2 seconds...")
		time.Sleep(2 * time.Second)
	}
	return retries > 0, nil
}
