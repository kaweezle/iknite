package init_test

import (
	"bytes"
	"io"
	"net"
	"testing"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	ikniteConfig "github.com/kaweezle/iknite/pkg/config"
	initPhases "github.com/kaweezle/iknite/pkg/k8s/phases/init"
)

func TestCreateKubeVipConfiguration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		wr      io.Writer
		config  *v1alpha1.IkniteClusterSpec
		name    string
		wantErr bool
	}{
		{
			name:    "nil writer",
			wr:      nil,
			config:  &v1alpha1.IkniteClusterSpec{},
			wantErr: true,
		},
		{
			name: "valid config and writer",
			wr:   new(bytes.Buffer),
			config: &v1alpha1.IkniteClusterSpec{
				DomainName: "iknite.local",
				Ip:         net.ParseIP("192.168.99.2"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotErr := initPhases.CreateKubeVipConfiguration(tt.wr, tt.config)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("CreateKubeVipConfiguration() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatalf("CreateKubeVipConfiguration() succeeded unexpectedly: %s", tt.name)
			}

			// Now check that the image is correctly set in the output
			outputBuf, ok := tt.wr.(*bytes.Buffer)
			if !ok {
				t.Fatal("Writer is not a bytes.Buffer")
			}
			outputStr := outputBuf.String()
			expectedImage := ikniteConfig.GetKubeVipImage()
			if !bytes.Contains([]byte(outputStr), []byte(expectedImage)) {
				t.Errorf("Output does not contain expected image %s", expectedImage)
			}
			if tt.config.DomainName != "" {
				if !bytes.Contains([]byte(outputStr), []byte(tt.config.DomainName)) {
					t.Errorf(
						"Output does not contain expected domain name %s",
						tt.config.DomainName,
					)
				}
			}
		})
	}
}
