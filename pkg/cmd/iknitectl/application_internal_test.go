/*
Copyright © 2025 Antoine Martin <antoine@openance.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package iknitectl

// cSpell: words dvcm appstage mockhost hostpkg crds testutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resmap"

	mockhost "github.com/kaweezle/iknite/mocks/pkg/host"
	hostpkg "github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/testutil"
)

// ---- helpers ----------------------------------------------------------------

func newMemFileExecutor(t *testing.T) hostpkg.FileExecutor {
	t.Helper()

	fileExecutor, ok := hostpkg.NewMemMapFS().(hostpkg.FileExecutor)
	if !ok {
		t.Fatal("failed to create FileExecutor from mem fs")
	}

	return fileExecutor
}

func writeFile(t *testing.T, fileExecutor hostpkg.FileExecutor, path, content string) {
	t.Helper()
	if err := fileExecutor.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := fileExecutor.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

type applicationContextKey string

type errWriter struct {
	err error
}

func (w errWriter) Write(_ []byte) (int, error) {
	return 0, w.err
}

type stubFileInfo struct {
	name string
	dir  bool
}

func (f stubFileInfo) Name() string       { return f.name }
func (f stubFileInfo) Size() int64        { return 0 }
func (f stubFileInfo) Mode() os.FileMode  { return 0o644 }
func (f stubFileInfo) ModTime() time.Time { return time.Time{} }
func (f stubFileInfo) IsDir() bool        { return f.dir }
func (f stubFileInfo) Sys() any           { return nil }

type interceptingFileExecutor struct {
	hostpkg.FileExecutor
	dirExistsErrs map[string]error
	existsErrs    map[string]error
	removeAllErrs map[string]error
	mkdirAllErrs  map[string]error
	readDirErrs   map[string]error
	readFileErrs  map[string]error
	writeFileErrs map[string]error
}

func (f *interceptingFileExecutor) DirExists(path string) (bool, error) {
	if err := f.dirExistsErrs[path]; err != nil {
		return false, err
	}
	exists, err := f.FileExecutor.DirExists(path)
	if err != nil {
		return false, fmt.Errorf("DirExists %s: %w", path, err)
	}
	return exists, nil
}

func (f *interceptingFileExecutor) Exists(path string) (bool, error) {
	if err := f.existsErrs[path]; err != nil {
		return false, err
	}
	exists, err := f.FileExecutor.Exists(path)
	if err != nil {
		return false, fmt.Errorf("Exists %s: %w", path, err)
	}
	return exists, nil
}

func (f *interceptingFileExecutor) RemoveAll(path string) error {
	if err := f.removeAllErrs[path]; err != nil {
		return err
	}
	if err := f.FileExecutor.RemoveAll(path); err != nil {
		return fmt.Errorf("RemoveAll %s: %w", path, err)
	}
	return nil
}

func (f *interceptingFileExecutor) MkdirAll(path string, perm os.FileMode) error {
	if err := f.mkdirAllErrs[path]; err != nil {
		return err
	}
	if err := f.FileExecutor.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("MkdirAll %s: %w", path, err)
	}
	return nil
}

func (f *interceptingFileExecutor) ReadDir(path string) ([]os.FileInfo, error) {
	if err := f.readDirErrs[path]; err != nil {
		return nil, err
	}
	files, err := f.FileExecutor.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("ReadDir %s: %w", path, err)
	}
	return files, nil
}

func (f *interceptingFileExecutor) ReadFile(path string) ([]byte, error) {
	if err := f.readFileErrs[path]; err != nil {
		return nil, err
	}
	data, err := f.FileExecutor.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ReadFile %s: %w", path, err)
	}
	return data, nil
}

func (f *interceptingFileExecutor) WriteFile(path string, data []byte, perm os.FileMode) error {
	if err := f.writeFileErrs[path]; err != nil {
		return err
	}
	if err := f.FileExecutor.WriteFile(path, data, perm); err != nil {
		return fmt.Errorf("WriteFile %s: %w", path, err)
	}
	return nil
}

func newDummyFileExecutor(t *testing.T, options *testutil.DummyHostOptions) *testutil.DelegateHost {
	t.Helper()

	if options == nil {
		options = &testutil.DummyHostOptions{}
	}

	host, err := testutil.NewDummyHost(hostpkg.NewMemMapFS(), options)
	if err != nil {
		t.Fatalf("NewDummyHost: %v", err)
	}
	if networkHost, ok := host.Net.(*testutil.DummyNetworkHost); ok {
		t.Cleanup(func() {
			if err := networkHost.Cleanup(); err != nil {
				t.Logf("Cleanup: %v", err)
			}
		})
	}

	return host
}

func writeKustomizeApp(t *testing.T, fileExecutor hostpkg.FileExecutor, dir string) {
	t.Helper()

	writeFile(t, fileExecutor, filepath.Join(dir, "kustomization.yaml"), `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- configmap.yaml
`)
	writeFile(t, fileExecutor, filepath.Join(dir, "configmap.yaml"), configMapContent)
}

func writeHelmChart(t *testing.T, fileExecutor hostpkg.FileExecutor, dir string) {
	t.Helper()

	writeFile(t, fileExecutor, filepath.Join(dir, "Chart.yaml"), "apiVersion: v2\nname: app\nversion: 0.1.0\n")
}

func writeAppstageApplication(t *testing.T, fileExecutor hostpkg.FileExecutor, sourcePath string) {
	t.Helper()

	const appstageDir = "/repo/appstages/appstage-dev"
	kustomizationPath := filepath.Join(appstageDir, "kustomization.yaml")
	applicationPath := filepath.Join(appstageDir, "application.yaml")

	writeFile(
		t,
		fileExecutor,
		kustomizationPath,
		`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- application.yaml
`,
	)
	writeFile(
		t,
		fileExecutor,
		applicationPath,
		fmt.Sprintf(`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app-one
spec:
  source:
    path: %s
`, sourcePath),
	)
}

type asYamlErrorResMap struct {
	resmap.ResMap
	err error
}

func (r asYamlErrorResMap) AsYaml() ([]byte, error) {
	return nil, r.err
}

func mustResMapFromYAML(t *testing.T, yamlData string) resmap.ResMap {
	t.Helper()

	resources, err := resmap.NewFactory(provider.NewDefaultDepProvider().GetResourceFactory()).
		NewResMapFromBytes([]byte(yamlData))
	if err != nil {
		t.Fatalf("NewResMapFromBytes: %v", err)
	}

	return resources
}

func assertRenderCommand(
	t *testing.T,
	run func(context.Context, hostpkg.FileExecutor) (resmap.ResMap, error),
	wantCmd string,
	wantArgs []string,
) {
	t.Helper()

	ctx := context.WithValue(context.Background(), applicationContextKey("name"), wantCmd)
	fileExecutor := mockhost.NewMockFileExecutor(t)
	fileExecutor.EXPECT().RunCommand(ctx, mock.Anything).Run(
		func(actualCtx context.Context, options *hostpkg.CommandOptions) {
			if actualCtx != ctx {
				t.Fatalf("expected context to be passed through unchanged")
			}
			if options.Cmd != wantCmd {
				t.Fatalf("expected command %q, got %q", wantCmd, options.Cmd)
			}
			if !reflect.DeepEqual(options.Args, wantArgs) {
				t.Fatalf("expected args %v, got %v", wantArgs, options.Args)
			}
			if options.Stdout == nil {
				t.Fatal("expected stdout writer to be configured")
			}
			if _, err := io.WriteString(options.Stdout, configMapContent); err != nil {
				t.Fatalf("WriteString: %v", err)
			}
		},
	).Return(nil)

	resources, err := run(ctx, fileExecutor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	yamlData, err := resources.AsYaml()
	if err != nil {
		t.Fatalf("AsYaml: %v", err)
	}
	if !bytes.Contains(yamlData, []byte("kind: ConfigMap")) {
		t.Fatalf("expected ConfigMap in output, got: %s", string(yamlData))
	}
}

// ---- detectAppType ----------------------------------------------------------

func TestDetectAppType_Kustomize(t *testing.T) {
	t.Parallel()
	fileExecutor := newMemFileExecutor(t)
	writeFile(t, fileExecutor, "/app/kustomization.yaml", "")

	typ, path, err := detectAppType(fileExecutor, "/app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != appTypeKustomize {
		t.Errorf("expected appTypeKustomize, got %v", typ)
	}
	if path != "/app" {
		t.Errorf("expected path /app, got %s", path)
	}
}

func TestDetectAppType_Helmfile(t *testing.T) {
	t.Parallel()
	fileExecutor := newMemFileExecutor(t)
	writeFile(t, fileExecutor, "/app/helmfile.yaml", "")

	typ, path, err := detectAppType(fileExecutor, "/app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != appTypeHelmfile {
		t.Errorf("expected appTypeHelmfile, got %v", typ)
	}
	if path != "/app/helmfile.yaml" {
		t.Errorf("expected path /app/helmfile.yaml, got %s", path)
	}
}

func TestDetectAppType_HelmfileGotmpl(t *testing.T) {
	t.Parallel()
	fileExecutor := newMemFileExecutor(t)
	writeFile(t, fileExecutor, "/app/helmfile.yaml.gotmpl", "")

	typ, path, err := detectAppType(fileExecutor, "/app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != appTypeHelmfile {
		t.Errorf("expected appTypeHelmfile, got %v", typ)
	}
	if path != "/app/helmfile.yaml.gotmpl" {
		t.Errorf("expected path /app/helmfile.yaml.gotmpl, got %s", path)
	}
}

func TestDetectAppType_Helm(t *testing.T) {
	t.Parallel()
	fileExecutor := newMemFileExecutor(t)
	writeFile(t, fileExecutor, "/app/Chart.yaml", "")

	typ, path, err := detectAppType(fileExecutor, "/app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != appTypeHelm {
		t.Errorf("expected appTypeHelm, got %v", typ)
	}
	if path != "/app" {
		t.Errorf("expected path /app, got %s", path)
	}
}

func TestDetectAppType_Unknown(t *testing.T) {
	t.Parallel()
	fileExecutor := newMemFileExecutor(t)
	if err := fileExecutor.MkdirAll("/app", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	typ, _, err := detectAppType(fileExecutor, "/app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != appTypeUnknown {
		t.Errorf("expected appTypeUnknown, got %v", typ)
	}
}

func TestDetectAppType_Errors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		mock func(*mockhost.MockFileExecutor)
		want string
	}{
		{
			name: "kustomization lookup",
			mock: func(fileExecutor *mockhost.MockFileExecutor) {
				fileExecutor.EXPECT().Exists("/app/kustomization.yaml").Return(false, errors.New("boom"))
			},
			want: "failed to check kustomization.yaml: boom",
		},
		{
			name: "helmfile lookup",
			mock: func(fileExecutor *mockhost.MockFileExecutor) {
				fileExecutor.EXPECT().Exists("/app/kustomization.yaml").Return(false, nil)
				fileExecutor.EXPECT().Exists("/app/helmfile.yaml").Return(false, errors.New("boom"))
			},
			want: "failed to check helmfile.yaml: boom",
		},
		{
			name: "chart lookup",
			mock: func(fileExecutor *mockhost.MockFileExecutor) {
				fileExecutor.EXPECT().Exists("/app/kustomization.yaml").Return(false, nil)
				fileExecutor.EXPECT().Exists("/app/helmfile.yaml").Return(false, nil)
				fileExecutor.EXPECT().Exists("/app/helmfile.yaml.gotmpl").Return(false, nil)
				fileExecutor.EXPECT().Exists("/app/Chart.yaml").Return(false, errors.New("boom"))
			},
			want: "failed to check Chart.yaml: boom",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			fileExecutor := mockhost.NewMockFileExecutor(t)
			testCase.mock(fileExecutor)

			_, _, err := detectAppType(fileExecutor, "/app")
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("expected error containing %q, got %v", testCase.want, err)
			}
		})
	}
}

// ---- CreateApplicationCmd ---------------------------------------------------

func TestCreateApplicationCmd(t *testing.T) {
	t.Parallel()
	out := &bytes.Buffer{}
	cmd := CreateApplicationCmd(newMemFileExecutor(t), out)
	if cmd == nil {
		t.Fatal("CreateApplicationCmd returned nil")
	}
	if cmd.Use != "application" {
		t.Errorf("expected Use 'application', got %s", cmd.Use)
	}
	names := map[string]bool{}
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, expected := range []string{"validate", "render", "render-all"} {
		if !names[expected] {
			t.Errorf("expected subcommand %q not found", expected)
		}
	}
}

// ---- runValidateApp / runRenderApp error paths ------------------------------

func TestRunValidateApp_MissingDir(t *testing.T) {
	t.Parallel()
	err := runValidateApp(context.Background(), newMemFileExecutor(t), &bytes.Buffer{}, "/nonexistent", "")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestRunRenderApp_MissingDir(t *testing.T) {
	t.Parallel()
	err := runRenderApp(context.Background(), newMemFileExecutor(t), &bytes.Buffer{}, "/nonexistent", "")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestRunRenderApp_UnknownType(t *testing.T) {
	t.Parallel()
	fileExecutor := newMemFileExecutor(t)
	if err := fileExecutor.MkdirAll("/app", 0o755); err != nil {
		t.Fatal(err)
	}
	err := runRenderApp(context.Background(), fileExecutor, &bytes.Buffer{}, "/app", "")
	if err == nil {
		t.Error("expected error for directory with no recognized app definition")
	}
}

func TestRenderHelmfile_UsesExecutorAndContext(t *testing.T) {
	t.Parallel()

	assertRenderCommand(
		t,
		func(ctx context.Context, fileExecutor hostpkg.FileExecutor) (resmap.ResMap, error) {
			return renderHelmfile(ctx, fileExecutor, "/app/helmfile.yaml")
		},
		"helmfile",
		[]string{"template", "--skip-tests", "--args=--skip-crds", "-f", "/app/helmfile.yaml"},
	)
}

func TestRenderHelm_UsesExecutorAndContext(t *testing.T) {
	t.Parallel()

	assertRenderCommand(
		t,
		func(ctx context.Context, fileExecutor hostpkg.FileExecutor) (resmap.ResMap, error) {
			return renderHelm(ctx, fileExecutor, "/charts/example")
		},
		"helm",
		[]string{"template", "example", "/charts/example", "--skip-crds"},
	)
}

func TestRunCommandToResmap_Errors(t *testing.T) {
	t.Parallel()

	t.Run("command failure includes stderr", func(t *testing.T) {
		t.Parallel()

		fileExecutor := mockhost.NewMockFileExecutor(t)
		fileExecutor.EXPECT().RunCommand(mock.Anything, mock.Anything).Run(
			func(_ context.Context, options *hostpkg.CommandOptions) {
				if _, err := io.WriteString(options.Stderr, "stderr output"); err != nil {
					t.Fatalf("WriteString: %v", err)
				}
			},
		).Return(errors.New("failed"))

		_, err := runCommandToResmap(context.Background(), fileExecutor, &hostpkg.CommandOptions{Cmd: "helm"})
		if err == nil || !strings.Contains(err.Error(), "command helm failed: failed") ||
			!strings.Contains(err.Error(), "stderr output") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid YAML", func(t *testing.T) {
		t.Parallel()

		fileExecutor := mockhost.NewMockFileExecutor(t)
		fileExecutor.EXPECT().RunCommand(mock.Anything, mock.Anything).Run(
			func(_ context.Context, options *hostpkg.CommandOptions) {
				if _, err := io.WriteString(options.Stdout, "not valid kubernetes yaml"); err != nil {
					t.Fatalf("WriteString: %v", err)
				}
			},
		).Return(nil)

		_, err := runCommandToResmap(context.Background(), fileExecutor, &hostpkg.CommandOptions{Cmd: "helm"})
		if err == nil || !strings.Contains(err.Error(), "failed to create resmap from helm output") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestRenderApp_HelmCommandError(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), applicationContextKey("name"), "helm")
	fileExecutor := mockhost.NewMockFileExecutor(t)
	fileExecutor.EXPECT().Exists("/app/kustomization.yaml").Return(false, nil)
	fileExecutor.EXPECT().Exists("/app/helmfile.yaml").Return(false, nil)
	fileExecutor.EXPECT().Exists("/app/helmfile.yaml.gotmpl").Return(false, nil)
	fileExecutor.EXPECT().Exists("/app/Chart.yaml").Return(true, nil)
	fileExecutor.EXPECT().RunCommand(ctx, mock.Anything).Return(errors.New("helm failed"))

	_, err := renderApp(ctx, fileExecutor, "/app")
	if err == nil || !strings.Contains(err.Error(), "command helm failed: helm failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderApp_DetectTypeError(t *testing.T) {
	t.Parallel()

	fileExecutor := mockhost.NewMockFileExecutor(t)
	fileExecutor.EXPECT().Exists("/app/kustomization.yaml").Return(false, errors.New("boom"))

	_, err := renderApp(context.Background(), fileExecutor, "/app")
	if err == nil || !strings.Contains(err.Error(), "failed to check kustomization.yaml: boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderApp_HelmfilePath(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), applicationContextKey("name"), "helmfile-render")
	fileExecutor := mockhost.NewMockFileExecutor(t)
	fileExecutor.EXPECT().Exists("/app/kustomization.yaml").Return(false, nil)
	fileExecutor.EXPECT().Exists("/app/helmfile.yaml").Return(true, nil)
	fileExecutor.EXPECT().RunCommand(ctx, mock.Anything).Run(
		func(_ context.Context, options *hostpkg.CommandOptions) {
			if _, err := io.WriteString(options.Stdout, configMapContent); err != nil {
				t.Fatalf("WriteString: %v", err)
			}
		},
	).Return(nil)

	resources, err := renderApp(ctx, fileExecutor, "/app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlData, err := resources.AsYaml()
	if err != nil {
		t.Fatalf("AsYaml: %v", err)
	}
	if !bytes.Contains(yamlData, []byte("kind: ConfigMap")) {
		t.Fatalf("expected ConfigMap in output, got %s", string(yamlData))
	}
}

func TestRenderAppWithOutput_ErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("render error is wrapped", func(t *testing.T) {
		t.Parallel()

		err := renderAppWithOutput(context.Background(), newMemFileExecutor(t), io.Discard, "/app", "")
		if err == nil || !strings.Contains(err.Error(), "while rendering with output") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("split error is wrapped", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		fileExecutor := mockhost.NewMockFileExecutor(t)
		fileExecutor.EXPECT().Exists("/app/kustomization.yaml").Return(false, nil)
		fileExecutor.EXPECT().Exists("/app/helmfile.yaml").Return(false, nil)
		fileExecutor.EXPECT().Exists("/app/helmfile.yaml.gotmpl").Return(false, nil)
		fileExecutor.EXPECT().Exists("/app/Chart.yaml").Return(true, nil)
		fileExecutor.EXPECT().RunCommand(ctx, mock.Anything).Run(
			func(_ context.Context, options *hostpkg.CommandOptions) {
				if _, err := io.WriteString(options.Stdout, configMapContent); err != nil {
					t.Fatalf("WriteString: %v", err)
				}
			},
		).Return(nil)
		fileExecutor.EXPECT().MkdirAll("/dest", os.FileMode(0o755)).Return(nil)
		fileExecutor.EXPECT().WriteFile("/dest/ConfigMap-test-config.yaml", mock.Anything, os.FileMode(0o644)).
			Return(errors.New("write failed"))

		err := renderAppWithOutput(ctx, fileExecutor, io.Discard, "/app", "/dest")
		if err == nil || !strings.Contains(err.Error(), "failed to split resources to directory") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("write error is wrapped", func(t *testing.T) {
		t.Parallel()

		fileExecutor := newMemFileExecutor(t)
		writeKustomizeApp(t, fileExecutor, "/app")

		err := renderAppWithOutput(
			context.Background(),
			fileExecutor,
			errWriter{err: errors.New("write failed")},
			"/app",
			"",
		)
		if err == nil || !strings.Contains(err.Error(), "failed to write output: write failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("resource serialization error is wrapped", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		fileExecutor := mockhost.NewMockFileExecutor(t)
		fileExecutor.EXPECT().Exists("/app/kustomization.yaml").Return(false, nil)
		fileExecutor.EXPECT().Exists("/app/helmfile.yaml").Return(false, nil)
		fileExecutor.EXPECT().Exists("/app/helmfile.yaml.gotmpl").Return(false, nil)
		fileExecutor.EXPECT().Exists("/app/Chart.yaml").Return(true, nil)
		fileExecutor.EXPECT().RunCommand(ctx, mock.Anything).Run(
			func(_ context.Context, options *hostpkg.CommandOptions) {
				if _, err := io.WriteString(options.Stdout, `apiVersion: v1
kind: ConfigMap
metadata:
  name: bad-config
data:
  ? [a, b]
  : value
`); err != nil {
					t.Fatalf("WriteString: %v", err)
				}
			},
		).Return(nil)

		err := renderAppWithOutput(ctx, fileExecutor, io.Discard, "/app", "")
		if err == nil || !strings.Contains(err.Error(), "failed to convert resources to YAML") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestRunKubeconform_UsesExecutorAndContext(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), applicationContextKey("name"), "kubeconform")
	fileExecutor := mockhost.NewMockFileExecutor(t)
	expectedArgs := []string{
		"-schema-location",
		"default",
		"-schema-location",
		"/schemas/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json",
		"-schema-location",
		"https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/" +
			"{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json",
		"-schema-location",
		"https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/master/customresourcedefinition.json",
		"-summary",
	}
	fileExecutor.EXPECT().RunCommand(ctx, mock.Anything).Run(
		func(actualCtx context.Context, options *hostpkg.CommandOptions) {
			if actualCtx != ctx {
				t.Fatalf("expected context to be passed through unchanged")
			}
			if options.Cmd != "kubeconform" {
				t.Fatalf("expected kubeconform command, got %q", options.Cmd)
			}
			if !reflect.DeepEqual(options.Args, expectedArgs) {
				t.Fatalf("expected args %v, got %v", expectedArgs, options.Args)
			}
			stdinData, err := io.ReadAll(options.Stdin)
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}
			if !bytes.Contains(stdinData, []byte("kind: ConfigMap")) {
				t.Fatalf("expected ConfigMap in stdin, got: %s", string(stdinData))
			}
			if options.Stdout != os.Stderr {
				t.Fatal("expected stdout to use os.Stderr")
			}
			if options.Stderr != os.Stderr {
				t.Fatal("expected stderr to use os.Stderr")
			}
		},
	).Return(nil)

	err := runKubeconform(ctx, fileExecutor, mustResMapFromYAML(t, configMapContent), "/schemas")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunKubeconform_Errors(t *testing.T) {
	t.Parallel()

	t.Run("resource serialization failure", func(t *testing.T) {
		t.Parallel()

		err := runKubeconform(
			context.Background(),
			newMemFileExecutor(t),
			asYamlErrorResMap{ResMap: mustResMapFromYAML(t, configMapContent), err: errors.New("yaml failed")},
			"",
		)
		if err == nil || !strings.Contains(err.Error(), "failed to convert resources to YAML: yaml failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("command failure without local schema dir", func(t *testing.T) {
		t.Parallel()

		ctx := context.WithValue(context.Background(), applicationContextKey("name"), "kubeconform-error")
		fileExecutor := mockhost.NewMockFileExecutor(t)
		expectedArgs := []string{
			"-schema-location",
			"default",
			"-schema-location",
			"https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/" +
				"{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json",
			"-schema-location",
			"https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/master/customresourcedefinition.json",
			"-summary",
		}
		fileExecutor.EXPECT().RunCommand(ctx, mock.Anything).Run(
			func(actualCtx context.Context, options *hostpkg.CommandOptions) {
				if actualCtx != ctx {
					t.Fatalf("expected context to be passed through unchanged")
				}
				if !reflect.DeepEqual(options.Args, expectedArgs) {
					t.Fatalf("expected args %v, got %v", expectedArgs, options.Args)
				}
			},
		).Return(errors.New("command failed"))

		err := runKubeconform(ctx, fileExecutor, mustResMapFromYAML(t, configMapContent), "")
		if err == nil || !strings.Contains(err.Error(), "kubeconform validation failed: command failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// ---- runRenderApp with kustomize (integration) ------------------------------

func TestRunRenderApp_Kustomize_Stdout(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(tmpDir, "kustomization.yaml"),
		[]byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- configmap.yaml
`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "configmap.yaml"), []byte(configMapContent), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runRenderApp(context.Background(), hostpkg.NewDefaultHost(), &out, tmpDir, ""); err != nil {
		t.Fatalf("runRenderApp: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("ConfigMap")) {
		t.Errorf("expected ConfigMap in output, got: %s", out.String())
	}
}

func TestRunRenderApp_Kustomize_SplitFiles(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "out")
	if err := os.WriteFile(
		filepath.Join(tmpDir, "kustomization.yaml"),
		[]byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- configmap.yaml
`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "configmap.yaml"), []byte(configMapContent), 0o600); err != nil {
		t.Fatal(err)
	}

	fileExecutor := hostpkg.NewDefaultHost()
	if err := runRenderApp(context.Background(), fileExecutor, &bytes.Buffer{}, tmpDir, destDir); err != nil {
		t.Fatalf("runRenderApp: %v", err)
	}

	files, err := fileExecutor.ReadDir(destDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
	if files[0].Name() != "ConfigMap-test-config.yaml" {
		t.Errorf("unexpected filename: %s", files[0].Name())
	}
}

func TestRunValidateApp_DirExistsError(t *testing.T) {
	t.Parallel()

	fileExecutor := mockhost.NewMockFileExecutor(t)
	fileExecutor.EXPECT().DirExists("/app").Return(false, errors.New("stat failed"))

	err := runValidateApp(context.Background(), fileExecutor, io.Discard, "/app", "")
	if err == nil || !strings.Contains(err.Error(), "failed to check directory: stat failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunValidateApp_RenderError(t *testing.T) {
	t.Parallel()

	fileExecutor := newMemFileExecutor(t)
	if err := fileExecutor.MkdirAll("/app", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	err := runValidateApp(context.Background(), fileExecutor, io.Discard, "/app", "")
	if err == nil || !strings.Contains(err.Error(), "has no recognized app definition") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRenderApp_DirExistsError(t *testing.T) {
	t.Parallel()

	fileExecutor := mockhost.NewMockFileExecutor(t)
	fileExecutor.EXPECT().DirExists("/app").Return(false, errors.New("stat failed"))

	err := runRenderApp(context.Background(), fileExecutor, io.Discard, "/app", "")
	if err == nil || !strings.Contains(err.Error(), "failed to check directory: stat failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---- parseApplicationsFromDir -----------------------------------------------

func TestParseApplicationsFromDir(t *testing.T) {
	t.Parallel()
	fileExecutor := newMemFileExecutor(t)

	appYAML := `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
spec:
  source:
    path: deploy/k8s/argocd/e2e/my-app
`
	nonAppYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg
`
	writeFile(t, fileExecutor, "/manifests/Application-my-app.yaml", appYAML)
	writeFile(t, fileExecutor, "/manifests/ConfigMap-cfg.yaml", nonAppYAML)

	apps, err := parseApplicationsFromDir(fileExecutor, "/manifests")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0].Metadata.Name != "my-app" {
		t.Errorf("expected name 'my-app', got '%s'", apps[0].Metadata.Name)
	}
	if apps[0].Spec.Source.Path != "deploy/k8s/argocd/e2e/my-app" {
		t.Errorf("unexpected source path: %s", apps[0].Spec.Source.Path)
	}
}

func TestParseApplicationsFromDir_ErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("read directory failure", func(t *testing.T) {
		t.Parallel()

		fileExecutor := mockhost.NewMockFileExecutor(t)
		fileExecutor.EXPECT().ReadDir("/manifests").Return(nil, errors.New("read dir failed"))

		_, err := parseApplicationsFromDir(fileExecutor, "/manifests")
		if err == nil || !strings.Contains(err.Error(), "failed to read directory /manifests: read dir failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("read file failure", func(t *testing.T) {
		t.Parallel()

		fileExecutor := mockhost.NewMockFileExecutor(t)
		fileExecutor.EXPECT().ReadDir("/manifests").Return([]os.FileInfo{stubFileInfo{name: "bad.yaml"}}, nil)
		fileExecutor.EXPECT().ReadFile("/manifests/bad.yaml").Return(nil, errors.New("read file failed"))

		_, err := parseApplicationsFromDir(fileExecutor, "/manifests")
		if err == nil || !strings.Contains(err.Error(), "failed to read bad.yaml: read file failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid yaml is skipped", func(t *testing.T) {
		t.Parallel()

		fileExecutor := newMemFileExecutor(t)
		writeFile(t, fileExecutor, "/manifests/00-invalid.yaml", "kind: [")
		writeFile(t, fileExecutor, "/manifests/01-ignore.txt", "ignored")
		writeFile(t, fileExecutor, "/manifests/02-application.yaml", `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: valid-app
spec:
  source:
    path: apps/valid-app
`)

		apps, err := parseApplicationsFromDir(fileExecutor, "/manifests")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(apps) != 1 || apps[0].Metadata.Name != "valid-app" {
			t.Fatalf("unexpected apps: %+v", apps)
		}
	})
}

// ---- runRenderAll error paths -----------------------------------------------

func TestRunRenderAll_MissingAppstagesDir(t *testing.T) {
	t.Parallel()
	err := runRenderAll(context.Background(), newMemFileExecutor(t), &bytes.Buffer{}, "/nonexistent", "/dest", ".")
	if err == nil {
		t.Error("expected error for nonexistent appstages directory")
	}
}

func TestRunRenderAll_NoAppstages(t *testing.T) {
	t.Parallel()
	fileExecutor := newMemFileExecutor(t)
	if err := fileExecutor.MkdirAll("/appstages", 0o755); err != nil {
		t.Fatal(err)
	}
	err := runRenderAll(context.Background(), fileExecutor, &bytes.Buffer{}, "/appstages", "/dest", ".")
	if err == nil {
		t.Error("expected error when no appstage-* directories found")
	}
}

func TestRunRenderAll_EarlyErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		mock func(*mockhost.MockFileExecutor)
		want string
	}{
		{
			name: "dir exists failure",
			mock: func(fileExecutor *mockhost.MockFileExecutor) {
				fileExecutor.EXPECT().DirExists("/appstages").Return(false, errors.New("stat failed"))
			},
			want: "failed to check appstages directory: stat failed",
		},
		{
			name: "remove destination failure",
			mock: func(fileExecutor *mockhost.MockFileExecutor) {
				fileExecutor.EXPECT().DirExists("/appstages").Return(true, nil)
				fileExecutor.EXPECT().RemoveAll("/dest").Return(errors.New("remove failed"))
			},
			want: "failed to remove destination directory: remove failed",
		},
		{
			name: "mkdir destination failure",
			mock: func(fileExecutor *mockhost.MockFileExecutor) {
				fileExecutor.EXPECT().DirExists("/appstages").Return(true, nil)
				fileExecutor.EXPECT().RemoveAll("/dest").Return(nil)
				fileExecutor.EXPECT().MkdirAll("/dest", os.FileMode(0o755)).Return(errors.New("mkdir failed"))
			},
			want: "failed to create destination directory: mkdir failed",
		},
		{
			name: "read appstages failure",
			mock: func(fileExecutor *mockhost.MockFileExecutor) {
				fileExecutor.EXPECT().DirExists("/appstages").Return(true, nil)
				fileExecutor.EXPECT().RemoveAll("/dest").Return(nil)
				fileExecutor.EXPECT().MkdirAll("/dest", os.FileMode(0o755)).Return(nil)
				fileExecutor.EXPECT().ReadDir("/appstages").Return(nil, errors.New("read failed"))
			},
			want: "failed to read appstages directory: read failed",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			fileExecutor := mockhost.NewMockFileExecutor(t)
			testCase.mock(fileExecutor)

			err := runRenderAll(context.Background(), fileExecutor, io.Discard, "/appstages", "/dest", "/repo")
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("expected error containing %q, got %v", testCase.want, err)
			}
		})
	}
}

func TestRunRenderAll_LateErrors(t *testing.T) {
	t.Parallel()

	t.Run("appstage render failure", func(t *testing.T) {
		t.Parallel()

		host := newDummyFileExecutor(t, &testutil.DummyHostOptions{})
		writeFile(
			t,
			host,
			"/repo/appstages/appstage-dev/kustomization.yaml",
			`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- missing.yaml
`,
		)

		err := runRenderAll(context.Background(), host, io.Discard, "/repo/appstages", "/dest", "/repo")
		if err == nil || !strings.Contains(err.Error(), "failed to render appstage appstage-dev") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("write manifests failure", func(t *testing.T) {
		t.Parallel()

		baseHost := newDummyFileExecutor(t, &testutil.DummyHostOptions{})
		writeAppstageApplication(t, baseHost, "apps/app-one")
		fileExecutor := &interceptingFileExecutor{
			FileExecutor: baseHost,
			writeFileErrs: map[string]error{
				"/dest/appstage-dev/manifests/Application-app-one.yaml": errors.New("write failed"),
			},
		}

		err := runRenderAll(context.Background(), fileExecutor, io.Discard, "/repo/appstages", "/dest", "/repo")
		if err == nil || !strings.Contains(err.Error(), "failed to write manifests for appstage appstage-dev") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("parse applications failure", func(t *testing.T) {
		t.Parallel()

		baseHost := newDummyFileExecutor(t, &testutil.DummyHostOptions{})
		writeAppstageApplication(t, baseHost, "apps/app-one")
		fileExecutor := &interceptingFileExecutor{
			FileExecutor: baseHost,
			readDirErrs: map[string]error{
				"/dest/appstage-dev/manifests": errors.New("read failed"),
			},
		}

		err := runRenderAll(context.Background(), fileExecutor, io.Discard, "/repo/appstages", "/dest", "/repo")
		if err == nil || !strings.Contains(err.Error(), "failed to parse applications for appstage appstage-dev") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing application source path", func(t *testing.T) {
		t.Parallel()

		host := newDummyFileExecutor(t, &testutil.DummyHostOptions{})
		writeAppstageApplication(t, host, "")

		err := runRenderAll(context.Background(), host, io.Discard, "/repo/appstages", "/dest", "/repo")
		if err == nil || !strings.Contains(
			err.Error(),
			"application app-one in appstage appstage-dev has no spec.source.path",
		) {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("application render failure", func(t *testing.T) {
		t.Parallel()

		host := newDummyFileExecutor(t, &testutil.DummyHostOptions{})
		writeAppstageApplication(t, host, "apps/app-one")

		err := runRenderAll(context.Background(), host, io.Discard, "/repo/appstages", "/dest", "/repo")
		if err == nil || !strings.Contains(err.Error(), "failed to render application app-one") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestCreateApplicationCmd_ExecuteSubcommands(t *testing.T) {
	t.Parallel()

	t.Run("validate", func(t *testing.T) {
		t.Parallel()

		host := newDummyFileExecutor(t, &testutil.DummyHostOptions{
			FakeOutputs: map[string]*testutil.FakeProcessOutput{
				"^kubeconform .* -summary$": testutil.FakeExec("", 0),
			},
		})
		writeKustomizeApp(t, host, "/app")

		cmd := CreateApplicationCmd(host, &bytes.Buffer{})
		cmd.SetArgs([]string{"validate", "/app"})

		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("ExecuteContext: %v", err)
		}
	})

	t.Run("render with default host", func(t *testing.T) {
		t.Parallel()

		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}

		var out bytes.Buffer
		tmpDir := t.TempDir()
		if err := os.WriteFile(
			filepath.Join(tmpDir, "kustomization.yaml"),
			[]byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- configmap.yaml
`),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "configmap.yaml"), []byte(configMapContent), 0o600); err != nil {
			t.Fatal(err)
		}

		cmd := CreateApplicationCmd(nil, &out)
		cmd.SetArgs([]string{"render", tmpDir})

		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("ExecuteContext: %v", err)
		}
		if !bytes.Contains(out.Bytes(), []byte("ConfigMap")) {
			t.Fatalf("expected rendered output, got %s", out.String())
		}
	})

	t.Run("render-all", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer
		host := newDummyFileExecutor(t, &testutil.DummyHostOptions{
			FakeOutputs: map[string]*testutil.FakeProcessOutput{
				"^helm template app-one /repo/apps/app-one --skip-crds$": testutil.FakeExec(configMapContent, 0),
			},
		})
		writeAppstageApplication(t, host, "apps/app-one")
		writeHelmChart(t, host, "/repo/apps/app-one")

		cmd := CreateApplicationCmd(host, &out)
		cmd.SetArgs([]string{"render-all", "/repo/appstages", "/dest", "--base-dir", "/repo"})

		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("ExecuteContext: %v", err)
		}
		if !strings.Contains(out.String(), "Rendering appstage appstage-dev") ||
			!strings.Contains(out.String(), "Rendering application app-one from apps/app-one") {
			t.Fatalf("unexpected output: %s", out.String())
		}
		files, err := host.ReadDir("/dest/appstage-dev/applications/app-one")
		if err != nil {
			t.Fatalf("ReadDir: %v", err)
		}
		if len(files) != 1 || files[0].Name() != "ConfigMap-test-config.yaml" {
			t.Fatalf("unexpected rendered files: %+v", files)
		}
	})
}
