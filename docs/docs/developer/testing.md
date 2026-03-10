!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Testing

This page describes how to run and write tests for Iknite.

## Running Tests

### Run All Tests

```bash
go test ./...
```

### Run with Coverage

```bash
go test -v -race -covermode=atomic -coverprofile=coverage.out ./...

# View coverage report in browser
go tool cover -html=coverage.out
```

### Run Tests for a Specific Package

```bash
# Test a specific package
go test ./pkg/k8s/...

# Test with verbose output
go test -v ./pkg/alpine/...

# Run a specific test
go test -run TestPreventKubeletServiceFromStarting ./pkg/k8s/...
```

### Run Tests with Race Detection

```bash
go test -race ./...
```

Race detection is always enabled in CI. Use it locally to catch concurrency issues.

## Test Structure

Tests follow Go conventions:

- Test files end with `_test.go`
- Tests live in the same package as the code they test
- Test names follow `Test_functionName_scenario` or `Test_functionName`
- Complex test suites use `testify/suite`

### Example Test File

```go
package k8s_test

import (
    "testing"

    "github.com/stretchr/testify/require"
    "github.com/kaweezle/iknite/pkg/k8s"
    tu "github.com/kaweezle/iknite/pkg/testutils"
    "github.com/kaweezle/iknite/pkg/utils"
)

func setupExecutor(t *testing.T) func() {
    t.Helper()
    executor := &tu.MockExecutor{}
    old := utils.Exec
    utils.Exec = executor
    oldFS := utils.FS
    utils.FS = utils.NewMemMapFS()
    return func() {
        utils.Exec = old
        utils.FS = oldFS
    }
}

func TestMyFunction(t *testing.T) {
    teardown := setupExecutor(t)
    defer teardown()

    req := require.New(t)
    // ... test logic
    req.NoError(err)
}
```

## Testing Patterns

### MockExecutor

The `pkg/testutils` package provides a `MockExecutor` to avoid running actual
shell commands in tests:

```go
import tu "github.com/kaweezle/iknite/pkg/testutils"

func TestSomethingWithCommand(t *testing.T) {
    executor := &tu.MockExecutor{}
    old := utils.Exec
    utils.Exec = executor
    defer func() { utils.Exec = old }()

    // MockExecutor returns empty output and no error by default
    err := myFunctionThatRunsACommand()
    require.NoError(t, err)
}
```

### Filesystem Mocking (Afero)

Use `utils.NewMemMapFS()` to mock filesystem operations:

```go
import "github.com/kaweezle/iknite/pkg/utils"

func TestFileOperation(t *testing.T) {
    oldFS := utils.FS
    utils.FS = utils.NewMemMapFS()
    defer func() { utils.FS = oldFS }()

    // Write a test file
    err := utils.FS.WriteFile("/etc/test.conf", []byte("content"), 0o644)
    require.NoError(t, err)

    // Call function under test
    result, err := myFunctionThatReadsFile("/etc/test.conf")
    require.NoError(t, err)
    require.Equal(t, "expected", result)
}
```

### Table-Driven Tests

Use table-driven tests for testing multiple scenarios:

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"empty input", "", "", true},
        {"valid input", "hello", "HELLO", false},
        {"special chars", "a-b_c", "A-B_C", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := MyFunction(tt.input)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            require.Equal(t, tt.want, got)
        })
    }
}
```

### Test Suites with testify/suite

For stateful tests that need setup/teardown:

```go
import (
    "testing"
    "github.com/stretchr/testify/suite"
)

type MyTestSuite struct {
    suite.Suite
    // fields for shared state
}

func (s *MyTestSuite) SetupTest() {
    // Run before each test
}

func (s *MyTestSuite) TearDownTest() {
    // Run after each test
}

func (s *MyTestSuite) TestSomething() {
    s.Require().NotNil(something)
}

func TestMyTestSuite(t *testing.T) {
    suite.Run(t, new(MyTestSuite))
}
```

## Test Isolation

**Important**: Reset global state in `TearDown` to avoid cross-test pollution.

The most critical global variables to reset:

```go
// Reset the executor mock
old := utils.Exec
utils.Exec = myMockExecutor
defer func() { utils.Exec = old }()

// Reset the filesystem mock
oldFS := utils.FS
utils.FS = utils.NewMemMapFS()
defer func() { utils.FS = oldFS }()
```

## Test Coverage Requirements

- Aim for **>85% code coverage** on new code
- All exported functions should have tests
- Error paths must be tested

## Writing Tests for New Code

When adding a new function or command:

1. **Create a test file** next to the implementation: `pkg/cmd/your_command_test.go`

2. **Test the happy path**:
   ```go
   func TestYourCommand_Success(t *testing.T) {
       // Set up mocks
       // Call function
       // Assert expected results
   }
   ```

3. **Test error paths**:
   ```go
   func TestYourCommand_WhenXFails(t *testing.T) {
       // Set up mocks to return errors
       // Call function
       // Assert error is returned
   }
   ```

4. **Test edge cases**:
   ```go
   func TestYourCommand_WhenInputIsEmpty(t *testing.T) {
       // Test boundary conditions
   }
   ```

## Integration Tests

The `test/` directory contains integration test resources:

```
test/
├── ops/
│   └── nginx/    ← Example nginx deployment for testing
└── vm/
    └── cloud-init/ ← VM test configuration
```

Integration tests that require a running cluster are not run in CI
automatically. They require manual execution with a live cluster.

## CI Test Execution

Tests run in GitHub Actions on every PR:

```yaml
# .github/workflows/go.yaml
- name: Test
  run: go test -v -race -covermode=atomic -coverprofile=coverage.out ./...
```

Test results and coverage reports are available in the GitHub Actions workflow.
