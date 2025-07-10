package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"sigs.k8s.io/yaml"
)

// Helper function to unmarshal YAML into Task objects
func taskFromYAMLForParam(yamlContent string) (v1.Task, error) {
	var task v1.Task
	err := yaml.Unmarshal([]byte(yamlContent), &task)
	return task, err
}

// Helper function to unmarshal YAML into Pipeline objects
func pipelineFromYAMLForParam(yamlContent string) (v1.Pipeline, error) {
	var pipeline v1.Pipeline
	err := yaml.Unmarshal([]byte(yamlContent), &pipeline)
	return pipeline, err
}

func TestValidateParameterReferences(t *testing.T) {
	tests := []struct {
		name           string
		pipelineSpec   v1.PipelineSpec
		rawYAML        string
		expectedErrors []string
		expectNoError  bool
	}{
		{
			name: "valid parameter references",
			pipelineSpec: v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{Name: "gitUrl", Type: v1.ParamTypeString},
					{Name: "gitRevision", Type: v1.ParamTypeString},
					{Name: "imageTag", Type: v1.ParamTypeString},
				},
			},
			rawYAML: `
apiVersion: tekton.dev/v1
kind: Pipeline
spec:
  params:
    - name: gitUrl
      type: string
    - name: gitRevision
      type: string
    - name: imageTag
      type: string
  tasks:
    - name: clone
      params:
        - name: url
          value: $(params.gitUrl)
        - name: revision
          value: $(params.gitRevision)
    - name: build
      params:
        - name: tag
          value: $(params.imageTag)
`,
			expectNoError: true,
		},
		{
			name: "undefined parameter reference",
			pipelineSpec: v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{Name: "gitUrl", Type: v1.ParamTypeString},
				},
			},
			rawYAML: `
apiVersion: tekton.dev/v1
kind: Pipeline
spec:
  params:
    - name: gitUrl
      type: string
  tasks:
    - name: clone
      params:
        - name: url
          value: $(params.gitUrl)
        - name: revision
          value: $(params.undefinedParam)
`,
			expectedErrors: []string{
				"parameter reference $(params.undefinedParam) not defined in pipeline spec",
			},
		},
		{
			name: "multiple undefined parameter references",
			pipelineSpec: v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{Name: "gitUrl", Type: v1.ParamTypeString},
				},
			},
			rawYAML: `
apiVersion: tekton.dev/v1
kind: Pipeline
spec:
  params:
    - name: gitUrl
      type: string
  tasks:
    - name: clone
      params:
        - name: url
          value: $(params.gitUrl)
        - name: revision
          value: $(params.undefinedParam1)
        - name: branch
          value: $(params.undefinedParam2)
`,
			expectedErrors: []string{
				"parameter reference $(params.undefinedParam1) not defined in pipeline spec",
				"parameter reference $(params.undefinedParam2) not defined in pipeline spec",
			},
		},
		{
			name: "no parameter references in YAML",
			pipelineSpec: v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{Name: "gitUrl", Type: v1.ParamTypeString},
				},
			},
			rawYAML: `
apiVersion: tekton.dev/v1
kind: Pipeline
spec:
  params:
    - name: gitUrl
      type: string
  tasks:
    - name: clone
      params:
        - name: url
          value: "https://github.com/example/repo"
`,
			expectNoError: true,
		},
		{
			name: "empty parameter name reference",
			pipelineSpec: v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{Name: "gitUrl", Type: v1.ParamTypeString},
				},
			},
			rawYAML: `
apiVersion: tekton.dev/v1
kind: Pipeline
spec:
  params:
    - name: gitUrl
      type: string
  tasks:
    - name: clone
      params:
        - name: url
          value: $(params.)
`,
			expectedErrors: []string{
				"parameter reference $(params.) not defined in pipeline spec",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateParameterReferences(tt.pipelineSpec, []byte(tt.rawYAML))

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

func TestExtractParameterReferences(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected []string
	}{
		{
			name:     "single parameter reference",
			yaml:     "value: $(params.gitUrl)",
			expected: []string{"gitUrl"},
		},
		{
			name: "multiple parameter references",
			yaml: `
url: $(params.gitUrl)
revision: $(params.gitRevision)
tag: $(params.imageTag)
`,
			expected: []string{"gitUrl", "gitRevision", "imageTag"},
		},
		{
			name: "duplicate parameter references",
			yaml: `
url: $(params.gitUrl)
backup-url: $(params.gitUrl)
`,
			expected: []string{"gitUrl"}, // Should deduplicate
		},
		{
			name: "no parameter references",
			yaml: `
url: https://github.com/example/repo
revision: main
`,
			expected: []string{},
		},
		{
			name: "parameter references with whitespace",
			yaml: `
url: $(params.gitUrl )
revision: $(params. gitRevision)
`,
			expected: []string{"gitUrl", "gitRevision"},
		},
		{
			name:     "parameter reference with hyphen",
			yaml:     "value: $(params.git-url)",
			expected: []string{"git-url"},
		},
		{
			name:     "parameter reference with underscore",
			yaml:     "value: $(params.git_url)",
			expected: []string{"git_url"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractParameterReferences(tt.yaml)
			assert.ElementsMatch(t, tt.expected, result, "Parameter references should match expected")
		})
	}
}

func TestValidateParameters(t *testing.T) {
	tests := []struct {
		name          string
		taskYAML      string
		expectedError bool
		errorContains []string
	}{
		{
			name: "valid parameters",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: valid-task
spec:
  params:
    - name: gitUrl
      type: string
      description: URL of the Git repository
    - name: gitRevision
      type: string
      description: Git revision to clone
      default: main
  steps:
    - name: clone
      image: alpine/git:latest
      script: |
        git clone $(params.gitUrl) -b $(params.gitRevision)
`,
			expectedError: false,
		},
		{
			name: "undefined parameter",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: undefined-param-task
spec:
  params:
    - name: gitUrl
      type: string
      description: URL of the Git repository
  steps:
    - name: clone
      image: alpine/git:latest
      script: git clone $(params.gitUrl)
`,
			expectedError: true,
			errorContains: []string{
				"parameter is not defined by the Task",
			},
		},
		{
			name: "type mismatch",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: type-mismatch-task
spec:
  params:
    - name: gitUrl
      type: string
      description: URL of the Git repository
  steps:
    - name: clone
      image: alpine/git:latest
      script: git clone $(params.gitUrl)
`,
			expectedError: true,
			errorContains: []string{
				"parameter has the incorrect type",
			},
		},
		{
			name: "missing required parameter",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: missing-required-task
spec:
  params:
    - name: gitUrl
      type: string
      description: URL of the Git repository
    - name: gitRevision
      type: string
      description: Git revision to clone (no default)
  steps:
    - name: clone
      image: alpine/git:latest
      script: git clone $(params.gitUrl)
`,
			expectedError: true,
			errorContains: []string{
				"parameter is required",
			},
		},
		{
			name: "optional parameter with default",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: optional-param-task
spec:
  params:
    - name: gitUrl
      type: string
      description: URL of the Git repository
    - name: gitRevision
      type: string
      description: Git revision to clone
      default: main
  steps:
    - name: clone
      image: alpine/git:latest
      script: git clone $(params.gitUrl) -b $(params.gitRevision)
`,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := taskFromYAMLForParam(tt.taskYAML)
			require.NoError(t, err, "Failed to unmarshal YAML")

			// Create mock parameters to test validation
			mockParams := v1.Params{
				{Name: "gitUrl", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "https://github.com/example/repo"}},
			}

			// For tests expecting specific errors, provide mismatched parameters
			if tt.name == "undefined parameter" {
				mockParams = append(mockParams, v1.Param{
					Name:  "undefinedParam",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "value"},
				})
			}
			if tt.name == "type mismatch" {
				mockParams = v1.Params{
					{Name: "gitUrl", Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"item1", "item2"}}},
				}
			}
			if tt.name == "missing required parameter" {
				// Only provide gitUrl, not gitRevision
				mockParams = v1.Params{
					{Name: "gitUrl", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "https://github.com/example/repo"}},
				}
			}
			if tt.name == "optional parameter with default" {
				mockParams = v1.Params{
					{Name: "gitUrl", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "https://github.com/example/repo"}},
				}
			}

			err = ValidateParameters(mockParams, task.Spec.Params)

			if tt.expectedError {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				errStr := err.Error()
				for _, expectedErr := range tt.errorContains {
					assert.Contains(t, errStr, expectedErr, "Expected error message to contain: %s", expectedErr)
				}
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.name)
			}
		})
	}
}

func TestGetPipelineTaskParam(t *testing.T) {
	params := v1.Params{
		{Name: "gitUrl", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "https://github.com/example/repo"}},
		{Name: "gitRevision", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "main"}},
	}

	tests := []struct {
		name          string
		paramName     string
		expectedFound bool
		expectedValue string
	}{
		{
			name:          "existing parameter",
			paramName:     "gitUrl",
			expectedFound: true,
			expectedValue: "https://github.com/example/repo",
		},
		{
			name:          "non-existing parameter",
			paramName:     "nonexistent",
			expectedFound: false,
		},
		{
			name:          "empty parameter name",
			paramName:     "",
			expectedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param, found := getPipelineTaskParam(tt.paramName, params)
			assert.Equal(t, tt.expectedFound, found, "Found status should match expected")
			if tt.expectedFound {
				assert.Equal(t, tt.expectedValue, param.Value.StringVal, "Parameter value should match expected")
			}
		})
	}
}

func TestGetTaskParam(t *testing.T) {
	specs := v1.ParamSpecs{
		{Name: "gitUrl", Type: v1.ParamTypeString},
		{Name: "gitRevision", Type: v1.ParamTypeString, Default: &v1.ParamValue{Type: v1.ParamTypeString, StringVal: "main"}},
	}

	tests := []struct {
		name          string
		paramName     string
		expectedFound bool
		expectedType  v1.ParamType
		hasDefault    bool
	}{
		{
			name:          "existing parameter without default",
			paramName:     "gitUrl",
			expectedFound: true,
			expectedType:  v1.ParamTypeString,
			hasDefault:    false,
		},
		{
			name:          "existing parameter with default",
			paramName:     "gitRevision",
			expectedFound: true,
			expectedType:  v1.ParamTypeString,
			hasDefault:    true,
		},
		{
			name:          "non-existing parameter",
			paramName:     "nonexistent",
			expectedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, found := getTaskParam(tt.paramName, specs)
			assert.Equal(t, tt.expectedFound, found, "Found status should match expected")
			if tt.expectedFound {
				assert.Equal(t, tt.expectedType, spec.Type, "Parameter type should match expected")
				if tt.hasDefault {
					assert.NotNil(t, spec.Default, "Parameter should have default value")
				} else {
					assert.Nil(t, spec.Default, "Parameter should not have default value")
				}
			}
		})
	}
}
