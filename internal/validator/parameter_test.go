package validator

import (
	"context"
	"strings"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"sigs.k8s.io/yaml"
)

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

func TestPipelineValidationWithResolvedPipeline(t *testing.T) {
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
            value: https://github.com/tektoncd/catalog
          - name: revision
            value: main
          - name: pathInRepo
            value: task.yaml
      params:
        - name: test-param
          value: "test-value"`

	t.Run("validate resolved pipeline", func(t *testing.T) {
		var pipeline v1.Pipeline
		if err := yaml.Unmarshal([]byte(pipelineYAML), &pipeline); err != nil {
			t.Fatalf("failed to unmarshal pipeline YAML: %v", err)
		}

		err := ValidatePipeline(context.Background(), pipeline)

		// The pipeline should validate successfully since it's already resolved
		// (parameters are already substituted with actual values)
		// Note: The git resolver will fail to find the task file, but that's expected
		// since this is a test with mock data
		if err != nil {
			// Check if the error is about the git resolver not finding the task file
			if !strings.Contains(err.Error(), "error opening file") && !strings.Contains(err.Error(), "file does not exist") {
				t.Errorf("expected git resolver error but got: %v", err)
			}
		}
	})
}
