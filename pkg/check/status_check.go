package check

// cSpell: words termenv lipgloss
// cSpell: disable
import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// cSpell: enable

type CheckStatus int

const (
	StatusPending CheckStatus = iota
	StatusRunning
	StatusSkipped
	StatusSuccess
	StatusFailed
)

func (s CheckStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusSkipped:
		return "skipped"
	case StatusSuccess:
		return "success"
	case StatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

type CheckData any

type CheckDataBuilder func() CheckData

type CheckFn func(ctx context.Context, data CheckData) (bool, string, error)

type CustomResultPrinter func(result *CheckResult, data CheckData, prefix string, spinView string) string

type Check struct {
	CheckFn       CheckFn
	CustomPrinter CustomResultPrinter
	Name          string
	Description   string
	Message       string
	DependsOn     []string
	SubChecks     []*Check
}

type CheckResult struct {
	Error         error
	Check         *Check
	Done          chan struct{}
	Message       string
	SubResults    []*CheckResult
	ParentResults []*CheckResult
	Status        CheckStatus
}

type CheckExecutor struct {
	resultNameMap map[string]*CheckResult
	Checks        []*Check
	Results       []*CheckResult
}

func (c *Check) NewResult() *CheckResult {
	subResults := make([]*CheckResult, 0, len(c.SubChecks))
	for _, subCheck := range c.SubChecks {
		subResults = append(subResults, subCheck.NewResult())
	}

	return &CheckResult{
		Check:      c,
		SubResults: subResults,
		Done:       make(chan struct{}),
	}
}

func (r *CheckResult) Name() string {
	return r.Check.Name
}

func (r *CheckResult) Success() bool {
	return r.Status == StatusSuccess
}

func (r *CheckResult) Failed() bool {
	return r.Status == StatusFailed
}

func (r *CheckResult) FillDependencies(resultNameMap map[string]*CheckResult) {
	for _, depName := range r.Check.DependsOn {
		if depResult, ok := resultNameMap[depName]; ok {
			r.ParentResults = append(r.ParentResults, depResult)
		}
	}

	for _, subResult := range r.SubResults {
		subResult.FillDependencies(resultNameMap)
	}
}

func (c *CheckResult) CheckFn(ctx context.Context, checkData CheckData) *CheckResult {
	if c.Check.CheckFn != nil {
		success, message, err := c.Check.CheckFn(ctx, checkData)
		c.Message = message
		c.Error = err
		if success {
			c.Status = StatusSuccess
		} else {
			c.Status = StatusFailed
		}
		return c
	} else {
		// No check function defined
		c.Error = fmt.Errorf("no check function defined for %s", c.Name())
		c.Status = StatusFailed
	}

	return c
}

func (c *CheckResult) waitForDependencies(ctx context.Context) bool {
	for _, parent := range c.ParentResults {
		select {
		case <-ctx.Done():
			return false
		case <-parent.Done:
			if !parent.Success() {
				c.Status = StatusSkipped
				c.Message = fmt.Sprintf("Skipped due to failure of %s", parent.Name())
				return false
			}
		}
	}
	return true
}

func (c *CheckResult) waitForSubChecks(ctx context.Context) bool {
	result := true
	errors := []string{}
	for _, subResult := range c.SubResults {
		select {
		case <-ctx.Done():
			return false
		case <-subResult.Done:
			if subResult.Failed() {
				errors = append(errors, subResult.Name())
				result = false
			}
		}
	}
	if !result {
		c.Status = StatusFailed
		c.Error = fmt.Errorf(" %d subChecks failed", len(errors))
	} else {
		c.Status = StatusSuccess
	}

	return result
}

func (c *CheckResult) Run(ctx context.Context, checkData CheckData) {
	go func() {
		defer close(c.Done)

		c.Status = StatusPending

		// Wait for dependencies
		if !c.waitForDependencies(ctx) {
			return
		}

		c.Status = StatusRunning

		// If check has subChecks, run them all concurrently
		if len(c.SubResults) > 0 {
			for _, subResult := range c.SubResults {
				subResult.Run(ctx, checkData)
			}

			c.waitForSubChecks(ctx)
			return
		}

		// No subChecks, run the check function if available
		c.CheckFn(ctx, checkData)
	}()
}

func NewPhase(name, description string, subChecks ...*Check) *Check {
	return &Check{
		Name:        name,
		Description: description,
		SubChecks:   subChecks,
	}
}

// FillResultNameMap fills a map with check results by name.
func FillResultNameMap(
	results []*CheckResult,
	resultNameMap map[string]*CheckResult,
) map[string]*CheckResult {
	if resultNameMap == nil {
		resultNameMap = make(map[string]*CheckResult)
	}
	for _, result := range results {
		resultNameMap[result.Name()] = result
		if len(result.SubResults) > 0 {
			FillResultNameMap(result.SubResults, resultNameMap)
		}
	}
	return resultNameMap
}

// PrepareChecks prepares the checks for running.
func PrepareChecks(checks []*Check) []*CheckResult {
	results := make([]*CheckResult, 0, len(checks))
	for _, check := range checks {
		results = append(results, check.NewResult())
	}

	resultNameMap := FillResultNameMap(results, nil)
	for _, result := range results {
		result.FillDependencies(resultNameMap)
	}

	return results
}

// RunChecks runs the checks.
func RunChecks(ctx context.Context, results []*CheckResult, checkData CheckData) []*CheckResult {
	for _, result := range results {
		result.Run(ctx, checkData)
	}
	return results
}

// NewCheckExecutor creates a new CheckExecutor.
func NewCheckExecutor() *CheckExecutor {
	return &CheckExecutor{}
}

func (e *CheckExecutor) PrepareRun() {
	e.Results = PrepareChecks(e.Checks)
	e.resultNameMap = FillResultNameMap(e.Results, e.resultNameMap)
	for _, result := range e.Results {
		result.FillDependencies(e.resultNameMap)
	}
}

func (e *CheckExecutor) AddCheck(check ...*Check) {
	e.Checks = append(e.Checks, check...)
}

var (
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))  // Green
	ErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
	GrayStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Gray
)

func (result *CheckResult) StatusString(spinView string) string {
	var status string
	var statusStyle lipgloss.Style

	switch result.Status {
	case StatusPending:
		status = "⋯"
		statusStyle = GrayStyle
	case StatusRunning:
		return spinView
	case StatusSkipped:
		status = "⊝"
		statusStyle = GrayStyle
	case StatusSuccess:
		status = "✓"
		statusStyle = SuccessStyle
	case StatusFailed:
		status = "✗"
		statusStyle = ErrorStyle
	}

	return statusStyle.Render(status)
}

func (result *CheckResult) FormatResult(prefix string, checkData CheckData, spinView string) string {
	status := result.StatusString(spinView)

	description := result.Check.Description
	if result.Error != nil || result.Status == StatusFailed {
		description = ErrorStyle.Render(description)
	}

	var output strings.Builder
	fmt.Fprintf(&output, "%s%s %s", prefix, status, description)

	if result.Error != nil {
		fmt.Fprintf(&output, " - %s", ErrorStyle.Render(result.Error.Error()))
	} else if result.Message != "" {
		fmt.Fprintf(&output, " - %s", GrayStyle.Render(result.Message))
	}
	output.WriteString("\n")

	if len(result.SubResults) > 0 {
		for _, subResult := range result.SubResults {
			output.WriteString(subResult.Format(prefix+"  ", checkData, spinView))
		}
	}

	return output.String()
}

func (result *CheckResult) Format(prefix string, checkData CheckData, spinView string) string {
	if result.Check.CustomPrinter != nil {
		return result.Check.CustomPrinter(result, checkData, prefix, spinView)
	}
	return result.FormatResult(prefix, checkData, spinView)
}

// Run runs the checks.
func (e *CheckExecutor) Run(ctx context.Context, checkData CheckData) []*CheckResult {
	// Start all top-level checks
	for _, result := range e.Results {
		result.Run(ctx, checkData)
	}

	allDone := make(chan struct{})
	go func() {
		for _, result := range e.Results {
			<-result.Done
		}
		close(allDone)
	}()

	for {
		select {
		case <-ctx.Done():
			return e.Results
		case <-allDone:
			return e.Results
		}
	}
}
