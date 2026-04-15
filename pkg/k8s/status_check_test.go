// cSpell: words paralleltest testpackage stretchr
package k8s_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/k8s"
)

func TestCheck_NewResultAndFillDependencies(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	checks := []*k8s.Check{
		{Name: "root", Description: "root", SubChecks: []*k8s.Check{{Name: "leaf", Description: "leaf"}}},
		{Name: "dependent", Description: "dep", DependsOn: []string{"leaf"}},
	}

	results := k8s.PrepareChecks(checks)
	req.Len(results, 2)
	req.Equal("leaf", results[0].SubResults[0].Name())
	req.NotNil(results[0].Done)

	nameMap := k8s.FillResultNameMap(results, nil)
	req.Contains(nameMap, "root")
	req.Contains(nameMap, "leaf")
	req.Contains(nameMap, "dependent")

	req.Len(results[1].ParentResults, 1)
	req.Equal("leaf", results[1].ParentResults[0].Name())
}

func TestCheckResult_CheckFn(t *testing.T) {
	t.Parallel()
	tests := []struct {
		check     *k8s.Check
		name      string
		wantError bool
		wantOK    bool
	}{
		{
			name: "success result",
			check: &k8s.Check{
				Name:        "ok",
				Description: "ok",
				CheckFn: func(_ context.Context, _ k8s.CheckData) (bool, string, error) {
					return true, "done", nil
				},
			},
			wantError: false,
			wantOK:    true,
		},
		{
			name: "failed result",
			check: &k8s.Check{
				Name:        "failed",
				Description: "failed",
				CheckFn: func(_ context.Context, _ k8s.CheckData) (bool, string, error) {
					return false, "bad", errors.New("boom")
				},
			},
			wantError: true,
			wantOK:    false,
		},
		{
			name:      "missing check function",
			check:     &k8s.Check{Name: "missing", Description: "missing"},
			wantError: true,
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			result := tt.check.NewResult().CheckFn(context.Background())
			if tt.wantError {
				req.Error(result.Error)
			} else {
				req.NoError(result.Error)
			}
			req.Equal(tt.wantOK, result.Success())
		})
	}
}

func TestCheckResult_RunAndDependencies(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	parent := &k8s.Check{
		Name:        "parent",
		Description: "parent",
		CheckFn: func(_ context.Context, _ k8s.CheckData) (bool, string, error) {
			return false, "", errors.New("no")
		},
	}
	child := &k8s.Check{
		Name:        "child",
		Description: "child",
		DependsOn:   []string{"parent"},
		CheckFn: func(_ context.Context, _ k8s.CheckData) (bool, string, error) {
			return true, "", nil
		},
	}

	results := k8s.PrepareChecks([]*k8s.Check{parent, child})
	k8s.RunChecks(context.Background(), results)

	for _, result := range results {
		<-result.Done
	}

	req.Equal(k8s.StatusFailed, results[0].Status)
	req.Equal(k8s.StatusSkipped, results[1].Status)
	req.Contains(results[1].Message, "Skipped due to failure")
}

func TestCheckResult_RunWithSubChecks(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	check := &k8s.Check{
		Name:        "phase",
		Description: "phase",
		SubChecks: []*k8s.Check{
			{
				Name:        "ok",
				Description: "ok",
				CheckFn:     func(_ context.Context, _ k8s.CheckData) (bool, string, error) { return true, "", nil },
			},
			{
				Name:        "ko",
				Description: "ko",
				CheckFn:     func(_ context.Context, _ k8s.CheckData) (bool, string, error) { return false, "", errors.New("bad") },
			},
		},
	}

	result := check.NewResult()
	result.Run(context.Background())
	<-result.Done

	req.Equal(k8s.StatusFailed, result.Status)
	req.Error(result.Error)
	req.Contains(result.Error.Error(), "subChecks failed")
}

func TestFormattingHelpers(t *testing.T) {
	t.Parallel()

	statuses := []k8s.CheckStatus{
		k8s.StatusPending,
		k8s.StatusRunning,
		k8s.StatusSkipped,
		k8s.StatusSuccess,
		k8s.StatusFailed,
	}
	for _, st := range statuses {
		t.Run(st.String(), func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			result := (&k8s.Check{Name: "name", Description: "desc"}).NewResult()
			result.Status = st
			statusText := result.StatusString("spinner")
			req.NotEmpty(statusText)
		})
	}

	result := (&k8s.Check{Name: "name", Description: "desc"}).NewResult()
	req := require.New(t)

	result.Status = k8s.StatusSuccess
	result.Message = "all good"
	req.Contains(result.FormatResult("", "-"), "all good")

	result.Status = k8s.StatusFailed
	result.Error = errors.New("broken")
	req.Contains(result.Format("", "-"), "broken")
}

func TestCheckExecutor_Run(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	checks := []*k8s.Check{
		{
			Name:        "a",
			Description: "a",
			CheckFn:     func(_ context.Context, _ k8s.CheckData) (bool, string, error) { return true, "", nil },
		},
		{
			Name:        "b",
			Description: "b",
			CheckFn:     func(_ context.Context, _ k8s.CheckData) (bool, string, error) { return true, "", nil },
		},
	}

	executor := k8s.NewCheckExecutor(checks)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	results := executor.Run(ctx)
	req.Len(results, 2)
	for _, result := range results {
		req.Equal(k8s.StatusSuccess, result.Status)
	}
}
