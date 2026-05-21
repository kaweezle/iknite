package reset

// cSpell: words cleanupservice

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"

	mockHost "github.com/kaweezle/iknite/mocks/pkg/host"
	mockReset "github.com/kaweezle/iknite/mocks/pkg/k8s/phases/reset"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/testutil"
)

const startedIkniteServiceLink = "/run/openrc/started/" + constants.IkniteService

func TestRunCleanupService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		dryRun    bool
		exists    bool
		expectRun bool
		runFails  bool
	}{
		{
			name:   "dry run skips service stop",
			dryRun: true,
		},
		{
			name:      "service already stopped",
			exists:    false,
			expectRun: false,
		},
		{
			name:      "service stop succeeds",
			exists:    true,
			expectRun: true,
		},
		{
			name:      "service stop failure is swallowed",
			exists:    true,
			expectRun: true,
			runFails:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			logger := testutil.TestLogger(t)
			hostMock := mockHost.NewMockHost(t)
			data := mockReset.NewMockCleanupServiceData(t)

			data.EXPECT().Logger().Return(logger).Once()
			data.EXPECT().Host().Return(hostMock).Once()
			data.EXPECT().DryRun().Return(tt.dryRun).Once()

			if !tt.dryRun {
				hostMock.EXPECT().Exists(startedIkniteServiceLink).Return(tt.exists, nil).Once()
				if tt.expectRun {
					runOutput := []byte("stopped")
					var runErr error
					if tt.runFails {
						runOutput = nil
						runErr = errors.New("stop failed")
					}
					hostMock.EXPECT().Run(false, "/sbin/rc-service", []string{constants.IkniteService, "stop"}).
						Return(runOutput, runErr).Once()
				}
			}

			err := runCleanupService(data)
			req.NoError(err)
		})
	}
}

func TestCleanupServicePhase(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	phase := NewCleanupServicePhase()

	req.Equal("cleanup-service", phase.Name)
	req.Equal([]string{"cleanupservice"}, phase.Aliases)
	req.Equal([]string{
		options.CertificatesDir,
		options.NodeCRISocket,
		options.CleanupTmpDir,
		options.DryRun,
	}, phase.InheritFlags)
	req.NotNil(phase.Run)

	logger := testutil.TestLogger(t)
	hostMock := mockHost.NewMockHost(t)
	data := mockReset.NewMockCleanupServiceData(t)

	data.EXPECT().Logger().Return(logger).Once()
	data.EXPECT().Host().Return(hostMock).Once()
	data.EXPECT().DryRun().Return(true).Once()

	err := phase.Run(data)
	req.NoError(err)
}

func TestRunCleanupService_BadData(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	err := runCleanupService("bad data")
	req.Error(err)
	req.Contains(err.Error(), "cleanup-node phase invoked with an invalid data struct")
}
