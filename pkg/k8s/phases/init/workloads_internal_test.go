// cSpell: words testutil apimachinery errgroup
package init

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mockData "github.com/kaweezle/iknite/mocks/pkg/k8s/phases/init"
	ikniteApi "github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/testutil"
)

func createTestIkniteCluster() *v1alpha1.IkniteCluster {
	return &v1alpha1.IkniteCluster{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "IkniteCluster",
			APIVersion: "iknite.kaweezle.com/v1alpha1",
		},
		Spec: v1alpha1.IkniteClusterSpec{
			DomainName:                      "iknite.local",
			Ip:                              net.ParseIP("192.168.99.2"),
			StatusUpdateIntervalSeconds:     1,
			StatusUpdateLongIntervalSeconds: 1,
		},
		Status: v1alpha1.IkniteClusterStatus{
			State:        ikniteApi.Stabilizing,
			CurrentPhase: "workloads",
		},
	}
}

func TestRunMonitorWorkloads(t *testing.T) {
	t.Parallel()

	t.Run("Normal flow", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		m := mockData.NewMockMonitorData(t)
		sOpts := &testutil.TestServerOptions{}
		getter := testutil.CreateDefaultTestClientGetter(t, sOpts)
		m.EXPECT().RESTClientGetter().Return(getter, nil).Once()

		cluster := createTestIkniteCluster()
		errGroup, egContext := errgroup.WithContext(t.Context())
		m.EXPECT().ErrGroup().Return(errGroup).Once()
		ctx, cancel := context.WithCancel(egContext)
		defer cancel()
		m.EXPECT().Context().Return(ctx).Maybe()
		m.EXPECT().IkniteCluster().Return(cluster).Twice()
		m.EXPECT().UpdateIkniteCluster(
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).RunAndReturn(func(state ikniteApi.ClusterState, phase string, ready, unready []*v1alpha1.WorkloadState) {
			req.Equal(ikniteApi.Running, state)
			req.Equal("daemonize", phase)
			req.Empty(unready)
			req.NotEmpty(ready)
			cancel()
		}).Once()

		err := runMonitorWorkloads(m)
		req.NoError(err)

		// Wait on context cancellation to ensure the test doesn't exit before the assertions in the callback are executed.
		err = errGroup.Wait()
		req.Error(err)
		req.Contains(err.Error(), "context canceled")
	})

	t.Run("Fail on bad data struct", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		err := runMonitorWorkloads("bad-data")
		req.Error(err)
	})

	t.Run("Fail bad getter", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		m := mockData.NewMockMonitorData(t)
		m.EXPECT().RESTClientGetter().Return(nil, errors.New("Bad getter")).Once()
		err := runMonitorWorkloads(m)
		req.Error(err)
		req.Contains(err.Error(), "cannot load the kubernetes configuration")
	})

	t.Run("Fail on download error", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		m := mockData.NewMockMonitorData(t)
		getter := testutil.CreateFailureGetter(t)
		m.EXPECT().RESTClientGetter().Return(getter, nil).Once()

		cluster := createTestIkniteCluster()
		errGroup, ctx := errgroup.WithContext(t.Context())
		m.EXPECT().ErrGroup().Return(errGroup).Once()

		m.EXPECT().Context().Return(ctx).Maybe()
		m.EXPECT().IkniteCluster().Return(cluster).Once()
		err := runMonitorWorkloads(m)
		req.NoError(err)
		err = errGroup.Wait()
		req.Error(err)
		req.Contains(err.Error(), "failed to get resource infos")
	})
}

func TestNewWorkloadsPhase(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	m := mockData.NewMockMonitorData(t)
	sOpts := &testutil.TestServerOptions{}
	getter := testutil.CreateDefaultTestClientGetter(t, sOpts)
	m.EXPECT().RESTClientGetter().Return(getter, nil).Once()

	cluster := createTestIkniteCluster()
	errGroup, egContext := errgroup.WithContext(t.Context())
	m.EXPECT().ErrGroup().Return(errGroup).Once()
	ctx, cancel := context.WithCancel(egContext)
	defer cancel()
	m.EXPECT().Context().Return(ctx).Maybe()
	m.EXPECT().IkniteCluster().Return(cluster).Twice()
	m.EXPECT().UpdateIkniteCluster(
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).RunAndReturn(func(state ikniteApi.ClusterState, phase string, ready, unready []*v1alpha1.WorkloadState) {
		req.Equal(ikniteApi.Running, state)
		req.Equal("daemonize", phase)
		req.Empty(unready)
		req.NotEmpty(ready)
		cancel()
	}).Once()

	phase := NewWorkloadsPhase()
	req.Equal("workloads", phase.Name)
	req.NotNil(phase.Run)
	err := phase.Run(m)
	req.NoError(err)

	// Wait on context cancellation to ensure the test doesn't exit before the assertions in the callback are executed.
	err = errGroup.Wait()
	req.Error(err)
	req.Contains(err.Error(), "context canceled")
}
