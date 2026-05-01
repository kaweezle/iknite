package check

// cSpell: words lipgloss
// cSpell: disable
import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	log "github.com/sirupsen/logrus"
)

// cSpell: enable

var _ tea.Model = (*CheckModel)(nil) // compile-time assertion that CheckModel implements tea.Model

type CheckModel struct {
	executor  *CheckExecutor
	checkData CheckData
	ctx       context.Context //nolint:containedctx // passed around to sub-checks
	cancel    context.CancelFunc
	spinner   spinner.Model
}

func (m *CheckModel) Init() tea.Cmd {
	go m.executor.Run(m.ctx, m.checkData)

	return tea.Batch(
		// tea.EnterAltScreen,
		tea.ClearScreen,
		m.spinner.Tick,
	)
}

func (m *CheckModel) Update(
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

func (m *CheckModel) View() string {
	var output strings.Builder
	for _, result := range m.executor.Results {
		output.WriteString(result.Format("", m.checkData, m.spinner.View()))
	}
	return output.String()
}

func (m *CheckModel) Context() context.Context {
	return m.ctx
}

func NewCheckModel(ctx context.Context, executor *CheckExecutor, checkData CheckData) *CheckModel {
	newCtx, cancel := context.WithCancel(ctx)
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return &CheckModel{
		executor:  executor,
		checkData: checkData,
		ctx:       newCtx,
		cancel:    cancel,
		spinner:   s,
	}
}
