package k8s

// cSpell: words termenv
import (
	"context"
	"fmt"
	"time"

	"github.com/muesli/termenv"
)

type CheckStatus int

const (
	StatusPending CheckStatus = iota
	StatusRunning
	StatusSkipped
	StatusSuccess
	StatusFailed
)

type CheckData = interface{}

type CheckDataBuilder func() (CheckData, error)

type CheckFn func(ctx context.Context, data CheckData) (bool, string, error)

type CustomResultPrinter func(result *CheckResult, prefix string, output *termenv.Output)

type Check struct {
	Name             string
	Description      string
	Message          string
	DependsOn        []string
	SubChecks        []*Check
	CheckFn          CheckFn
	CheckDataBuilder CheckDataBuilder
	CustomPrinter    CustomResultPrinter
}

type CheckResult struct {
	Check         *Check
	Status        CheckStatus
	Message       string
	Error         error
	SubResults    []*CheckResult
	ParentResults []*CheckResult
	Done          chan struct{}
	CheckData     CheckData
}

type CheckExecutor struct {
	Checks  []*Check
	Results []*CheckResult

	resultNameMap map[string]*CheckResult
}

func (c *Check) NewResult() *CheckResult {
	var subResults []*CheckResult
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
		if c.Check.CheckDataBuilder != nil {
			data, err := c.Check.CheckDataBuilder()
			if err != nil {
				c.Status = StatusFailed
				c.Error = err
				return
			}
			c.CheckData = data
		}

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

// FillResultNameMap fills a map with check results by name
func FillResultNameMap(results []*CheckResult, resultNameMap map[string]*CheckResult) map[string]*CheckResult {
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

// PrepareChecks prepares the checks for running
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

// RunChecks runs the checks
func RunChecks(ctx context.Context, results []*CheckResult) []*CheckResult {
	for _, result := range results {
		result.Run(ctx)
	}
	return results
}

// NewCheckExecutor creates a new CheckExecutor
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

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (result *CheckResult) PrettyStatus(output *termenv.Output) termenv.Style {
	var status string
	var statusColor termenv.Style
	p := output.Profile

	switch result.Status {
	case StatusPending:
		status = "..."
		statusColor = output.String(status).Foreground(p.Color("240")) // Gray
	case StatusRunning:
		frame := int((time.Now().UnixNano() / int64(time.Millisecond) / 250) % int64(len(spinnerFrames)))
		status = spinnerFrames[frame]
		statusColor = output.String(status).Foreground(p.Color("240")) // Gray
	case StatusSkipped:
		status = "⊝"
		statusColor = output.String(status).Foreground(p.Color("240")) // Gray
	case StatusSuccess:
		status = "✓"
		statusColor = output.String(status).Foreground(p.Color("46")) // Green
	case StatusFailed:
		status = "✗"
		statusColor = output.String(status).Foreground(p.Color("196")) // Red
	}

	return statusColor
}

func (result *CheckResult) PrettyPrint(prefix string, output *termenv.Output) {
	var statusColor termenv.Style
	p := output.Profile

	statusColor = result.PrettyStatus(output)

	description := output.String(result.Check.Description)
	if result.Error != nil || result.Status == StatusFailed {
		description = description.Foreground(p.Color("203")) // Light red
	}

	output.WriteString(fmt.Sprintf("%s%s %s", prefix, statusColor, description))

	if result.Error != nil {
		output.WriteString(fmt.Sprintf(" - %s", output.String(result.Error.Error()).Foreground(p.Color("203"))))
	} else if result.Message != "" {
		output.WriteString(fmt.Sprintf(" - %s", output.String(result.Message).Foreground(p.Color("240"))))
	}
	output.WriteString("\n")

	if len(result.SubResults) > 0 {
		for _, subResult := range result.SubResults {
			subResult.Print(prefix+"  ", output)
		}
	}
}

func (result *CheckResult) Print(prefix string, output *termenv.Output) {
	// Use custom printer if available
	if result.Check.CustomPrinter != nil {
		result.Check.CustomPrinter(result, prefix, output)
		return
	}
	result.PrettyPrint(prefix, output)
}

// PrintResults prints the results
func (e *CheckExecutor) PrintResults(output *termenv.Output) {
	output.ClearScreen()
	for _, result := range e.Results {
		result.Print("", output)
	}
}

// Run runs the checks
func (e *CheckExecutor) Run(ctx context.Context, output *termenv.Output) []*CheckResult {
	// Start all top-level checks
	for _, result := range e.Results {
		result.Run(ctx)
	}

	// Print status every 250ms until all checks complete
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

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
			e.PrintResults(output)
			return e.Results
		case <-ticker.C:
			e.PrintResults(output)
		}
	}
}
