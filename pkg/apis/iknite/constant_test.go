// cSpell: words stretchr
package iknite_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/apis/iknite"
)

func TestClusterState_StringSetAndStable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		wantString string
		value      iknite.ClusterState
		stable     bool
	}{
		{name: "undefined", value: iknite.Undefined, wantString: "Undefined", stable: true},
		{name: "stopped", value: iknite.Stopped, wantString: "Stopped", stable: true},
		{name: "started", value: iknite.Started, wantString: "Started", stable: false},
		{name: "initializing", value: iknite.Initializing, wantString: "Initializing", stable: false},
		{name: "stabilizing", value: iknite.Stabilizing, wantString: "Stabilizing", stable: false},
		{name: "running", value: iknite.Running, wantString: "Running", stable: true},
		{name: "stopping", value: iknite.Stopping, wantString: "Stopping", stable: false},
		{name: "cleaning", value: iknite.Cleaning, wantString: "Cleaning", stable: false},
		{name: "failed", value: iknite.Failed, wantString: "Failed", stable: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			req.Equal(tt.wantString, tt.value.String())
			req.Equal(tt.stable, tt.value.Stable())

			var state iknite.ClusterState
			state.Set(tt.wantString)
			req.Equal(tt.value, state)
		})
	}
}

func TestClusterState_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	original := iknite.Running
	payload, err := original.MarshalJSON()
	req.NoError(err)
	req.Equal("\"Running\"", string(payload))

	var decoded iknite.ClusterState
	err = decoded.UnmarshalJSON(payload)
	req.NoError(err)
	req.Equal(iknite.Running, decoded)

	b, err := json.Marshal(original)
	req.NoError(err)
	req.Equal("\"Running\"", string(b))
}
