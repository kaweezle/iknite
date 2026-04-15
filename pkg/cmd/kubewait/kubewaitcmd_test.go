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
// cSpell: words stretchr
//
package kubewait_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/cmd/kubewait"
)

func TestCreateKubewaitCmd(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	buf := &bytes.Buffer{}
	cmd := kubewait.CreateKubewaitCmd(buf)
	req.NotNil(cmd)
	req.Equal("kubewait", cmd.Name())
	req.NotNil(cmd.PersistentPreRunE)

	flags := cmd.Flags()
	req.NotNil(flags.Lookup("kubeconfig"))
	req.NotNil(flags.Lookup("timeout"))
	req.NotNil(flags.Lookup("resource-types"))
	req.NotNil(flags.Lookup("status-update-interval"))
	req.NotNil(flags.Lookup("resources-update-interval"))

	err := flags.Parse([]string{"--resource-types", "deployments,daemonsets", "default"})
	req.NoError(err)
}
