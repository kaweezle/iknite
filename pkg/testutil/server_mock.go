package testutil

import (
	"crypto"
	"crypto/x509"
	"embed"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	argoprojv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	certUtil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
	pkiutil "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/host"
)

//go:embed testdata
var content embed.FS

func NewWorkloadRESTMapper(includeApplications bool) meta.RESTMapper {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "apps", Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
	mapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}, meta.RESTScopeNamespace)
	mapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"}, meta.RESTScopeNamespace)
	if includeApplications {
		mapper.Add(argoprojv1.ApplicationSchemaGroupVersionKind, meta.RESTScopeNamespace)
	}
	return mapper
}

func AddClientCertificateToConfig(t *testing.T, config *rest.Config, server *httptest.Server) {
	t.Helper()
	fullCert := server.TLS.Certificates[0]
	priv, ok := fullCert.PrivateKey.(crypto.Signer)
	require.True(t, ok, "Private key is not a crypto.Signer")

	certConfig := &pkiutil.CertConfig{
		Config: certUtil.Config{
			CommonName:   "iknite-client",
			Organization: []string{"system:iknite"},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
	}

	cert, key, err := pkiutil.NewCertAndKey(server.Certificate(), priv, certConfig)
	require.NoError(t, err)
	clientCertPem, err := certUtil.EncodeCertificates(cert)
	require.NoError(t, err)
	clientKeyPem, err := keyutil.MarshalPrivateKeyToPEM(key)
	require.NoError(t, err)

	config.CertData = clientCertPem
	config.KeyData = clientKeyPem
}

func CreateTestAPIServer(t *testing.T, handler http.HandlerFunc) *rest.Config {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	cert := server.Certificate()
	pem, err := certUtil.EncodeCertificates(cert)
	require.NoError(t, err)

	// The server wouldn't enforce client authentication, but we create a client certificate to test that the client
	// configuration is correctly set up and that the server can parse the client certificate if provided.
	// We do this at this point because the server CA key will be hidden afterwards
	restConfig := &rest.Config{Host: server.URL, TLSClientConfig: rest.TLSClientConfig{CAData: pem}}
	AddClientCertificateToConfig(t, restConfig, server)
	return restConfig
}

func WriteRestConfigToFile(t *testing.T, config *rest.Config, fs host.FileSystem, path, confName string) {
	t.Helper()
	apiConfig := kubeconfig.CreateWithCerts(
		config.Host,
		confName, // cluster name
		confName, // user name
		config.CAData,
		config.KeyData,
		config.CertData,
	)

	if apiConfig.Extensions == nil {
		apiConfig.Extensions = make(map[string]runtime.Object)
	}
	obj, err := scheme.Scheme.New(v1alpha1.SchemeGroupVersionWithKind)
	require.NoError(t, err)
	apiConfig.Extensions["test-rest-mapper"] = obj

	content, err := clientcmd.Write(*apiConfig)
	require.NoError(t, err, "Failed to serialize kubeconfig")

	err = fs.WriteFile(path, content, 0o644)
	require.NoError(t, err, "Failed to write kubeconfig file")
}

func PatchHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("Received request: %s %s %s", r.Method, r.URL.Path, r.URL.RawQuery)
	switch r.Method {
	case http.MethodPatch:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			logrus.Errorf("Failed to read request body: %v", err)
		} else {
			logrus.Infof("Request body: %s", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body) //nolint:errcheck // test server, ignore error
	default:
		logrus.Warnf("Unexpected request: %s %s %s", r.Method, r.URL.Path, r.URL.RawQuery)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

const notFoundResponse = `{
    "kind": "Status",
    "apiVersion": "v1",
    "metadata": {},
    "status": "Failure",
    "message": "the server could not find the requested resource",
    "reason": "NotFound",
    "details": {}
    "code": 404
}`

type RequestLog struct {
	Method     string
	Path       string
	Query      string
	Body       string
	StatusCode int
}

func ellipsis(s string, n int) string {
	r := []rune(s) // Convert s to a slice of runes and truncate it instead.
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func (l *RequestLog) String() string {
	r := &strings.Builder{}
	fmt.Fprintf(r, "%s %s", l.Method, l.Path)
	if l.Query != "" {
		fmt.Fprintf(r, "?%s", l.Query)
	}
	fmt.Fprintf(r, " - %d", l.StatusCode)
	if l.Body != "" {
		fmt.Fprintf(r, "\nbody (%d bytes) %s", len(l.Body), ellipsis(l.Body, 100))
	}
	return r.String()
}

type TestServerOptions struct {
	FailurePaths []string
	Requests     []RequestLog
}

func ContentPatchHandler(subdir string, options *TestServerOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := &RequestLog{
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  r.URL.RawQuery,
		}
		defer func() {
			options.Requests = append(options.Requests, *log)
			logrus.Info(log.String())
		}()
		path := strings.TrimSuffix(r.URL.Path, "/")
		if slices.Contains(options.FailurePaths, path) {
			logrus.Infof("Simulating failure for path: %s", path)
			log.StatusCode = http.StatusInternalServerError
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		switch r.Method {
		case http.MethodGet:
			filename := filepath.Join("testdata", subdir, path+".json")
			content, err := content.ReadFile(filename)
			if err != nil {
				if os.IsNotExist(err) {
					logrus.Errorf("File not found: %s", filename)
					log.StatusCode = http.StatusNotFound
					w.WriteHeader(http.StatusNotFound)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(notFoundResponse)) //nolint:errcheck // test server, ignore error
				} else {
					logrus.Errorf("Failed to read file %s: %v", filename, err)
					log.StatusCode = http.StatusInternalServerError
					w.WriteHeader(http.StatusInternalServerError)
				}
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			log.StatusCode = http.StatusOK
			_, _ = w.Write(content) //nolint:errcheck,gosec // test server, ignore error
		case http.MethodPatch:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				logrus.Errorf("Failed to read request body: %v", err)
			}
			log.Body = string(body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			log.StatusCode = http.StatusOK
			_, _ = w.Write(body) //nolint:errcheck // test server, ignore error
		case http.MethodPost:
			body, err := io.ReadAll(r.Body)
			log.Body = string(body)
			content_type := r.Header.Get("Content-Type")
			if err != nil {
				logrus.Errorf("Failed to read request body: %v", err)
			} else {
				logrus.Infof("Request body: %s", string(body))
				logrus.Infof("Content-Type: %s", content_type)
			}
			w.Header().Set("Content-Type", content_type)
			w.WriteHeader(http.StatusOK)
			log.StatusCode = http.StatusOK
			_, _ = w.Write(body) //nolint:errcheck // test server, ignore error
		default:
			logrus.Warnf("Unexpected request: %s %s %s", r.Method, r.URL.Path, r.URL.RawQuery)
			w.WriteHeader(http.StatusInternalServerError)
			log.StatusCode = http.StatusInternalServerError
		}
	}
}

const basicConfigmap = `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
`

const basicKustomization = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: kube-system
resources:
- configmap.yaml
`

func CreateBasicKustomization(fs host.FileSystem, dir string, failing bool) error {
	err := fs.MkdirAll(dir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	if err := fs.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(basicKustomization), 0o600); err != nil {
		return fmt.Errorf("failed to write kustomization.yaml: %w", err)
	}
	if !failing {
		if err := fs.WriteFile(filepath.Join(dir, "configmap.yaml"), []byte(basicConfigmap), 0o600); err != nil {
			return fmt.Errorf("failed to write configmap.yaml: %w", err)
		}
	}
	return nil
}
