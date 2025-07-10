package validator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"sigs.k8s.io/yaml"
)

// Helper function to unmarshal YAML into Task objects
func taskFromYAML(yamlContent string) (v1.Task, error) {
	var task v1.Task
	err := yaml.Unmarshal([]byte(yamlContent), &task)
	return task, err
}

func taskV1Beta1FromYAML(yamlContent string) (v1beta1.Task, error) {
	var task v1beta1.Task
	err := yaml.Unmarshal([]byte(yamlContent), &task)
	return task, err
}

func TestValidateTaskV1(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		taskYAML      string
		expectedError bool
		errorContains string
	}{
		{
			name: "valid task",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: valid-task
spec:
  steps:
    - name: step1
      image: alpine:latest
      script: echo 'Hello World'
`,
			expectedError: false,
		},
		{
			name: "task with parameters",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: task-with-params
spec:
  params:
    - name: gitUrl
      type: string
      description: Git repository URL
    - name: gitRevision
      type: string
      description: Git revision
      default: main
  steps:
    - name: clone
      image: alpine/git:latest
      script: git clone $(params.gitUrl) -b $(params.gitRevision)
`,
			expectedError: false,
		},
		{
			name: "task with results",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: task-with-results
spec:
  results:
    - name: commit
      type: string
      description: Git commit hash
    - name: files
      type: array
      description: List of files
  steps:
    - name: get-commit
      image: alpine/git:latest
      script: git rev-parse HEAD | tee $(results.commit.path)
`,
			expectedError: false,
		},
		{
			name: "task with workspaces",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: task-with-workspaces
spec:
  workspaces:
    - name: source
      description: Source code workspace
      mountPath: /workspace/source
    - name: output
      description: Output workspace
      optional: true
  steps:
    - name: build
      image: alpine:latest
      script: cd /workspace/source && make build
`,
			expectedError: false,
		},
		{
			name: "task with volumes",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: task-with-volumes
spec:
  volumes:
    - name: cache
      emptyDir: {}
  steps:
    - name: build
      image: alpine:latest
      volumeMounts:
        - name: cache
          mountPath: /cache
      script: echo 'Building with cache'
`,
			expectedError: false,
		},
		{
			name: "invalid task without steps",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: invalid-task
spec:
  # No steps defined
`,
			expectedError: true,
			errorContains: "missing field(s)",
		},
		{
			name: "task with invalid step name",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: task-invalid-step
spec:
  steps:
    - name: invalid-step-name-
      image: alpine:latest
      script: echo 'Hello World'
`,
			expectedError: true,
			errorContains: "invalid value",
		},
		{
			name: "task with duplicate step names",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: task-duplicate-steps
spec:
  steps:
    - name: step1
      image: alpine:latest
      script: echo 'Step 1'
    - name: step1
      image: alpine:latest
      script: echo 'Step 1 duplicate'
`,
			expectedError: true,
			errorContains: "invalid value: step1: spec.steps[1].name",
		},
		{
			name: "task with invalid parameter type",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: task-invalid-param
spec:
  params:
    - name: invalidParam
      type: invalid
  steps:
    - name: step1
      image: alpine:latest
      script: echo 'Test'
`,
			expectedError: true,
			errorContains: "invalid value",
		},
		{
			name: "task with invalid result type",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: task-invalid-result
spec:
  results:
    - name: invalidResult
      type: invalid
  steps:
    - name: step1
      image: alpine:latest
      script: echo 'Test'
`,
			expectedError: true,
			errorContains: "invalid value",
		},
		{
			name: "task with duplicate parameter names",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: task-duplicate-params
spec:
  params:
    - name: param1
      type: string
    - name: param1
      type: string
  steps:
    - name: step1
      image: alpine:latest
      script: echo 'Test'
`,
			expectedError: true,
			errorContains: "parameter appears more than once: spec.params[param1]",
		},
		{
			name: "task with duplicate result names",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: task-duplicate-results
spec:
  results:
    - name: result1
      type: string
    - name: result1
      type: string
  steps:
    - name: step1
      image: alpine:latest
      script: echo 'Test'
`,
			expectedError: false, // Tekton validation doesn't catch this
			errorContains: "",
		},
		{
			name: "task with duplicate workspace names",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: task-duplicate-workspaces
spec:
  workspaces:
    - name: source
    - name: source
  steps:
    - name: step1
      image: alpine:latest
      script: echo 'Test'
`,
			expectedError: true,
			errorContains: "workspace name",
		},
		{
			name: "task with reserved step name",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: task-reserved-step
spec:
  steps:
    - name: place-scripts
      image: alpine:latest
      script: echo 'Test'
`,
			expectedError: false, // Tekton validation doesn't catch this
			errorContains: "",
		},
		{
			name: "task with reserved result name",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: task-reserved-result
spec:
  results:
    - name: STEPS_COMPLETED
      type: string
  steps:
    - name: step1
      image: alpine:latest
      script: echo 'Test'
`,
			expectedError: false, // Tekton validation doesn't catch this
			errorContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := taskFromYAML(tt.taskYAML)
			require.NoError(t, err, "Failed to unmarshal YAML")

			err = ValidateTaskV1(ctx, task)

			if tt.expectedError {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains, "Expected error message to contain: %s", tt.errorContains)
				}
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.name)
			}
		})
	}
}

func TestValidateTaskV1Beta1(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		taskYAML      string
		expectedError bool
		errorContains string
	}{
		{
			name: "valid v1beta1 task",
			taskYAML: `
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: valid-task-v1beta1
spec:
  steps:
    - name: step1
      image: alpine:latest
      script: echo 'Hello World'
`,
			expectedError: false,
		},
		{
			name: "v1beta1 task with parameters",
			taskYAML: `
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: task-with-params-v1beta1
spec:
  params:
    - name: gitUrl
      type: string
      description: Git repository URL
    - name: gitRevision
      type: string
      description: Git revision
      default: main
  steps:
    - name: clone
      image: alpine/git:latest
      script: git clone $(params.gitUrl) -b $(params.gitRevision)
`,
			expectedError: false,
		},
		{
			name: "v1beta1 task with results",
			taskYAML: `
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: task-with-results-v1beta1
spec:
  results:
    - name: commit
      description: Git commit hash
    - name: files
      description: List of files
  steps:
    - name: get-commit
      image: alpine/git:latest
      script: git rev-parse HEAD | tee $(results.commit.path)
`,
			expectedError: false,
		},
		{
			name: "v1beta1 task with workspaces",
			taskYAML: `
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: task-with-workspaces-v1beta1
spec:
  workspaces:
    - name: source
      description: Source code workspace
      mountPath: /workspace/source
    - name: output
      description: Output workspace
      optional: true
  steps:
    - name: build
      image: alpine:latest
      script: cd /workspace/source && make build
`,
			expectedError: false,
		},
		{
			name: "invalid v1beta1 task without steps",
			taskYAML: `
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: invalid-task-v1beta1
spec:
  # No steps defined
`,
			expectedError: true,
			errorContains: "missing field(s)",
		},
		{
			name: "v1beta1 task with invalid step name",
			taskYAML: `
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: task-invalid-step-v1beta1
spec:
  steps:
    - name: invalid-step-name-
      image: alpine:latest
      script: echo 'Hello World'
`,
			expectedError: true,
			errorContains: "invalid value",
		},
		{
			name: "v1beta1 task with duplicate step names",
			taskYAML: `
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: task-duplicate-steps-v1beta1
spec:
  steps:
    - name: step1
      image: alpine:latest
      script: echo 'First step'
    - name: step1
      image: alpine:latest
      script: echo 'Second step'
`,
			expectedError: true,
			errorContains: "invalid value: step1: spec.steps[1].name",
		},
		{
			name: "v1beta1 task with invalid parameter name",
			taskYAML: `
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: task-invalid-param-v1beta1
spec:
  params:
    - name: invalid-param-name-
      type: string
  steps:
    - name: step1
      image: alpine:latest
      script: echo 'Hello World'
`,
			expectedError: false, // Tekton validation doesn't catch this
			errorContains: "",
		},
		{
			name: "v1beta1 task with duplicate parameter names",
			taskYAML: `
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: task-duplicate-params-v1beta1
spec:
  params:
    - name: gitUrl
      type: string
    - name: gitUrl
      type: string
  steps:
    - name: step1
      image: alpine:latest
      script: echo 'Hello World'
`,
			expectedError: true,
			errorContains: "parameter appears more than once: spec.params[gitUrl]",
		},
		{
			name: "v1beta1 task with invalid result name",
			taskYAML: `
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: task-invalid-result-v1beta1
spec:
  results:
    - name: invalid-result-name-
  steps:
    - name: step1
      image: alpine:latest
      script: echo 'Hello World'
`,
			expectedError: true,
			errorContains: "invalid key name \"invalid-result-name-\"",
		},
		{
			name: "v1beta1 task with duplicate result names",
			taskYAML: `
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: task-duplicate-results-v1beta1
spec:
  results:
    - name: commit
    - name: commit
  steps:
    - name: step1
      image: alpine:latest
      script: echo 'Hello World'
`,
			expectedError: false, // Tekton validation doesn't catch this
			errorContains: "",
		},
		{
			name: "v1beta1 task with invalid workspace name",
			taskYAML: `
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: task-invalid-workspace-v1beta1
spec:
  workspaces:
    - name: invalid-workspace-name-
  steps:
    - name: step1
      image: alpine:latest
      script: echo 'Hello World'
`,
			expectedError: false, // Tekton validation doesn't catch this
			errorContains: "",
		},
		{
			name: "v1beta1 task with duplicate workspace names",
			taskYAML: `
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: task-duplicate-workspaces-v1beta1
spec:
  workspaces:
    - name: source
    - name: source
  steps:
    - name: step1
      image: alpine:latest
      script: echo 'Hello World'
`,
			expectedError: true,
			errorContains: "workspace name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := taskV1Beta1FromYAML(tt.taskYAML)
			require.NoError(t, err, "Failed to unmarshal YAML")

			err = ValidateTaskV1Beta1(ctx, task)

			if tt.expectedError {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains, "Expected error message to contain: %s", tt.errorContains)
				}
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.name)
			}
		})
	}
}

func TestTaskValidationComplexScenarios(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		taskYAML      string
		expectedError bool
		description   string
	}{
		{
			name: "task with complex parameter substitution",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: complex-param-task
spec:
  params:
    - name: gitUrl
      type: string
    - name: tags
      type: array
    - name: config
      type: object
      properties:
        env:
          type: string
      default:
        env: "dev"
  steps:
    - name: clone
      image: alpine/git:latest
      script: git clone $(params.gitUrl)
    - name: build
      image: alpine:latest
      script: |
        echo 'Building with URL: $(params.gitUrl)'
        echo 'First tag: $(params.tags[0])'
        echo 'Environment: $(params.config.env)'
`,
			expectedError: false,
			description:   "Task with all parameter types should be valid",
		},
		{
			name: "task with complex workspace usage",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: complex-workspace-task
spec:
  workspaces:
    - name: source
      description: Source code workspace
      mountPath: /workspace/source
    - name: cache
      description: Cache workspace
      mountPath: /cache
      optional: true
    - name: output
      description: Output workspace
      readOnly: false
  steps:
    - name: build
      image: alpine:latest
      script: cd /workspace/source && make build && cp output/* /workspace/output/
`,
			expectedError: false,
			description:   "Task with various workspace configurations should be valid",
		},
		{
			name: "task with sidecars",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: task-with-sidecars
spec:
  steps:
    - name: main
      image: alpine:latest
      script: echo 'Main step'
  sidecars:
    - name: database
      image: postgres:13
      script: postgres -D /var/lib/postgresql/data
`,
			expectedError: false,
			description:   "Task with sidecars should be valid",
		},
		{
			name: "task with step templates",
			taskYAML: `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: task-with-step-template
spec:
  stepTemplate:
    image: alpine:latest
    env:
      - name: DEFAULT_ENV
        value: production
  steps:
    - name: step1
      script: echo 'Step 1 with template'
    - name: step2
      script: echo 'Step 2 with template'
`,
			expectedError: false,
			description:   "Task with step template should be valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := taskFromYAML(tt.taskYAML)
			require.NoError(t, err, "Failed to unmarshal YAML")

			err = ValidateTaskV1(ctx, task)

			if tt.expectedError {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s - %s", tt.name, tt.description)
			}
		})
	}
}
