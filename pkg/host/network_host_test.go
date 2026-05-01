package host_test

import (
	"context"
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

func TestGetHostConfig(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	nh := host.NewDefaultNetworkHost()
	config := nh.GetHostsConfig()
	req.NotNil(config)
}

func TestIsHostMapped(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	nh := host.NewDefaultNetworkHost()
	localhost := net.ParseIP("127.0.0.1")
	req.NotNil(localhost)
	result, ips := nh.IsHostMapped(context.Background(), localhost, "localhost")
	req.True(result, "localhost should be mapped to 127.0.0.1")
	req.Contains(ips, localhost, "localhost should be mapped to 127.0.0.1")
	// Check for a non-existent mapping
	nonexistent := net.ParseIP("10.0.0.16")
	req.NotNil(nonexistent)
	result, ips = nh.IsHostMapped(context.Background(), nonexistent, "nonexistent")
	req.False(result, "10.0.0.16 shouldn't be mapped to nonexistent")
}
