// cSpell: words paralleltest testutils
//
//nolint:paralleltest,lll // uses package globals and long mocked JSON payloads
package cri_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	mockHost "github.com/kaweezle/iknite/mocks/github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/cri"
	"github.com/kaweezle/iknite/pkg/host"
)

func TestWaitForContainerService(t *testing.T) {
	tests := []struct {
		prepareExec  func(m *mockHost.MockExecutor)
		name         string
		wantReady    bool
		wantErr      bool
		expectSocket bool
	}{
		{
			name: "service becomes ready",
			prepareExec: func(m *mockHost.MockExecutor) {
				m.On(
					"Run",
					false,
					"/usr/bin/crictl",
					[]string{
						"--runtime-endpoint",
						"unix://" + constants.ContainerServiceSock,
						"info",
					},
				).Return([]byte(`{"status":{"conditions":[{"type":"RuntimeReady","status":true},{"type":"NetworkReady","status":true}]}}`), nil).Once()
			},
			wantReady:    true,
			wantErr:      false,
			expectSocket: true,
		},
		{
			name: "service not ready after retries",
			prepareExec: func(m *mockHost.MockExecutor) {
				m.On(
					"Run",
					false,
					"/usr/bin/crictl",
					[]string{
						"--runtime-endpoint",
						"unix://" + constants.ContainerServiceSock,
						"info",
					},
				).Return([]byte(`{"status":{"conditions":[{"type":"RuntimeReady","status":false}]}}`), nil).Times(3)
			},
			wantReady:    false,
			wantErr:      false,
			expectSocket: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			fs := host.NewMemMapFS()
			mockExec := mockHost.NewMockExecutor(t)
			tt.prepareExec(mockExec)

			if tt.expectSocket {
				req.NoError(fs.WriteFile(constants.ContainerServiceSock, []byte(""), 0o600))
			}

			ready, err := cri.WaitForContainerService(fs, mockExec)
			if tt.wantErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}
			req.Equal(tt.wantReady, ready)
		})
	}
}

func TestCRIStatusResponseJSONShape(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	status := cri.CRIStatusResponse{
		Status: cri.CRIStatus{Conditions: []cri.CRICondition{{Type: "RuntimeReady", Status: true}}},
	}
	req.Equal("RuntimeReady", status.Status.Conditions[0].Type)
	req.True(status.Status.Conditions[0].Status)
	req.Error(errors.New("x"))
}
