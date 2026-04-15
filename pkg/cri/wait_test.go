// cSpell: words paralleltest stretchr testutils
//
//nolint:paralleltest,lll // uses package globals and long mocked JSON payloads
package cri_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/cri"
	tu "github.com/kaweezle/iknite/pkg/testutils"
)

var waitForContainerServiceTestMu sync.Mutex

func TestWaitForContainerService(t *testing.T) {
	waitForContainerServiceTestMu.Lock()
	defer waitForContainerServiceTestMu.Unlock()

	tests := []struct {
		prepareExec  func(m *tu.MockExecutor)
		name         string
		wantReady    bool
		wantErr      bool
		expectSocket bool
	}{
		{
			name: "service becomes ready",
			prepareExec: func(m *tu.MockExecutor) {
				m.On(
					"Run",
					false,
					"/usr/bin/crictl",
					"--runtime-endpoint",
					"unix://"+constants.ContainerServiceSock,
					"info",
				).Return(`{"status":{"conditions":[{"type":"RuntimeReady","status":true},{"type":"NetworkReady","status":true}]}}`, nil).Once()
			},
			wantReady:    true,
			wantErr:      false,
			expectSocket: true,
		},
		{
			name: "service not ready after retries",
			prepareExec: func(m *tu.MockExecutor) {
				m.On(
					"Run",
					false,
					"/usr/bin/crictl",
					"--runtime-endpoint",
					"unix://"+constants.ContainerServiceSock,
					"info",
				).Return(`{"status":{"conditions":[{"type":"RuntimeReady","status":false}]}}`, nil).Times(3)
			},
			wantReady:    false,
			wantErr:      false,
			expectSocket: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			fs, mockExec, cleanup := tu.CreateTestFSAndExecutor()
			t.Cleanup(cleanup)
			tt.prepareExec(mockExec)

			if tt.expectSocket {
				req.NoError(fs.WriteFile(constants.ContainerServiceSock, []byte(""), 0o600))
			}

			ready, err := cri.WaitForContainerService()
			if tt.wantErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}
			req.Equal(tt.wantReady, ready)
			mockExec.AssertExpectations(t)
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
