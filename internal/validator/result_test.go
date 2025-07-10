package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"sigs.k8s.io/yaml"
)

// Helper functions for unmarshaling YAML into result-related objects
func taskResultsFromYAML(yamlContent string) ([]v1.TaskResult, error) {
	var results []v1.TaskResult
	err := yaml.Unmarshal([]byte(yamlContent), &results)
	return results, err
}

func TestValidateResults(t *testing.T) {
	tests := []struct {
		name           string
		resultRefs     []*v1.ResultRef
		allTaskResults map[string][]v1.TaskResult
		expectedErrors []string
		expectNoError  bool
	}{
		{
			name: "valid result references",
			resultRefs: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "commit"},
				{PipelineTask: "build", Result: "image"},
			},
			allTaskResults: map[string][]v1.TaskResult{
				"clone": {{Name: "commit", Type: v1.ResultsTypeString}},
				"build": {{Name: "image", Type: v1.ResultsTypeString}},
			},
			expectNoError: true,
		},
		{
			name: "non-existent pipeline task",
			resultRefs: []*v1.ResultRef{
				{PipelineTask: "nonexistent", Result: "commit"},
			},
			allTaskResults: map[string][]v1.TaskResult{
				"clone": {{Name: "commit", Type: v1.ResultsTypeString}},
			},
			expectedErrors: []string{
				"commit result from non-existent nonexistent PipelineTask",
			},
		},
		{
			name: "non-existent result",
			resultRefs: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "nonexistent"},
			},
			allTaskResults: map[string][]v1.TaskResult{
				"clone": {{Name: "commit", Type: v1.ResultsTypeString}},
			},
			expectedErrors: []string{
				"non-existent nonexistent result from clone PipelineTask",
			},
		},
		{
			name: "multiple errors",
			resultRefs: []*v1.ResultRef{
				{PipelineTask: "nonexistent", Result: "commit"},
				{PipelineTask: "clone", Result: "nonexistent"},
			},
			allTaskResults: map[string][]v1.TaskResult{
				"clone": {{Name: "commit", Type: v1.ResultsTypeString}},
			},
			expectedErrors: []string{
				"commit result from non-existent nonexistent PipelineTask",
				"non-existent nonexistent result from clone PipelineTask",
			},
		},
		{
			name:       "empty result refs",
			resultRefs: []*v1.ResultRef{},
			allTaskResults: map[string][]v1.TaskResult{
				"clone": {{Name: "commit", Type: v1.ResultsTypeString}},
			},
			expectNoError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateResults(tt.resultRefs, tt.allTaskResults)

			if tt.expectNoError {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.name)
			} else {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				errStr := err.Error()
				for _, expectedErr := range tt.expectedErrors {
					assert.Contains(t, errStr, expectedErr, "Expected error message to contain: %s", expectedErr)
				}
			}
		})
	}
}

func TestValidateResultsWithContext(t *testing.T) {
	tests := []struct {
		name           string
		resultRefs     []*v1.ResultRef
		allTaskResults map[string][]v1.TaskResult
		usageContexts  map[string]resultUsageContext
		expectedErrors []string
		expectNoError  bool
	}{
		{
			name: "valid string result usage",
			resultRefs: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "commit"},
			},
			allTaskResults: map[string][]v1.TaskResult{
				"clone": {{Name: "commit", Type: v1.ResultsTypeString}},
			},
			usageContexts: map[string]resultUsageContext{
				"clone.commit": {
					Location:     "PipelineTask build parameter url",
					ExpectedType: "string",
					ActualUsage:  "$(tasks.clone.results.commit)",
				},
			},
			expectNoError: true,
		},
		{
			name: "valid array result usage",
			resultRefs: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "files"},
			},
			allTaskResults: map[string][]v1.TaskResult{
				"clone": {{Name: "files", Type: v1.ResultsTypeArray}},
			},
			usageContexts: map[string]resultUsageContext{
				"clone.files": {
					Location:     "PipelineTask build parameter files",
					ExpectedType: "array",
					ActualUsage:  "$(tasks.clone.results.files)",
				},
			},
			expectNoError: true,
		},
		{
			name: "valid array indexing usage",
			resultRefs: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "files"},
			},
			allTaskResults: map[string][]v1.TaskResult{
				"clone": {{Name: "files", Type: v1.ResultsTypeArray}},
			},
			usageContexts: map[string]resultUsageContext{
				"clone.files": {
					Location:     "PipelineTask build parameter file",
					ExpectedType: "string",
					ActualUsage:  "$(tasks.clone.results.files[0])",
				},
			},
			expectNoError: true,
		},
		{
			name: "invalid array usage as string",
			resultRefs: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "files"},
			},
			allTaskResults: map[string][]v1.TaskResult{
				"clone": {{Name: "files", Type: v1.ResultsTypeArray}},
			},
			usageContexts: map[string]resultUsageContext{
				"clone.files": {
					Location:     "PipelineTask build parameter file",
					ExpectedType: "string",
					ActualUsage:  "$(tasks.clone.results.files)",
				},
			},
			expectedErrors: []string{
				"result type mismatch: files result from clone PipelineTask is defined as type \"array\" but used as type \"string\"",
			},
		},
		{
			name: "valid object property usage",
			resultRefs: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "metadata"},
			},
			allTaskResults: map[string][]v1.TaskResult{
				"clone": {{Name: "metadata", Type: v1.ResultsTypeObject}},
			},
			usageContexts: map[string]resultUsageContext{
				"clone.metadata": {
					Location:     "PipelineTask build parameter author",
					ExpectedType: "string",
					ActualUsage:  "$(tasks.clone.results.metadata.author)",
				},
			},
			expectNoError: true,
		},
		{
			name: "invalid object usage as string",
			resultRefs: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "metadata"},
			},
			allTaskResults: map[string][]v1.TaskResult{
				"clone": {{Name: "metadata", Type: v1.ResultsTypeObject}},
			},
			usageContexts: map[string]resultUsageContext{
				"clone.metadata": {
					Location:     "PipelineTask build parameter metadata",
					ExpectedType: "string",
					ActualUsage:  "$(tasks.clone.results.metadata)",
				},
			},
			expectedErrors: []string{
				"result type mismatch: metadata result from clone PipelineTask is defined as type \"object\" but used as type \"string\"",
			},
		},
		{
			name: "multiple type mismatches",
			resultRefs: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "files"},
				{PipelineTask: "clone", Result: "metadata"},
			},
			allTaskResults: map[string][]v1.TaskResult{
				"clone": {
					{Name: "files", Type: v1.ResultsTypeArray},
					{Name: "metadata", Type: v1.ResultsTypeObject},
				},
			},
			usageContexts: map[string]resultUsageContext{
				"clone.files": {
					Location:     "PipelineTask build parameter file",
					ExpectedType: "string",
					ActualUsage:  "$(tasks.clone.results.files)",
				},
				"clone.metadata": {
					Location:     "PipelineTask build parameter meta",
					ExpectedType: "string",
					ActualUsage:  "$(tasks.clone.results.metadata)",
				},
			},
			expectedErrors: []string{
				"result type mismatch: files result from clone PipelineTask is defined as type \"array\" but used as type \"string\"",
				"result type mismatch: metadata result from clone PipelineTask is defined as type \"object\" but used as type \"string\"",
			},
		},
		{
			name: "non-existent result with context",
			resultRefs: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "nonexistent"},
			},
			allTaskResults: map[string][]v1.TaskResult{
				"clone": {{Name: "commit", Type: v1.ResultsTypeString}},
			},
			usageContexts: map[string]resultUsageContext{
				"clone.nonexistent": {
					Location:     "PipelineTask build parameter value",
					ExpectedType: "string",
					ActualUsage:  "$(tasks.clone.results.nonexistent)",
				},
			},
			expectedErrors: []string{
				"non-existent nonexistent result from clone PipelineTask",
			},
		},
		{
			name: "empty contexts",
			resultRefs: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "commit"},
			},
			allTaskResults: map[string][]v1.TaskResult{
				"clone": {{Name: "commit", Type: v1.ResultsTypeString}},
			},
			usageContexts: map[string]resultUsageContext{},
			expectNoError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateResultsWithContext(tt.resultRefs, tt.allTaskResults, tt.usageContexts)

			if tt.expectNoError {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.name)
			} else {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				errStr := err.Error()
				for _, expectedErr := range tt.expectedErrors {
					assert.Contains(t, errStr, expectedErr, "Expected error message to contain: %s", expectedErr)
				}
			}
		})
	}
}

func TestIsResultTypeCompatible(t *testing.T) {
	tests := []struct {
		name         string
		definedType  string
		expectedType string
		actualUsage  string
		expected     bool
	}{
		// String type tests
		{
			name:         "string to string",
			definedType:  "string",
			expectedType: "string",
			actualUsage:  "$(tasks.task.results.result)",
			expected:     true,
		},
		{
			name:         "string to array (invalid)",
			definedType:  "string",
			expectedType: "array",
			actualUsage:  "$(tasks.task.results.result)",
			expected:     false,
		},
		// Array type tests
		{
			name:         "array to array",
			definedType:  "array",
			expectedType: "array",
			actualUsage:  "$(tasks.task.results.result)",
			expected:     true,
		},
		{
			name:         "array to string via indexing",
			definedType:  "array",
			expectedType: "string",
			actualUsage:  "$(tasks.task.results.result[0])",
			expected:     true,
		},
		{
			name:         "array to string via wildcard",
			definedType:  "array",
			expectedType: "string",
			actualUsage:  "$(tasks.task.results.result[*])",
			expected:     true,
		},
		{
			name:         "array to string without indexing (invalid)",
			definedType:  "array",
			expectedType: "string",
			actualUsage:  "$(tasks.task.results.result)",
			expected:     false,
		},
		// Object type tests
		{
			name:         "object to object",
			definedType:  "object",
			expectedType: "object",
			actualUsage:  "$(tasks.task.results.result)",
			expected:     true,
		},
		{
			name:         "object to string via property access",
			definedType:  "object",
			expectedType: "string",
			actualUsage:  "$(tasks.task.results.result.property)",
			expected:     true,
		},
		{
			name:         "object to string without property access (invalid)",
			definedType:  "object",
			expectedType: "string",
			actualUsage:  "$(tasks.task.results.result)",
			expected:     false,
		},
		// Default and special cases
		{
			name:         "empty expected type",
			definedType:  "string",
			expectedType: "",
			actualUsage:  "$(tasks.task.results.result)",
			expected:     true,
		},
		{
			name:         "unknown defined type defaults to string",
			definedType:  "unknown",
			expectedType: "string",
			actualUsage:  "$(tasks.task.results.result)",
			expected:     true,
		},
		{
			name:         "unknown defined type to array (invalid)",
			definedType:  "unknown",
			expectedType: "array",
			actualUsage:  "$(tasks.task.results.result)",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isResultTypeCompatible(tt.definedType, tt.expectedType, tt.actualUsage)
			assert.Equal(t, tt.expected, result, "Type compatibility should match expected")
		})
	}
}

func TestIsArrayIndexUsage(t *testing.T) {
	tests := []struct {
		name     string
		usage    string
		expected bool
	}{
		{
			name:     "numeric index",
			usage:    "$(tasks.task.results.array[0])",
			expected: true,
		},
		{
			name:     "wildcard index",
			usage:    "$(tasks.task.results.array[*])",
			expected: true,
		},
		{
			name:     "high numeric index",
			usage:    "$(tasks.task.results.array[123])",
			expected: true,
		},
		{
			name:     "no index",
			usage:    "$(tasks.task.results.array)",
			expected: false,
		},
		{
			name:     "empty brackets",
			usage:    "$(tasks.task.results.array[])",
			expected: false,
		},
		{
			name:     "string in brackets",
			usage:    "$(tasks.task.results.array[item])",
			expected: false,
		},
		{
			name:     "multiple indices",
			usage:    "$(tasks.task.results.array[0][1])",
			expected: true, // Should still match first index
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isArrayIndexUsage(tt.usage)
			assert.Equal(t, tt.expected, result, "Array index usage detection should match expected")
		})
	}
}

func TestIsObjectPropertyUsage(t *testing.T) {
	tests := []struct {
		name     string
		usage    string
		expected bool
	}{
		{
			name:     "single property access",
			usage:    "$(tasks.task.results.object.property)",
			expected: true,
		},
		{
			name:     "nested property access",
			usage:    "$(tasks.task.results.object.nested.property)",
			expected: true,
		},
		{
			name:     "no property access",
			usage:    "$(tasks.task.results.object)",
			expected: false,
		},
		{
			name:     "property with hyphen",
			usage:    "$(tasks.task.results.object.property-name)",
			expected: true,
		},
		{
			name:     "property with underscore",
			usage:    "$(tasks.task.results.object.property_name)",
			expected: true,
		},
		{
			name:     "no results in usage",
			usage:    "$(tasks.task.object.property)",
			expected: false,
		},
		{
			name:     "trailing spaces",
			usage:    "$(tasks.task.results.object.property )",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isObjectPropertyUsage(tt.usage)
			assert.Equal(t, tt.expected, result, "Object property usage detection should match expected")
		})
	}
}

func TestExtractResultUsageContexts(t *testing.T) {
	tests := []struct {
		name     string
		rawYAML  string
		location string
		expected map[string]resultUsageContext
	}{
		{
			name: "single result reference",
			rawYAML: `
params:
  - name: url
    value: $(tasks.clone.results.commit)
`,
			location: "PipelineTask build",
			expected: map[string]resultUsageContext{
				"clone.commit": {
					Location:     "PipelineTask build",
					ExpectedType: "string",
					ActualUsage:  "$(tasks.clone.results.commit)",
				},
			},
		},
		{
			name: "multiple result references",
			rawYAML: `
params:
  - name: url
    value: $(tasks.clone.results.commit)
  - name: files
    value: $(tasks.clone.results.files)
`,
			location: "PipelineTask build",
			expected: map[string]resultUsageContext{
				"clone.commit": {
					Location:     "PipelineTask build",
					ExpectedType: "string",
					ActualUsage:  "$(tasks.clone.results.commit)",
				},
				"clone.files": {
					Location:     "PipelineTask build",
					ExpectedType: "string",
					ActualUsage:  "$(tasks.clone.results.files)",
				},
			},
		},
		{
			name: "array indexing usage",
			rawYAML: `
params:
  - name: file
    value: $(tasks.clone.results.files[0])
`,
			location: "PipelineTask build",
			expected: map[string]resultUsageContext{
				"clone.files": {
					Location:     "PipelineTask build",
					ExpectedType: "string",
					ActualUsage:  "$(tasks.clone.results.files[0])",
				},
			},
		},
		{
			name: "object property usage",
			rawYAML: `
params:
  - name: author
    value: $(tasks.clone.results.metadata.author)
`,
			location: "PipelineTask build",
			expected: map[string]resultUsageContext{
				"clone.metadata": {
					Location:     "PipelineTask build",
					ExpectedType: "string",
					ActualUsage:  "$(tasks.clone.results.metadata.author)",
				},
			},
		},
		{
			name: "no result references",
			rawYAML: `
params:
  - name: url
    value: "https://github.com/example/repo"
`,
			location: "PipelineTask build",
			expected: map[string]resultUsageContext{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractResultUsageContexts([]byte(tt.rawYAML), tt.location)
			assert.Equal(t, tt.expected, result, "Extracted contexts should match expected")
		})
	}
}

func TestDetermineExpectedTypeFromUsage(t *testing.T) {
	tests := []struct {
		name      string
		fullUsage string
		suffix    string
		expected  string
	}{
		{
			name:      "simple result usage",
			fullUsage: "$(tasks.task.results.result)",
			suffix:    "",
			expected:  "string",
		},
		{
			name:      "array indexing usage",
			fullUsage: "$(tasks.task.results.array[0])",
			suffix:    "",
			expected:  "string",
		},
		{
			name:      "wildcard array indexing",
			fullUsage: "$(tasks.task.results.array[*])",
			suffix:    "",
			expected:  "string",
		},
		{
			name:      "object property usage",
			fullUsage: "$(tasks.task.results.object.property)",
			suffix:    ".property",
			expected:  "string",
		},
		{
			name:      "nested object property usage",
			fullUsage: "$(tasks.task.results.object.nested.property)",
			suffix:    ".nested.property",
			expected:  "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineExpectedTypeFromUsage(tt.fullUsage, tt.suffix)
			assert.Equal(t, tt.expected, result, "Expected type should match")
		})
	}
}

func TestValidateResultsWithRawYAML(t *testing.T) {
	tests := []struct {
		name           string
		resultRefs     []*v1.ResultRef
		allTaskResults map[string][]v1.TaskResult
		rawYAML        string
		location       string
		expectedErrors []string
		expectNoError  bool
	}{
		{
			name: "valid usage with raw YAML",
			resultRefs: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "commit"},
			},
			allTaskResults: map[string][]v1.TaskResult{
				"clone": {{Name: "commit", Type: v1.ResultsTypeString}},
			},
			rawYAML: `
params:
  - name: url
    value: $(tasks.clone.results.commit)
`,
			location:      "PipelineTask build",
			expectNoError: true,
		},
		{
			name: "type mismatch with raw YAML context",
			resultRefs: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "files"},
			},
			allTaskResults: map[string][]v1.TaskResult{
				"clone": {{Name: "files", Type: v1.ResultsTypeArray}},
			},
			rawYAML: `
params:
  - name: file
    value: $(tasks.clone.results.files)
`,
			location: "PipelineTask build",
			expectedErrors: []string{
				"result type mismatch: files result from clone PipelineTask is defined as type \"array\" but used as type \"string\"",
			},
		},
		{
			name: "nil raw YAML falls back to basic validation",
			resultRefs: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "commit"},
			},
			allTaskResults: map[string][]v1.TaskResult{
				"clone": {{Name: "commit", Type: v1.ResultsTypeString}},
			},
			rawYAML:       "",
			location:      "PipelineTask build",
			expectNoError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rawYAML []byte
			if tt.rawYAML != "" {
				rawYAML = []byte(tt.rawYAML)
			}

			err := ValidateResultsWithRawYAML(tt.resultRefs, tt.allTaskResults, rawYAML, tt.location)

			if tt.expectNoError {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.name)
			} else {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				errStr := err.Error()
				for _, expectedErr := range tt.expectedErrors {
					assert.Contains(t, errStr, expectedErr, "Expected error message to contain: %s", expectedErr)
				}
			}
		})
	}
}
