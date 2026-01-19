package k8s

// cSpell: words lipgloss
// cSpell: disable
import (
	"context"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	log "github.com/sirupsen/logrus"
)

// cSpell: enable

type CheckModel struct {
	executor *CheckExecutor
	ctx      context.Context //nolint:containedctx // passed around to sub-checks
	cancel   context.CancelFunc
	spinner  spinner.Model
}

func (m CheckModel) Init() tea.Cmd { //nolint:gocritic // Implements tea.Model
	go m.executor.Run(m.ctx)

	return tea.Batch(
		// tea.EnterAltScreen,
		tea.ClearScreen,
		m.spinner.Tick,
	)
}

func (m CheckModel) Update( //nolint:gocritic // Implements tea.Model
	msg tea.Msg,
) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		log.Infof("Canceling checks %v", m.cancel)
		m.cancel()
		return m, tea.Batch(
			// tea.ExitAltScreen,
			tea.Quit,
		)
	default:
		// Check if all done
		allDone := true
		for _, result := range m.executor.Results {
			select {
			case <-result.Done:
				continue
			default:
				allDone = false
			}
		}
		if allDone {
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m CheckModel) View() string { //nolint:gocritic // Implements tea.Model
	var output string
	for _, result := range m.executor.Results {
		output += result.Format("", m.spinner.View())
	}
	return output
}

func NewCheckModel(ctx context.Context, executor *CheckExecutor) CheckModel {
	newCtx, cancel := context.WithCancel(ctx)
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return CheckModel{
		executor: executor,
		ctx:      newCtx,
		cancel:   cancel,
		spinner:  s,
	}
}
