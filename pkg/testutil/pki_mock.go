package testutil

import (
	"fmt"

	certutil "k8s.io/client-go/util/cert"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta4"
	pkiutil "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/pki"
)

func CreateTestCA(fs host.FileSystem, dir string) error {
	caCert, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName: "test-ca",
		},
		EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
	})
	if err != nil {
		return fmt.Errorf("failed to create test CA certificate and key: %w", err)
	}

	if dir == "" {
		dir = v1beta4.DefaultCertificatesDir
	}
	if err := pki.WriteCertAndKey(fs, dir, "ca", caCert, caKey); err != nil {
		return fmt.Errorf("failed to write test CA certificate and key: %w", err)
	}
	return nil
}
