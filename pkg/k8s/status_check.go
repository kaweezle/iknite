package k8s

// cSpell: words termenv lipgloss
// cSpell: disable
import (
	"context"
	"fmt"

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

type CheckData any

type CheckDataBuilder func() CheckData

type CheckFn func(ctx context.Context, data CheckData) (bool, string, error)

type CustomResultPrinter func(result *CheckResult, prefix string, spinView string) string

type Check struct {
	CheckFn          CheckFn
	CheckDataBuilder CheckDataBuilder
	CustomPrinter    CustomResultPrinter
	Name             string
	Description      string
	Message          string
	DependsOn        []string
	SubChecks        []*Check
}

type CheckResult struct {
	Error         error
	CheckData     CheckData
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
	var subResults []*CheckResult
	for _, subCheck := range c.SubChecks {
		subResults = append(subResults, subCheck.NewResult())
	}

	var checkData CheckData
	if c.CheckDataBuilder != nil {
		checkData = c.CheckDataBuilder()
	}

	return &CheckResult{
		Check:      c,
		SubResults: subResults,
		Done:       make(chan struct{}),
		CheckData:  checkData,
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

func (c *CheckResult) CheckFn(ctx context.Context) *CheckResult {
	if c.Check.CheckFn != nil {
		success, message, err := c.Check.CheckFn(ctx, c.CheckData)
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

func (c *CheckResult) Run(ctx context.Context) {
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
				subResult.Run(ctx)
			}

			c.waitForSubChecks(ctx)
			return
		}

		// No subChecks, run the check function if available
		c.CheckFn(ctx)
	}()
}

func NewPhase(name, description string, subChecks []*Check) *Check {
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
	var results []*CheckResult
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
func RunChecks(ctx context.Context, results []*CheckResult) []*CheckResult {
	for _, result := range results {
		result.Run(ctx)
	}
	return results
}

// NewCheckExecutor creates a new CheckExecutor.
func NewCheckExecutor(checks []*Check) *CheckExecutor {
	e := &CheckExecutor{
		Checks:        checks,
		Results:       PrepareChecks(checks),
		resultNameMap: nil,
	}
	e.resultNameMap = FillResultNameMap(e.Results, e.resultNameMap)
	for _, result := range e.Results {
		result.FillDependencies(e.resultNameMap)
	}

	return e
}

var (
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))  // Green
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
	grayStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Gray
)

func (result *CheckResult) StatusString(spinView string) string {
	var status string
	var statusStyle lipgloss.Style

	switch result.Status {
	case StatusPending:
		status = "⋯"
		statusStyle = grayStyle
	case StatusRunning:
		return spinView
	case StatusSkipped:
		status = "⊝"
		statusStyle = grayStyle
	case StatusSuccess:
		status = "✓"
		statusStyle = successStyle
	case StatusFailed:
		status = "✗"
		statusStyle = errorStyle
	}

	return statusStyle.Render(status)
}

func (result *CheckResult) FormatResult(prefix string, spinView string) string {
	status := result.StatusString(spinView)

	description := result.Check.Description
	if result.Error != nil || result.Status == StatusFailed {
		description = errorStyle.Render(description)
	}

	output := fmt.Sprintf("%s%s %s", prefix, status, description)

	if result.Error != nil {
		output += fmt.Sprintf(" - %s", errorStyle.Render(result.Error.Error()))
	} else if result.Message != "" {
		output += fmt.Sprintf(" - %s", grayStyle.Render(result.Message))
	}
	output += "\n"

	if len(result.SubResults) > 0 {
		for _, subResult := range result.SubResults {
			output += subResult.Format(prefix+"  ", spinView)
		}
	}

	return output
}

func (result *CheckResult) Format(prefix string, spinView string) string {
	if result.Check.CustomPrinter != nil {
		return result.Check.CustomPrinter(result, prefix, spinView)
	}
	return result.FormatResult(prefix, spinView)
}

// Run runs the checks.
func (e *CheckExecutor) Run(ctx context.Context) []*CheckResult {
	// Start all top-level checks
	for _, result := range e.Results {
		result.Run(ctx)
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
