package validate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseParamValues(t *testing.T) {
	tests := []struct {
		name           string
		paramStrs      []string
		expectedParams map[string]string
		expectedError  bool
		errorContains  string
	}{
		{
			name:      "valid single parameter",
			paramStrs: []string{"gitUrl=https://github.com/example/repo.git"},
			expectedParams: map[string]string{
				"gitUrl": "https://github.com/example/repo.git",
			},
			expectedError: false,
		},
		{
			name: "valid multiple parameters",
			paramStrs: []string{
				"gitUrl=https://github.com/example/repo.git",
				"gitRevision=main",
				"buildArgs=--verbose",
			},
			expectedParams: map[string]string{
				"gitUrl":      "https://github.com/example/repo.git",
				"gitRevision": "main",
				"buildArgs":   "--verbose",
			},
			expectedError: false,
		},
		{
			name:      "parameter with spaces around equals",
			paramStrs: []string{"gitUrl = https://github.com/example/repo.git"},
			expectedParams: map[string]string{
				"gitUrl": "https://github.com/example/repo.git",
			},
			expectedError: false,
		},
		{
			name:      "parameter with empty value",
			paramStrs: []string{"emptyParam="},
			expectedParams: map[string]string{
				"emptyParam": "",
			},
			expectedError: false,
		},
		{
			name:      "parameter with equals in value",
			paramStrs: []string{"complexParam=key=value"},
			expectedParams: map[string]string{
				"complexParam": "key=value",
			},
			expectedError: false,
		},
		{
			name:      "parameter with special characters",
			paramStrs: []string{"specialParam=value-with_special.chars@123"},
			expectedParams: map[string]string{
				"specialParam": "value-with_special.chars@123",
			},
			expectedError: false,
		},
		{
			name:          "invalid parameter format - no equals",
			paramStrs:     []string{"invalidParam"},
			expectedError: true,
			errorContains: "invalid parameter format",
		},
		{
			name:          "invalid parameter format - empty key",
			paramStrs:     []string{"=value"},
			expectedError: true,
			errorContains: "empty parameter key",
		},
		{
			name:          "invalid parameter format - spaces only key",
			paramStrs:     []string{"   =value"},
			expectedError: true,
			errorContains: "empty parameter key",
		},
		{
			name: "mixed valid and invalid parameters",
			paramStrs: []string{
				"validParam=value",
				"invalidParam",
			},
			expectedError: true,
			errorContains: "invalid parameter format",
		},
		{
			name:           "empty parameter list",
			paramStrs:      []string{},
			expectedParams: map[string]string{},
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, err := parseParamValues(tt.paramStrs)

			if tt.expectedError {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains,
						"Expected error message to contain: %s", tt.errorContains)
				}
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.name)
				assert.Equal(t, tt.expectedParams, params,
					"Expected parameters to match for test case: %s", tt.name)
			}
		})
	}
}

func TestSubstituteParameters(t *testing.T) {
	tests := []struct {
		name           string
		yamlContent    []byte
		params         map[string]string
		expectedResult []byte
	}{
		{
			name: "substitute single parameter",
			yamlContent: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline
spec:
  params:
    - name: gitUrl
      type: string
  tasks:
    - name: clone
      params:
        - name: url
          value: $(params.gitUrl)
`),
			params: map[string]string{
				"gitUrl": "https://github.com/example/repo.git",
			},
			expectedResult: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline
spec:
  params:
    - name: gitUrl
      type: string
  tasks:
    - name: clone
      params:
        - name: url
          value: https://github.com/example/repo.git
`),
		},
		{
			name: "substitute multiple parameters",
			yamlContent: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline
spec:
  params:
    - name: gitUrl
      type: string
    - name: gitRevision
      type: string
  tasks:
    - name: clone
      params:
        - name: url
          value: $(params.gitUrl)
        - name: revision
          value: $(params.gitRevision)
`),
			params: map[string]string{
				"gitUrl":      "https://github.com/example/repo.git",
				"gitRevision": "main",
			},
			expectedResult: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline
spec:
  params:
    - name: gitUrl
      type: string
    - name: gitRevision
      type: string
  tasks:
    - name: clone
      params:
        - name: url
          value: https://github.com/example/repo.git
        - name: revision
          value: main
`),
		},
		{
			name: "substitute same parameter multiple times",
			yamlContent: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline
spec:
  params:
    - name: gitUrl
      type: string
  tasks:
    - name: clone
      params:
        - name: url
          value: $(params.gitUrl)
        - name: mirror
          value: $(params.gitUrl)
`),
			params: map[string]string{
				"gitUrl": "https://github.com/example/repo.git",
			},
			expectedResult: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline
spec:
  params:
    - name: gitUrl
      type: string
  tasks:
    - name: clone
      params:
        - name: url
          value: https://github.com/example/repo.git
        - name: mirror
          value: https://github.com/example/repo.git
`),
		},
		{
			name: "no parameters to substitute",
			yamlContent: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline
spec:
  tasks:
    - name: hello
      taskSpec:
        steps:
          - name: hello
            image: alpine:latest
            script: echo "Hello World"
`),
			params: map[string]string{
				"gitUrl": "https://github.com/example/repo.git",
			},
			expectedResult: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline
spec:
  tasks:
    - name: hello
      taskSpec:
        steps:
          - name: hello
            image: alpine:latest
            script: echo "Hello World"
`),
		},
		{
			name: "parameter not provided",
			yamlContent: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline
spec:
  tasks:
    - name: clone
      params:
        - name: url
          value: $(params.gitUrl)
        - name: revision
          value: $(params.gitRevision)
`),
			params: map[string]string{
				"gitUrl": "https://github.com/example/repo.git",
				// gitRevision not provided
			},
			expectedResult: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline
spec:
  tasks:
    - name: clone
      params:
        - name: url
          value: https://github.com/example/repo.git
        - name: revision
          value: $(params.gitRevision)
`),
		},
		{
			name: "empty parameters map",
			yamlContent: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline
spec:
  tasks:
    - name: clone
      params:
        - name: url
          value: $(params.gitUrl)
`),
			params: map[string]string{},
			expectedResult: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline
spec:
  tasks:
    - name: clone
      params:
        - name: url
          value: $(params.gitUrl)
`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substituteParameters(tt.yamlContent, tt.params)
			assert.Equal(t, tt.expectedResult, result,
				"Expected substituted content to match for test case: %s", tt.name)
		})
	}
}

func TestRun(t *testing.T) {
	ctx := context.Background()

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "validate-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name          string
		fileName      string
		fileContent   []byte
		runtimeParams map[string]string
		expectedError bool
		errorContains string
	}{
		{
			name:     "valid pipeline file",
			fileName: "valid-pipeline.yaml",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline
spec:
  params:
    - name: gitUrl
      type: string
    - name: gitRevision
      type: string
      default: main
  tasks:
    - name: clone
      taskSpec:
        params:
          - name: url
            type: string
          - name: revision
            type: string
        steps:
          - name: clone
            image: alpine/git:latest
            script: |
              git clone $(params.url) -b $(params.revision)
      params:
        - name: url
          value: $(params.gitUrl)
        - name: revision
          value: $(params.gitRevision)
`),
			runtimeParams: map[string]string{
				"gitUrl": "https://github.com/example/repo.git",
			},
			expectedError: true,
			errorContains: "parameter reference $(params.url) not defined in pipeline spec",
		},
		{
			name:     "valid task file",
			fileName: "valid-task.yaml",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: test-task
spec:
  params:
    - name: url
      type: string
    - name: revision
      type: string
      default: main
  steps:
    - name: clone
      image: alpine/git:latest
      script: |
        git clone $(params.url) -b $(params.revision)
`),
			runtimeParams: map[string]string{},
			expectedError: false,
		},
		{
			name:     "valid task file v1beta1",
			fileName: "valid-task-v1beta1.yaml",
			fileContent: []byte(`apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: test-task-v1beta1
spec:
  params:
    - name: url
      type: string
    - name: revision
      type: string
      default: main
  steps:
    - name: clone
      image: alpine/git:latest
      script: |
        git clone $(params.url) -b $(params.revision)
`),
			runtimeParams: map[string]string{},
			expectedError: false,
		},
		{
			name:     "valid pipelinerun file",
			fileName: "valid-pipelinerun.yaml",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: test-pipelinerun
spec:
  pipelineSpec:
    params:
      - name: gitUrl
        type: string
      - name: gitRevision
        type: string
        default: main
    tasks:
      - name: clone
        taskSpec:
          params:
            - name: url
              type: string
            - name: revision
              type: string
          steps:
            - name: clone
              image: alpine/git:latest
              script: |
                git clone $(params.url) -b $(params.revision)
        params:
          - name: url
            value: $(params.gitUrl)
          - name: revision
            value: $(params.gitRevision)
  params:
    - name: gitUrl
      value: https://github.com/example/repo.git
    - name: gitRevision
      value: main
`),
			runtimeParams: map[string]string{},
			expectedError: true,
			errorContains: "could not find any PipelineRun in your .tekton/ directory",
		},
		{
			name:     "pipeline with runtime parameter substitution",
			fileName: "pipeline-with-runtime-params.yaml",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:  
  name: test-pipeline-runtime-params
spec:
  params:
    - name: gitUrl
      type: string
    - name: gitRevision
      type: string
      default: main
  tasks:
    - name: clone
      taskSpec:
        params:
          - name: url
            type: string
          - name: revision
            type: string
        steps:
          - name: clone
            image: alpine/git:latest
            script: |
              git clone $(params.url) -b $(params.revision)
      params:
        - name: url
          value: $(params.gitUrl)
        - name: revision
          value: $(params.gitRevision)
`),
			runtimeParams: map[string]string{
				"gitUrl":      "https://github.com/example/repo.git",
				"gitRevision": "feature-branch",
			},
			expectedError: true,
			errorContains: "parameter reference $(params.url) not defined in pipeline spec",
		},
		{
			name:     "pipeline with parameter validation errors",
			fileName: "pipeline-with-param-errors.yaml",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline-param-errors
spec:
  params:
    - name: gitUrl
      type: string
  tasks:
    - name: clone
      taskSpec:
        params:
          - name: url
            type: string
          - name: revision
            type: string
        steps:
          - name: clone
            image: alpine/git:latest
            script: |
              git clone $(params.url) -b $(params.revision)
      params:
        - name: url
          value: $(params.gitUrl)
        - name: revision
          value: $(params.undefinedParam)
`),
			runtimeParams: map[string]string{
				"gitUrl": "https://github.com/example/repo.git",
			},
			expectedError: true,
			errorContains: "undefinedParam",
		},
		{
			name:          "non-existent file",
			fileName:      "non-existent.yaml",
			fileContent:   nil, // Don't create file
			runtimeParams: map[string]string{},
			expectedError: true,
			errorContains: "no such file or directory",
		},
		{
			name:     "invalid yaml file",
			fileName: "invalid.yaml",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline
spec:
  invalid: yaml: content:
`),
			runtimeParams: map[string]string{},
			expectedError: true,
			errorContains: "error",
		},
		{
			name:     "unsupported resource type",
			fileName: "unsupported.yaml",
			fileContent: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
`),
			runtimeParams: map[string]string{},
			expectedError: true,
			errorContains: "not supported",
		},
		{
			name:          "empty file",
			fileName:      "empty.yaml",
			fileContent:   []byte(""),
			runtimeParams: map[string]string{},
			expectedError: true,
			errorContains: "is not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tempDir, tt.fileName)

			// Create test file if content is provided
			if tt.fileContent != nil {
				err := os.WriteFile(filePath, tt.fileContent, 0644)
				require.NoError(t, err)
			}

			err := run(ctx, filePath, tt.runtimeParams)

			if tt.expectedError {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains,
						"Expected error message to contain: %s", tt.errorContains)
				}
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.name)
			}
		})
	}
}

func TestRunWithComplexScenarios(t *testing.T) {
	ctx := context.Background()

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "validate-complex-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name          string
		fileName      string
		fileContent   []byte
		runtimeParams map[string]string
		expectedError bool
		errorContains string
	}{
		{
			name:     "pipeline with git resolver",
			fileName: "pipeline-git-resolver.yaml",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline-git-resolver
spec:
  params:
    - name: gitUrl
      type: string
    - name: gitRevision
      type: string
      default: main
  tasks:
    - name: clone-from-git
      taskRef:
        resolver: git
        params:
          - name: url
            value: https://github.com/tektoncd/catalog.git
          - name: pathInRepo
            value: task/git-clone/0.9/git-clone.yaml
          - name: revision
            value: main
      params:
        - name: url
          value: $(params.gitUrl)
        - name: revision
          value: $(params.gitRevision)
`),
			runtimeParams: map[string]string{
				"gitUrl":      "https://github.com/example/repo.git",
				"gitRevision": "main",
			},
			expectedError: true, // Git resolver validation might fail without network
			errorContains: "required workspace",
		},
		{
			name:     "pipeline with bundle resolver",
			fileName: "pipeline-bundle-resolver.yaml",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline-bundle-resolver
spec:
  params:
    - name: image
      type: string
  tasks:
    - name: build-from-bundle
      taskRef:
        resolver: bundles
        params:
          - name: bundle
            value: $(params.image)
          - name: name
            value: buildah
          - name: kind
            value: task
      params:
        - name: IMAGE
          value: $(params.image)
`),
			runtimeParams: map[string]string{
				"image": "registry.redhat.io/ubi8/ubi:latest",
			},
			expectedError: true, // Bundle resolver validation might fail without registry access
			errorContains: "cannot retrieve the oci image",
		},
		{
			name:     "pipeline with workspaces",
			fileName: "pipeline-workspaces.yaml",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline-workspaces
spec:
  workspaces:
    - name: source
      description: Source workspace
    - name: cache
      description: Cache workspace
      optional: true
  tasks:
    - name: clone
      taskSpec:
        workspaces:
          - name: output
            description: Output workspace
        steps:
          - name: clone
            image: alpine/git:latest
            script: |
              git clone repo /workspace/output
      workspaces:
        - name: output
          workspace: source
    - name: build
      taskSpec:
        workspaces:
          - name: source
            description: Source workspace
          - name: cache
            description: Cache workspace
            optional: true
        steps:
          - name: build
            image: alpine:latest
            script: |
              make build
      workspaces:
        - name: source
          workspace: source
        - name: cache
          workspace: cache
`),
			runtimeParams: map[string]string{},
			expectedError: false,
		},
		{
			name:     "pipeline with results",
			fileName: "pipeline-results.yaml",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline-results
spec:
  results:
    - name: commitHash
      type: string
      description: Git commit hash
      value: $(tasks.clone.results.commit)
  tasks:
    - name: clone
      taskSpec:
        results:
          - name: commit
            type: string
            description: Git commit hash
        steps:
          - name: clone
            image: alpine/git:latest
            script: |
              git clone repo
              git rev-parse HEAD | tee $(results.commit.path)
    - name: build
      taskSpec:
        params:
          - name: commit
            type: string
        steps:
          - name: build
            image: alpine:latest
            script: |
              echo "Building commit $(params.commit)"
              make build
      params:
        - name: commit
          value: $(tasks.clone.results.commit)
`),
			runtimeParams: map[string]string{},
			expectedError: true,
			errorContains: "parameter reference $(params.commit) not defined in pipeline spec",
		},
		{
			name:     "pipeline with finally tasks",
			fileName: "pipeline-finally.yaml",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: test-pipeline-finally
spec:
  tasks:
    - name: build
      taskSpec:
        results:
          - name: status
            type: string
        steps:
          - name: build
            image: alpine:latest
            script: |
              make build
              echo "success" | tee $(results.status.path)
  finally:
    - name: cleanup
      taskSpec:
        params:
          - name: buildStatus
            type: string
        steps:
          - name: cleanup
            image: alpine:latest
            script: |
              echo "Cleaning up, build status: $(params.buildStatus)"
              rm -rf /tmp/build
      params:
        - name: buildStatus
          value: $(tasks.build.results.status)
    - name: notify
      taskSpec:
        steps:
          - name: notify
            image: alpine:latest
            script: |
              echo "Pipeline completed"
`),
			runtimeParams: map[string]string{},
			expectedError: true,
			errorContains: "parameter reference $(params.buildStatus) not defined in pipeline spec",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tempDir, tt.fileName)

			// Create test file if content is provided
			if tt.fileContent != nil {
				err := os.WriteFile(filePath, tt.fileContent, 0644)
				require.NoError(t, err)
			}

			err := run(ctx, filePath, tt.runtimeParams)

			if tt.expectedError {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains,
						"Expected error message to contain: %s", tt.errorContains)
				}
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.name)
			}
		})
	}
}

func TestLogRuntimeParameters(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]string
	}{
		{
			name: "single parameter",
			params: map[string]string{
				"gitUrl": "https://github.com/example/repo.git",
			},
		},
		{
			name: "multiple parameters",
			params: map[string]string{
				"gitUrl":      "https://github.com/example/repo.git",
				"gitRevision": "main",
				"buildArgs":   "--verbose",
			},
		},
		{
			name:   "empty parameters",
			params: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test just ensures the function doesn't panic
			// In a real scenario, you might want to capture and verify log output
			assert.NotPanics(t, func() {
				logRuntimeParameters(tt.params)
			}, "logRuntimeParameters should not panic")
		})
	}
}

func TestRunValidationIntegration(t *testing.T) {
	ctx := context.Background()

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "validate-integration-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Test integration scenario: pipeline with all features
	complexPipelineContent := []byte(`apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: complex-integration-pipeline
spec:
  description: Complex pipeline for integration testing
  params:
    - name: gitUrl
      type: string
      description: Git repository URL
    - name: gitRevision
      type: string
      default: main
      description: Git revision to clone
    - name: buildArgs
      type: array
      default: ["--verbose"]
      description: Build arguments
    - name: runTests
      type: string
      default: "true"
      description: Whether to run tests
  workspaces:
    - name: source
      description: Source code workspace
    - name: cache
      description: Build cache workspace
      optional: true
  results:
    - name: commitHash
      type: string
      description: Git commit hash
      value: $(tasks.clone.results.commit)
    - name: buildStatus
      type: string
      description: Build status
      value: $(tasks.build.results.status)
  tasks:
    - name: clone
      taskSpec:
        params:
          - name: url
            type: string
          - name: revision
            type: string
        workspaces:
          - name: output
            description: Output workspace
        results:
          - name: commit
            type: string
            description: Git commit hash
        steps:
          - name: clone
            image: alpine/git:latest
            script: |
              git clone $(params.url) -b $(params.revision) /workspace/output
              cd /workspace/output
              git rev-parse HEAD | tee $(results.commit.path)
      params:
        - name: url
          value: $(params.gitUrl)
        - name: revision
          value: $(params.gitRevision)
      workspaces:
        - name: output
          workspace: source
    - name: build
      taskSpec:
        params:
          - name: buildArgs
            type: array
        workspaces:
          - name: source
            description: Source workspace
          - name: cache
            description: Cache workspace
            optional: true
        results:
          - name: status
            type: string
            description: Build status
        steps:
          - name: build
            image: alpine:latest
            script: |
              cd /workspace/source
              echo "Building with args: $(params.buildArgs[*])"
              make build $(params.buildArgs[*])
              echo "success" | tee $(results.status.path)
      params:
        - name: buildArgs
          value: $(params.buildArgs)
      workspaces:
        - name: source
          workspace: source
        - name: cache
          workspace: cache
      runAfter:
        - clone
    - name: test
      when:
        - input: $(params.runTests)
          operator: in
          values: ["true"]
      taskSpec:
        workspaces:
          - name: source
            description: Source workspace
        results:
          - name: testResults
            type: string
            description: Test results
        steps:
          - name: test
            image: alpine:latest
            script: |
              cd /workspace/source
              echo "Running tests"
              make test
              echo "passed" | tee $(results.testResults.path)
      workspaces:
        - name: source
          workspace: source
      runAfter:
        - build
  finally:
    - name: cleanup
      taskSpec:
        params:
          - name: buildStatus
            type: string
        steps:
          - name: cleanup
            image: alpine:latest
            script: |
              echo "Cleaning up, build status: $(params.buildStatus)"
              rm -rf /tmp/build
      params:
        - name: buildStatus
          value: $(tasks.build.results.status)
    - name: report
      taskSpec:
        params:
          - name: commit
            type: string
          - name: buildStatus
            type: string
        steps:
          - name: report
            image: alpine:latest
            script: |
              echo "Pipeline completed for commit $(params.commit)"
              echo "Build status: $(params.buildStatus)"
      params:
        - name: commit
          value: $(tasks.clone.results.commit)
        - name: buildStatus
          value: $(tasks.build.results.status)
`)

	filePath := filepath.Join(tempDir, "complex-pipeline.yaml")
	err = os.WriteFile(filePath, complexPipelineContent, 0644)
	require.NoError(t, err)

	// Test with runtime parameters
	runtimeParams := map[string]string{
		"gitUrl":      "https://github.com/example/complex-repo.git",
		"gitRevision": "feature-branch",
		"runTests":    "true",
	}

	err = run(ctx, filePath, runtimeParams)
	assert.Error(t, err, "Complex pipeline validation should fail due to parameter validation issues")
	assert.Contains(t, err.Error(), "parameter reference validation", "Should contain parameter validation errors")
}
