// cSpell: words stretchr paralleltest stmts
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCoverageEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line      string
		name      string
		wantErr   bool
		wantExec  int
		wantStart int
	}{
		{
			name:      "valid entry",
			line:      "github.com/acme/proj/a.go:10.2,12.9 3 0",
			wantErr:   false,
			wantExec:  0,
			wantStart: 10,
		},
		{
			name:    "invalid entry",
			line:    "not-valid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			entry, err := parseCoverageEntry(tt.line, 7)
			if tt.wantErr {
				req.Error(err)
				return
			}
			req.NoError(err)
			req.Equal(tt.wantExec, entry.execCount)
			req.Equal(tt.wantStart, entry.startLine)
			req.Equal(3, entry.numStmts)
			req.Equal(3, entry.Len())
		})
	}
}

func TestReadModuleNameReadWriteLines(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	dir := t.TempDir()
	goMod := filepath.Join(dir, "go.mod")
	req.NoError(os.WriteFile(goMod, []byte("module github.com/acme/project\n\ngo 1.25\n"), 0o600))

	moduleName, err := readModuleName(goMod)
	req.NoError(err)
	req.Equal("github.com/acme/project", moduleName)

	out := filepath.Join(dir, "coverage.out")
	req.NoError(writeLines(out, []string{"mode: set", "a.go:1.1,1.2 1 0"}))
	lines, err := readLines(out)
	req.NoError(err)
	req.Equal([]string{"mode: set", "a.go:1.1,1.2 1 0"}, lines)
}

//nolint:paralleltest // tests change process working directory
func TestResolveSourcePathAndShouldIgnore(t *testing.T) {
	req := require.New(t)
	dir := t.TempDir()
	req.NoError(os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o600))

	oldWD, err := os.Getwd()
	req.NoError(err)
	req.NoError(os.Chdir(dir))
	t.Cleanup(func() {
		req.NoError(os.Chdir(oldWD))
	})

	source, rel, err := resolveSourcePath("github.com/acme/project", "github.com/acme/project/main.go")
	req.NoError(err)
	req.Equal("main.go", source)
	req.Equal("main.go", rel)

	entry := &coverageEntry{startLine: 3, endLine: 3}
	analysis := &fileAnalysis{nocovRanges: []lineRange{{start: 1, end: 5}}}
	req.True(shouldIgnore(entry, analysis))
	req.False(shouldIgnore(&coverageEntry{startLine: 6, endLine: 7}, analysis))
}

func TestBuildAnalysisAndPrintSummary(t *testing.T) {
	t.Parallel()

	req := require.New(t)
	dir := t.TempDir()

	content := `package main

func covered() int {
	return 1
}

// nocov: skip all from below
func skipped() int {
	return 2
}
`

	file := filepath.Join(dir, "sample.go")
	req.NoError(os.WriteFile(file, []byte(content), 0o600))

	analysis, err := buildAnalysis(file)
	req.NoError(err)
	req.NotEmpty(analysis.nocovRanges)

	buf := &bytes.Buffer{}
	printSummary(buf, map[string]int{"a.go": 2, "b.go": 3}, 5)
	out := buf.String()
	req.Contains(out, "ignored lines by file")
	req.Contains(out, "a.go: 2")
	req.Contains(out, "total ignored lines: 5")

	buf.Reset()
	printSummary(buf, map[string]int{}, 0)
	req.Equal("ignored lines: 0\n", buf.String())
}

func TestReadModuleNameErrors(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	dir := t.TempDir()
	goMod := filepath.Join(dir, "go.mod")
	req.NoError(os.WriteFile(goMod, []byte("go 1.25\n"), 0o600))

	_, err := readModuleName(goMod)
	req.Error(err)
	req.Contains(err.Error(), "module declaration not found")
}
