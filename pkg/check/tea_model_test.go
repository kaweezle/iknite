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

	executor := &check.CheckExecutor{Results: []*check.CheckResult{}}
	model := check.NewCheckModel(context.Background(), executor)

	cmd := model.Init()
	req.NotNil(cmd)
	req.NotNil(model.Context())
}

func TestCheckModelView(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	done := make(chan struct{})
	close(done)

	executor := &check.CheckExecutor{Results: []*check.CheckResult{{
		Check:   &check.Check{Name: "a", Description: "check-a"},
		Done:    done,
		Status:  check.StatusSuccess,
		Message: "ok",
	}}}
	model := check.NewCheckModel(context.Background(), executor)

	view := model.View()
	req.Contains(view, "check-a")
}

func TestCheckModelUpdateKeyCancelsContext(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	executor := &check.CheckExecutor{Results: []*check.CheckResult{}}
	model := check.NewCheckModel(context.Background(), executor)

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

		done := make(chan struct{})
		close(done)
		executor := &check.CheckExecutor{Results: []*check.CheckResult{{
			Check: &check.Check{Name: "a", Description: "a"},
			Done:  done,
		}}}
		model := check.NewCheckModel(context.Background(), executor)

		_, cmd := model.Update(struct{}{})
		req.NotNil(cmd)
		msg := cmd()
		_, ok := msg.(tea.QuitMsg)
		req.True(ok)
	})

	t.Run("running checks updates spinner", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		executor := &check.CheckExecutor{Results: []*check.CheckResult{{
			Check: &check.Check{Name: "a", Description: "a"},
			Done:  make(chan struct{}),
		}}}
		model := check.NewCheckModel(context.Background(), executor)

		_, cmd := model.Update(spinner.TickMsg{})
		req.NotNil(cmd)
	})
}
