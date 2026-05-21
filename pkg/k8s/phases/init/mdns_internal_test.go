package init

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mockInit "github.com/kaweezle/iknite/mocks/pkg/k8s/phases/init"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/testutil"
)

func TestMDNSPublish(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	spec := &v1alpha1.IkniteClusterSpec{CreateIp: true, DomainName: "cluster.iknite", EnableMDNS: true}
	v1alpha1.SetDefaults_IkniteClusterSpec(spec)
	cluster := &v1alpha1.IkniteCluster{Spec: *spec}

	logger := testutil.TestLogger(t)
	m := mockInit.NewMockMdnsData(t)
	m.EXPECT().Logger().Return(logger).Once()
	m.EXPECT().IkniteCluster().Return(cluster).Once()
	m.EXPECT().RegisterShutdownHook("mdns", mock.Anything).RunAndReturn(func(_ string, fn func() error) {
		t.Cleanup(func() {
			req.NoError(fn())
		})
	}).Once()

	err := runMDnsPublish(m)
	req.NoError(err, "expected no error from runMDnsPublish")

	// run with bad data
	err = runMDnsPublish("toto")
	req.Error(err)
	req.Contains(err.Error(), "mdns phase invoked with an invalid data struct")
}

func TestMDNSPublish_MDNSDisabled(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	spec := &v1alpha1.IkniteClusterSpec{CreateIp: true, DomainName: "cluster.iknite"}
	v1alpha1.SetDefaults_IkniteClusterSpec(spec)
	cluster := &v1alpha1.IkniteCluster{Spec: *spec}

	logger := testutil.TestLogger(t)
	m := mockInit.NewMockMdnsData(t)
	m.EXPECT().Logger().Return(logger).Once()
	m.EXPECT().IkniteCluster().Return(cluster).Once()

	err := runMDnsPublish(m)
	req.NoError(err, "expected no error from runMDnsPublish with MDNS disabled")
}

func TestMDNSPublishPhase(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	spec := &v1alpha1.IkniteClusterSpec{CreateIp: true, DomainName: "cluster.iknite", EnableMDNS: true}
	v1alpha1.SetDefaults_IkniteClusterSpec(spec)
	cluster := &v1alpha1.IkniteCluster{Spec: *spec}

	logger := testutil.TestLogger(t)
	m := mockInit.NewMockMdnsData(t)
	m.EXPECT().Logger().Return(logger).Once()
	m.EXPECT().IkniteCluster().Return(cluster).Once()
	m.EXPECT().RegisterShutdownHook("mdns", mock.Anything).RunAndReturn(func(_ string, fn func() error) {
		t.Cleanup(func() {
			req.NoError(fn())
		})
	}).Once()

	phase := NewMDnsPublishPhase()
	err := phase.Run(m)
	req.NoError(err, "expected no error from MDNS publish phase")
}
