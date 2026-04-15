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
// cSpell: words stretchr clientcmd
package server_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/server"
)

func TestStatusAndHealthzMethodHandling(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	dir := t.TempDir()
	createTestCA(t, dir)
	spec := makeTestSpec(0)
	req.NoError(server.EnsureServerCertAndKey(dir, []string{spec.DomainName}, []net.IP{spec.Ip}))

	iSrv, err := server.NewIkniteServer(dir, spec)
	req.NoError(err)

	postStatusReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/status", http.NoBody)
	req.NoError(err)
	postStatusRec := httptest.NewRecorder()
	iSrv.ServeHTTP(postStatusRec, postStatusReq)
	req.Equal(http.StatusMethodNotAllowed, postStatusRec.Code)

	getStatusReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/status", http.NoBody)
	req.NoError(err)
	getStatusRec := httptest.NewRecorder()
	iSrv.ServeHTTP(getStatusRec, getStatusReq)
	req.Equal(http.StatusServiceUnavailable, getStatusRec.Code)

	postHealthReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/healthz", http.NoBody)
	req.NoError(err)
	postHealthRec := httptest.NewRecorder()
	iSrv.ServeHTTP(postHealthRec, postHealthReq)
	req.Equal(http.StatusMethodNotAllowed, postHealthRec.Code)
}

func TestShutdownServerNil(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	var srv *server.IkniteServer
	req.NoError(server.ShutdownServer(srv))
}

func TestEnsureIkniteConfAddressSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		spec          *v1alpha1.IkniteClusterSpec
		wantServerSub string
	}{
		{
			name: "loopback with domain uses domain",
			spec: &v1alpha1.IkniteClusterSpec{
				Ip:               net.ParseIP("127.0.0.1"),
				DomainName:       "iknite.local",
				StatusServerPort: 11443,
			},
			wantServerSub: "iknite.local:11443",
		},
		{
			name: "loopback without domain uses localhost",
			spec: &v1alpha1.IkniteClusterSpec{
				Ip:               net.ParseIP("127.0.0.1"),
				StatusServerPort: 11443,
			},
			wantServerSub: "localhost:11443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			dir := t.TempDir()
			createTestCA(t, dir)
			req.NoError(server.EnsureClientCertAndKey(dir))

			confPath := filepath.Join(dir, constants.IkniteConfName+".conf")
			req.NoError(server.EnsureIkniteConf(dir, confPath, tt.spec))

			cfg, err := clientcmd.LoadFromFile(confPath)
			req.NoError(err)
			ctx := cfg.Contexts[cfg.CurrentContext]
			req.NotNil(ctx)
			req.Contains(cfg.Clusters[ctx.Cluster].Server, tt.wantServerSub)
		})
	}
}
