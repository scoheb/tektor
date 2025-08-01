package validator

import (
	"context"
	"strings"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"sigs.k8s.io/yaml"
)

func TestResolveParameterExpressions(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		runtimeParams  map[string]string
		expectedOutput string
	}{
		{
			name:           "resolve single parameter",
			input:          "$(params.taskGitUrl)",
			runtimeParams:  map[string]string{"taskGitUrl": "https://github.com/example/repo"},
			expectedOutput: "https://github.com/example/repo",
		},
		{
			name:           "resolve parameter in URL",
			input:          "https://github.com/$(params.org)/$(params.repo)",
			runtimeParams:  map[string]string{"org": "tektoncd", "repo": "catalog"},
			expectedOutput: "https://github.com/tektoncd/catalog",
		},
		{
			name:           "no parameter to resolve",
			input:          "https://github.com/tektoncd/catalog",
			runtimeParams:  map[string]string{"taskGitUrl": "https://github.com/example/repo"},
			expectedOutput: "https://github.com/tektoncd/catalog",
		},
		{
			name:           "parameter not provided - should remain unchanged",
			input:          "$(params.taskGitUrl)",
			runtimeParams:  map[string]string{},
			expectedOutput: "$(params.taskGitUrl)",
		},
		{
			name:           "mixed resolved and unresolved parameters",
			input:          "$(params.taskGitUrl)/$(params.unresolvedParam)",
			runtimeParams:  map[string]string{"taskGitUrl": "https://github.com/example/repo"},
			expectedOutput: "https://github.com/example/repo/$(params.unresolvedParam)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveParameterExpressions(tt.input, tt.runtimeParams)
			if result != tt.expectedOutput {
				t.Errorf("resolveParameterExpressions() = %q, want %q", result, tt.expectedOutput)
			}
		})
	}
}

func TestResolveParamsInTaskRefParams(t *testing.T) {
	tests := []struct {
		name          string
		paramsYAML    string
		runtimeParams map[string]string
		expectedLen   int
		checkParam    string
		expectedValue string
	}{
		{
			name: "resolve parameter in TaskRef params",
			paramsYAML: `
- name: url
  value: "$(params.taskGitUrl)"
- name: revision  
  value: "$(params.gitRevision)"`,
			runtimeParams: map[string]string{
				"taskGitUrl":  "https://github.com/tektoncd/catalog",
				"gitRevision": "main",
			},
			expectedLen:   2,
			checkParam:    "url",
			expectedValue: "https://github.com/tektoncd/catalog",
		},
		{
			name: "no runtime params provided",
			paramsYAML: `
- name: url
  value: "$(params.taskGitUrl)"`,
			runtimeParams: map[string]string{},
			expectedLen:   1,
			checkParam:    "url",
			expectedValue: "$(params.taskGitUrl)",
		},
		{
			name:          "empty params",
			paramsYAML:    "[]",
			runtimeParams: map[string]string{"taskGitUrl": "https://github.com/tektoncd/catalog"},
			expectedLen:   0,
		},
		{
			name: "mixed resolved and unresolved params",
			paramsYAML: `
- name: url
  value: "$(params.taskGitUrl)"
- name: path
  value: "$(params.unresolvedParam)/task.yaml"
- name: static
  value: "static-value"`,
			runtimeParams: map[string]string{"taskGitUrl": "https://github.com/tektoncd/catalog"},
			expectedLen:   3,
			checkParam:    "path",
			expectedValue: "$(params.unresolvedParam)/task.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var params v1.Params
			if err := yaml.Unmarshal([]byte(tt.paramsYAML), &params); err != nil {
				t.Fatalf("failed to unmarshal params YAML: %v", err)
			}

			result := resolveParamsInTaskRefParams(params, tt.runtimeParams)

			if len(result) != tt.expectedLen {
				t.Errorf("resolveParamsInTaskRefParams() returned %d params, want %d", len(result), tt.expectedLen)
				return
			}

			if tt.checkParam != "" {
				found := false
				for _, param := range result {
					if param.Name == tt.checkParam {
						found = true
						if param.Value.StringVal != tt.expectedValue {
							t.Errorf("param %q value = %q, want %q", tt.checkParam, param.Value.StringVal, tt.expectedValue)
						}
						break
					}
				}
				if !found {
					t.Errorf("param %q not found in result", tt.checkParam)
				}
			}
		})
	}
}

func TestPipelineValidationWithRuntimeParams(t *testing.T) {
	pipelineYAML := `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline
spec:
  params:
    - name: gitUrl
      type: string
      description: Git repository URL
    - name: gitRevision  
      type: string
      description: Git revision
      default: main
  tasks:
    - name: test-task
      taskRef:
        resolver: git
        params:
          - name: url
            value: $(params.gitUrl)
          - name: revision
            value: $(params.gitRevision)
          - name: pathInRepo
            value: task.yaml
      params:
        - name: test-param
          value: "test-value"`

	tests := []struct {
		name            string
		runtimeParams   map[string]string
		expectError     bool
		errorContains   string
		errorNotContain string
	}{
		{
			name: "with runtime parameters provided",
			runtimeParams: map[string]string{
				"gitUrl":      "https://github.com/tektoncd/catalog",
				"gitRevision": "main",
			},
			expectError:     true,                         // Will still error due to git resolution, but parameters are resolved
			errorNotContain: "invalid git repository url", // Key test: parameters should be resolved
		},
		{
			name:          "without runtime parameters",
			runtimeParams: map[string]string{},
			expectError:   true,
			errorContains: "invalid git repository url: $(params.gitUrl)", // Original error
		},
		{
			name: "partial runtime parameters",
			runtimeParams: map[string]string{
				"gitUrl": "https://github.com/tektoncd/catalog",
				// gitRevision not provided, but has default
			},
			expectError:     true,
			errorNotContain: "invalid git repository url", // Parameters should be resolved
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pipeline v1.Pipeline
			if err := yaml.Unmarshal([]byte(pipelineYAML), &pipeline); err != nil {
				t.Fatalf("failed to unmarshal pipeline YAML: %v", err)
			}

			err := ValidatePipeline(context.Background(), pipeline, tt.runtimeParams)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error to contain %q, but got: %v", tt.errorContains, err)
				}
				if tt.errorNotContain != "" && strings.Contains(err.Error(), tt.errorNotContain) {
					t.Errorf("expected error NOT to contain %q, but got: %v", tt.errorNotContain, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}
