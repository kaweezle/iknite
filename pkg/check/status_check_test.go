// cSpell: words paralleltest testpackage
package check_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/check"
)

func TestCheck_NewResultAndFillDependencies(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	checks := []*check.Check{
		{Name: "root", Description: "root", SubChecks: []*check.Check{{Name: "leaf", Description: "leaf"}}},
		{Name: "dependent", Description: "dep", DependsOn: []string{"leaf"}},
	}

	results := check.PrepareChecks(checks, nil)
	req.Len(results, 2)
	req.Equal("leaf", results[0].SubResults[0].Name())
	req.NotNil(results[0].Done)

	nameMap := check.FillResultNameMap(results, nil)
	req.Contains(nameMap, "root")
	req.Contains(nameMap, "leaf")
	req.Contains(nameMap, "dependent")

	req.Len(results[1].ParentResults, 1)
	req.Equal("leaf", results[1].ParentResults[0].Name())
}

func TestCheckResult_CheckFn(t *testing.T) {
	t.Parallel()
	tests := []struct {
		check     *check.Check
		name      string
		wantError bool
		wantOK    bool
	}{
		{
			name: "success result",
			check: &check.Check{
				Name:        "ok",
				Description: "ok",
				CheckFn: func(_ context.Context, _ check.CheckData) (bool, string, error) {
					return true, "done", nil
				},
			},
			wantError: false,
			wantOK:    true,
		},
		{
			name: "failed result",
			check: &check.Check{
				Name:        "failed",
				Description: "failed",
				CheckFn: func(_ context.Context, _ check.CheckData) (bool, string, error) {
					return false, "bad", errors.New("boom")
				},
			},
			wantError: true,
			wantOK:    false,
		},
		{
			name:      "missing check function",
			check:     &check.Check{Name: "missing", Description: "missing"},
			wantError: true,
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			result := tt.check.NewResult(nil).CheckFn(context.Background())
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

	parent := &check.Check{
		Name:        "parent",
		Description: "parent",
		CheckFn: func(_ context.Context, _ check.CheckData) (bool, string, error) {
			return false, "", errors.New("no")
		},
	}
	child := &check.Check{
		Name:        "child",
		Description: "child",
		DependsOn:   []string{"parent"},
		CheckFn: func(_ context.Context, _ check.CheckData) (bool, string, error) {
			return true, "", nil
		},
	}

	results := check.PrepareChecks([]*check.Check{parent, child}, nil)
	check.RunChecks(context.Background(), results)

	for _, result := range results {
		<-result.Done
	}

	req.Equal(check.StatusFailed, results[0].Status)
	req.Equal(check.StatusSkipped, results[1].Status)
	req.Contains(results[1].Message, "Skipped due to failure")
}

func TestCheckResult_RunWithSubChecks(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	c := &check.Check{
		Name:        "phase",
		Description: "phase",
		SubChecks: []*check.Check{
			{
				Name:        "ok",
				Description: "ok",
				CheckFn: func(_ context.Context, _ check.CheckData) (bool, string, error) {
					return true, "", nil
				},
			},
			{
				Name:        "ko",
				Description: "ko",
				CheckFn: func(_ context.Context, _ check.CheckData) (bool, string, error) {
					return false, "", errors.New("bad")
				},
			},
		},
	}

	result := c.NewResult(nil)
	result.Run(context.Background())
	<-result.Done

	req.Equal(check.StatusFailed, result.Status)
	req.Error(result.Error)
	req.Contains(result.Error.Error(), "subChecks failed")
}

func TestFormattingHelpers(t *testing.T) {
	t.Parallel()

	statuses := []check.CheckStatus{
		check.StatusPending,
		check.StatusRunning,
		check.StatusSkipped,
		check.StatusSuccess,
		check.StatusFailed,
	}
	for _, st := range statuses {
		t.Run(st.String(), func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			result := (&check.Check{Name: "name", Description: "desc"}).NewResult(nil)
			result.Status = st
			statusText := result.StatusString("spinner")
			req.NotEmpty(statusText)
		})
	}

	result := (&check.Check{Name: "name", Description: "desc"}).NewResult(nil)
	req := require.New(t)

	result.Status = check.StatusSuccess
	result.Message = "all good"
	req.Contains(result.FormatResult("", "-"), "all good")

	result.Status = check.StatusFailed
	result.Error = errors.New("broken")
	req.Contains(result.Format("", "-"), "broken")
}

func TestCheckExecutor_Run(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	checks := []*check.Check{
		{
			Name:        "a",
			Description: "a",
			CheckFn:     func(_ context.Context, _ check.CheckData) (bool, string, error) { return true, "", nil },
		},
		{
			Name:        "b",
			Description: "b",
			CheckFn:     func(_ context.Context, _ check.CheckData) (bool, string, error) { return true, "", nil },
		},
	}

	executor := check.NewCheckExecutor(checks, nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	results := executor.Run(ctx)
	req.Len(results, 2)
	for _, result := range results {
		req.Equal(check.StatusSuccess, result.Status)
	}
}
