# Test Fixtures

This directory contains YAML test fixtures for comprehensive validation testing of the tektor application.

## Pipeline Fixtures

### Valid Pipelines
- **`pipeline-valid.yaml`** - A comprehensive valid pipeline demonstrating:
  - Parameter definitions with defaults and types (string, array, object)
  - Workspace declarations (required and optional)
  - Embedded tasks with proper parameter and result references
  - Finally tasks for cleanup and reporting
  - All validation scenarios that should pass

- **`pipeline-git-resolver.yaml`** - A pipeline using git resolver for remote task references:
  - Git resolver task references with proper parameters
  - Mixed embedded and remote tasks
  - Parameter references in resolver configurations

### Invalid Pipelines
- **`pipeline-invalid-params.yaml`** - Parameter validation errors:
  - Undefined parameter references
  - Missing required parameters
  - Parameter type mismatches

- **`pipeline-invalid-results.yaml`** - Result validation errors:
  - References to non-existent tasks
  - References to non-existent results
  - Invalid result usage patterns

- **`pipeline-workspace-errors.yaml`** - Workspace validation errors:
  - Undefined workspace references
  - Missing required workspace bindings
  - Conflicting workspace mount paths

## Task Fixtures

### Valid Tasks
- **`task-valid.yaml`** - A comprehensive valid task demonstrating:
  - All parameter types (string, array, object) with defaults
  - All result types (string, array, object)
  - Workspace declarations (required and optional)
  - Volume definitions and mounts
  - Multiple steps with different configurations
  - Sidecars with proper configuration
  - Environment variables and volume mounts

### Invalid Tasks
- **`task-invalid.yaml`** - Task validation errors:
  - Invalid parameter and result types
  - Undefined parameter and result references
  - Conflicting workspace mount paths
  - Invalid volume references
  - Invalid naming conventions

## PipelineRun Fixtures

### Valid PipelineRuns
- **`pipelinerun-valid.yaml`** - A comprehensive valid pipelinerun demonstrating:
  - Embedded pipeline specification
  - Parameter value assignments with type compatibility
  - Workspace bindings with different volume sources
  - Timeout configurations
  - All validation scenarios that should pass

### Invalid PipelineRuns
- **`pipelinerun-invalid.yaml`** - PipelineRun validation errors:
  - Missing required parameters
  - Parameter type mismatches
  - Missing required workspace bindings
  - Undefined parameter and workspace references

## Specialized Test Fixtures

- **`result-types.yaml`** - Result type validation scenarios:
  - Different result types (string, array, object)
  - Valid type compatibility patterns
  - Invalid type usage examples
  - Array indexing and object property access

## Usage

These fixtures are used by the test suites in:
- `internal/validator/*_test.go` - Unit tests for validation components
- `cmd/validate/validate_test.go` - Integration tests for the validate command
- `internal/pac/pac_test.go` - PAC resolution functionality tests

Each fixture is designed to test specific validation scenarios and can be used with the tektor validate command:

```bash
# Test valid scenarios
tektor validate testdata/pipeline-valid.yaml
tektor validate testdata/task-valid.yaml
tektor validate testdata/pipelinerun-valid.yaml

# Test invalid scenarios (should produce validation errors)
tektor validate testdata/pipeline-invalid-params.yaml
tektor validate testdata/pipeline-invalid-results.yaml
tektor validate testdata/task-invalid.yaml

# Test with runtime parameters
tektor validate testdata/pipeline-valid.yaml \
  --param gitUrl=https://github.com/example/repo.git \
  --param gitRevision=feature-branch
```

## Test Coverage

The fixtures provide comprehensive coverage for:

### Parameter Validation
- ✅ Required vs optional parameters
- ✅ Parameter type validation (string, array, object)
- ✅ Parameter reference validation
- ✅ Runtime parameter substitution
- ✅ Undefined parameter detection

### Result Validation
- ✅ Result type compatibility (string, array, object)
- ✅ Result reference validation
- ✅ Array indexing syntax
- ✅ Object property access syntax
- ✅ Non-existent result detection

### Workspace Validation
- ✅ Required vs optional workspaces
- ✅ Workspace binding validation
- ✅ Mount path conflict detection
- ✅ Undefined workspace detection

### Task Validation
- ✅ Embedded task validation
- ✅ Git resolver validation
- ✅ Bundle resolver validation
- ✅ Step and sidecar validation
- ✅ Volume and volume mount validation

### Pipeline Validation
- ✅ Task dependency validation
- ✅ Finally task validation
- ✅ Pipeline result validation
- ✅ Complex pipeline scenarios

### PipelineRun Validation
- ✅ Pipeline specification validation
- ✅ Parameter compatibility validation
- ✅ Workspace binding validation
- ✅ Timeout configuration validation 
