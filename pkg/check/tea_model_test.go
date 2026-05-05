// cSpell: words charmbracelet bubbletea
package check_test

import (
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/check"
)

func TestCheckModelInitAndView(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	executor := check.NewCheckExecutor()
	model := check.NewCheckModel(context.Background(), executor, nil)

	cmd := model.Init()
	req.NotNil(cmd)
	req.NotNil(model.Context())
}

func TestCheckModelView(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	executor := check.NewCheckExecutor()
	executor.AddCheck(&check.Check{
		Name:        "check-a",
		Description: "desc-a",
		CheckFn: func(_ context.Context, _ check.CheckData) (bool, string, error) {
			return true, "ok", nil
		},
	})
	executor.PrepareRun()

	model := check.NewCheckModel(t.Context(), executor, nil)
	view := model.View()
	req.Contains(view, "desc-a")
}

func TestCheckModelUpdateKeyCancelsContext(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	executor := check.NewCheckExecutor()
	executor.AddCheck(&check.Check{
		Name:        "check-a",
		Description: "desc-a",
		CheckFn: func(_ context.Context, _ check.CheckData) (bool, string, error) {
			return true, "ok", nil
		},
	})
	executor.PrepareRun()

	model := check.NewCheckModel(context.Background(), executor, nil)

	_, cmd := model.Update(tea.KeyMsg{})
	req.NotNil(cmd)

	select {
	case <-model.Context().Done():
	default:
		t.Fatal("expected model context to be canceled")
	}

	msg := cmd()
	_, ok := msg.(tea.QuitMsg)
	req.True(ok)
}

func TestCheckModelUpdateDefaultBranches(t *testing.T) {
	t.Parallel()

	t.Run("all checks done returns quit", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		executor := check.NewCheckExecutor()
		executor.AddCheck(&check.Check{
			Name:        "a",
			Description: "a",
			CheckFn: func(_ context.Context, _ check.CheckData) (bool, string, error) {
				return true, "ok", nil
			},
		})
		executor.PrepareRun()
		res := executor.Run(t.Context(), nil)
		req.NotNil(res)
		req.Len(res, 1)

		model := check.NewCheckModel(t.Context(), executor, nil)

		_, cmd := model.Update(struct{}{})
		req.NotNil(cmd)
		msg := cmd()
		_, ok := msg.(tea.QuitMsg)
		req.True(ok)
	})

	t.Run("running checks updates spinner", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		executor := check.NewCheckExecutor()
		executor.AddCheck(&check.Check{
			Name:        "a",
			Description: "a",
			CheckFn: func(_ context.Context, _ check.CheckData) (bool, string, error) {
				return true, "ok", nil
			},
		})
		executor.PrepareRun()
		model := check.NewCheckModel(t.Context(), executor, nil)

		_, cmd := model.Update(spinner.TickMsg{})
		req.NotNil(cmd)
	})
}
