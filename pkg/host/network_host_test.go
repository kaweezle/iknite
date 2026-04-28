package host_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/host"
)

func TestIPExists(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	localhost := net.ParseIP("127.0.0.1")
	req.NotNil(localhost)

	nh := host.NewDefaultNetworkHost()

	result, err := nh.CheckIpExists(localhost)
	req.NoError(err)
	req.True(result, "Localhost should exist")

	nonexistent := net.ParseIP("10.0.0.16")
	req.NotNil(nonexistent)

	result, err = nh.CheckIpExists(nonexistent)
	req.NoError(err)
	req.False(result, "10.0.0.16 shouldn't exist")
}
