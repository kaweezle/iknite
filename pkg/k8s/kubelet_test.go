// cSpell: words cgroupfs
package k8s_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/k8s"
)

func TestCheckServerRunning(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start a simple HTTP server in the background
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok")) //nolint:errcheck // Test server, ignoring write errors
	}))
	defer srv.Close()

	// Check if the server is running
	err := k8s.CheckServerRunning(ctx, srv.URL, "ok", 1, 1, 1*time.Second)
	req.NoError(err)
}

func TestCheckServerRunning_BadResponse(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start a simple HTTP server in the background
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("bad")) //nolint:errcheck // Test server, ignoring write errors
	}))
	defer srv.Close()

	// Check if the server is running
	err := k8s.CheckServerRunning(ctx, srv.URL, "ok", 2, 1, 1*time.Second)
	req.Error(err)
	req.Contains(err.Error(), "unexpected response body")
}

func TestCheckServerRunning_NotFound(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start a simple HTTP server in the background
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// Check if the server is running
	err := k8s.CheckServerRunning(ctx, srv.URL, "ok", 1, 1, 1*time.Second)
	req.Error(err)
	req.Contains(err.Error(), "unexpected status code")
}

func TestCheckServerRunning_BadURL(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start a simple HTTP server in the background
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// Check if the server is running
	badChar := string([]byte{0x7f}) // Creates the ASCII DEL Control Character
	err := k8s.CheckServerRunning(ctx, srv.URL+"/foo"+badChar, "ok", 1, 1, 1*time.Second)
	req.Error(err)
	req.Contains(err.Error(), "failed to create HTTP request")
}

func TestCheckServerRunning_BadServer(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if the server is running
	err := k8s.CheckServerRunning(ctx, "http://127.0.0.2/foo", "ok", 1, 1, 1*time.Second)
	req.Error(err)
	req.Contains(err.Error(), "failed to make HTTP request")
}
