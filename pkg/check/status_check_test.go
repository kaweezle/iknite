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

func TestCheckStatus_StringDefault(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	req.Equal("unknown", check.CheckStatus(999).String())
}

func TestNewPhase(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	sub := &check.Check{Name: "leaf", Description: "leaf"}
	phase := check.NewPhase("phase1", "First Phase", sub)

	req.Equal("phase1", phase.Name)
	req.Equal("First Phase", phase.Description)
	req.Len(phase.SubChecks, 1)
	req.Equal("leaf", phase.SubChecks[0].Name)
}

func TestCheckResult_WaitForDependencies_CtxDone(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	// parentDone is never closed so ctx.Done() is the only ready channel.
	parentDone := make(chan struct{})
	parentResult := &check.CheckResult{
		Check:  &check.Check{Name: "slow", Description: "slow"},
		Done:   parentDone,
		Status: check.StatusRunning,
	}

	childCheck := &check.Check{
		Name:        "child",
		Description: "child",
		DependsOn:   []string{"slow"},
		CheckFn: func(_ context.Context, _ check.CheckData) (bool, string, error) {
			return true, "", nil
		},
	}
	childResult := childCheck.NewResult(nil)
	childResult.ParentResults = []*check.CheckResult{parentResult}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Run so ctx.Done() fires immediately

	childResult.Run(ctx)
	<-childResult.Done

	// Status stays StatusPending: ctx canceled before dependency finished.
	req.Equal(check.StatusPending, childResult.Status)
}

func TestCheckResult_WaitForSubChecks_AllSuccess(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	phase := &check.Check{
		Name:        "phase",
		Description: "phase",
		SubChecks: []*check.Check{
			{
				Name:        "ok1",
				Description: "ok1",
				CheckFn: func(_ context.Context, _ check.CheckData) (bool, string, error) {
					return true, "done", nil
				},
			},
		},
	}

	result := phase.NewResult(nil)
	result.Run(context.Background())
	<-result.Done

	req.Equal(check.StatusSuccess, result.Status)
}

func TestCheckResult_WaitForSubChecks_CtxDone(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	// subRunning signals that the sub-check's CheckFn has started.
	subRunning := make(chan struct{})
	never := make(chan struct{}) // never closed, so sub-check blocks forever

	parent := &check.Check{
		Name:        "parent",
		Description: "parent",
		SubChecks: []*check.Check{
			{
				Name:        "never-ending",
				Description: "never-ending",
				CheckFn: func(_ context.Context, _ check.CheckData) (bool, string, error) {
					close(subRunning)
					<-never // blocks until test ends (goroutine leak is acceptable)
					return true, "", nil
				},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	parentResult := parent.NewResult(nil)
	parentResult.Run(ctx)

	<-subRunning // wait until sub-check is inside its CheckFn
	cancel()     // only ctx.Done() can fire; sub-check.Done never closes

	<-parentResult.Done
	// waitForSubChecks returned false via ctx.Done() without setting a new status.
	req.Equal(check.StatusRunning, parentResult.Status)
}

func TestCheckResult_FormatWithSubResults(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	done := make(chan struct{})
	close(done)

	subResult := &check.CheckResult{
		Check:  &check.Check{Name: "sub", Description: "sub-desc"},
		Done:   done,
		Status: check.StatusSuccess,
	}
	parentResult := &check.CheckResult{
		Check:      &check.Check{Name: "parent", Description: "parent-desc"},
		Done:       done,
		Status:     check.StatusSuccess,
		SubResults: []*check.CheckResult{subResult},
	}

	output := parentResult.FormatResult("", "-")
	req.Contains(output, "parent-desc")
	req.Contains(output, "sub-desc")
}

func TestCheckResult_FormatWithCustomPrinter(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	const customOutput = "custom-printer-output"
	c := &check.Check{
		Name:        "custom",
		Description: "custom-desc",
		CustomPrinter: func(_ *check.CheckResult, _, _ string) string {
			return customOutput
		},
	}

	result := c.NewResult(nil)
	result.Status = check.StatusSuccess

	req.Equal(customOutput, result.Format("", "-"))
}

func TestCheckExecutor_Run_CtxCancellation(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	// checkRunning signals that the check's CheckFn is active.
	// never ensures the check never finishes, so ctx.Done() wins in executor.Run.
	checkRunning := make(chan struct{})
	never := make(chan struct{})

	blocker := &check.Check{
		Name:        "blocker",
		Description: "blocker",
		CheckFn: func(_ context.Context, _ check.CheckData) (bool, string, error) {
			close(checkRunning)
			<-never // blocks until test ends (goroutine leak is acceptable)
			return true, "", nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	executor := check.NewCheckExecutor([]*check.Check{blocker}, nil)

	runDone := make(chan []*check.CheckResult, 1)
	go func() {
		runDone <- executor.Run(ctx)
	}()

	<-checkRunning // check is running; its Done will never close
	cancel()       // trigger ctx.Done() path in executor.Run

	results := <-runDone
	req.Len(results, 1)
}

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
