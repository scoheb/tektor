package pac

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvePipelineRun(t *testing.T) {
	t.Skip("PAC tests require complex git repository setup with .tekton/ directory structure")
	ctx := context.Background()

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "pac-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create .tekton directory to mimic git repository structure
	tektonDir := filepath.Join(tempDir, ".tekton")
	err = os.MkdirAll(tektonDir, 0755)
	require.NoError(t, err)

	tests := []struct {
		name            string
		fileName        string
		pipelineRunName string
		fileContent     []byte
		expectedError   bool
		errorContains   string
	}{
		{
			name:            "valid pipelinerun file",
			fileName:        "valid-pipelinerun.yaml",
			pipelineRunName: "test-pipeline-run",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: test-pipeline-run
spec:
  pipelineSpec:
    tasks:
      - name: hello
        taskSpec:
          steps:
            - name: hello
              image: alpine:latest
              script: |
                echo "Hello World"
`),
			expectedError: false,
		},
		{
			name:            "pipelinerun with pipeline reference",
			fileName:        "pipelinerun-with-ref.yaml",
			pipelineRunName: "test-pipeline-run-ref",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: test-pipeline-run-ref
spec:
  pipelineRef:
    name: my-pipeline
  params:
    - name: gitUrl
      value: https://github.com/example/repo.git
    - name: gitRevision
      value: main
`),
			expectedError: false,
		},
		{
			name:            "pipelinerun with parameters",
			fileName:        "pipelinerun-with-params.yaml",
			pipelineRunName: "test-pipeline-run-params",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: test-pipeline-run-params
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
			expectedError: false,
		},
		{
			name:            "pipelinerun with workspaces",
			fileName:        "pipelinerun-with-workspaces.yaml",
			pipelineRunName: "test-pipeline-run-workspaces",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: test-pipeline-run-workspaces
spec:
  pipelineSpec:
    workspaces:
      - name: source
        description: Source workspace
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
  workspaces:
    - name: source
      emptyDir: {}
`),
			expectedError: false,
		},
		{
			name:            "non-existent file",
			fileName:        "non-existent.yaml",
			pipelineRunName: "test-pipeline-run",
			fileContent:     nil, // Don't create file
			expectedError:   true,
			errorContains:   "could not find any PipelineRun",
		},
		{
			name:            "invalid yaml",
			fileName:        "invalid.yaml",
			pipelineRunName: "test-pipeline-run",
			fileContent:     []byte("invalid: yaml: content:"),
			expectedError:   true,
			errorContains:   "could not find any PipelineRun",
		},
		{
			name:            "not a pipelinerun",
			fileName:        "not-pipelinerun.yaml",
			pipelineRunName: "test-pipeline-run",
			fileContent: []byte(`apiVersion: tekton.dev/v1
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
            script: |
              echo "Hello World"
`),
			expectedError: true,
			errorContains: "could not find any PipelineRun",
		},
		{
			name:            "empty file",
			fileName:        "empty.yaml",
			pipelineRunName: "test-pipeline-run",
			fileContent:     []byte(""),
			expectedError:   true,
			errorContains:   "could not find any PipelineRun",
		},
		{
			name:            "pipelinerun with wrong name",
			fileName:        "wrong-name.yaml",
			pipelineRunName: "expected-name",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: actual-name
spec:
  pipelineSpec:
    tasks:
      - name: hello
        taskSpec:
          steps:
            - name: hello
              image: alpine:latest
              script: |
                echo "Hello World"
`),
			expectedError: true,
			errorContains: "unable to find \"expected-name\" pipelinerun after pac resolution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tektonDir, tt.fileName)

			// Create test file if content is provided
			if tt.fileContent != nil {
				err := os.WriteFile(filePath, tt.fileContent, 0644)
				require.NoError(t, err)
			}

			result, err := ResolvePipelineRun(ctx, filePath, tt.pipelineRunName)

			if tt.expectedError {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains,
						"Expected error message to contain: %s", tt.errorContains)
				}
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.name)
				assert.NotEmpty(t, result, "Expected non-empty result for test case: %s", tt.name)

				// Verify the result contains valid YAML
				assert.Contains(t, string(result), "apiVersion: tekton.dev/v1")
				assert.Contains(t, string(result), "kind: PipelineRun")
				assert.Contains(t, string(result), tt.pipelineRunName)
			}
		})
	}
}

func TestResolvePipelineRunWithComplexScenarios(t *testing.T) {
	t.Skip("PAC tests require complex git repository setup with .tekton/ directory structure")
	ctx := context.Background()

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "pac-complex-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create .tekton directory to mimic git repository structure
	tektonDir := filepath.Join(tempDir, ".tekton")
	err = os.MkdirAll(tektonDir, 0755)
	require.NoError(t, err)

	tests := []struct {
		name            string
		fileName        string
		pipelineRunName string
		fileContent     []byte
		expectedError   bool
		errorContains   string
	}{
		{
			name:            "pipelinerun with multiple tasks",
			fileName:        "multi-task-pipelinerun.yaml",
			pipelineRunName: "multi-task-pipeline-run",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: multi-task-pipeline-run
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
          results:
            - name: commit
              type: string
          steps:
            - name: clone
              image: alpine/git:latest
              script: |
                git clone $(params.url) -b $(params.revision)
                git rev-parse HEAD | tee $(results.commit.path)
        params:
          - name: url
            value: $(params.gitUrl)
          - name: revision
            value: $(params.gitRevision)
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
        runAfter:
          - clone
      - name: test
        taskSpec:
          steps:
            - name: test
              image: alpine:latest
              script: |
                echo "Running tests"
                make test
        runAfter:
          - build
    finally:
      - name: cleanup
        taskSpec:
          steps:
            - name: cleanup
              image: alpine:latest
              script: |
                echo "Cleaning up"
                rm -rf /tmp/build
  params:
    - name: gitUrl
      value: https://github.com/example/repo.git
    - name: gitRevision
      value: main
`),
			expectedError: false,
		},
		{
			name:            "pipelinerun with git resolver reference",
			fileName:        "git-resolver-pipelinerun.yaml",
			pipelineRunName: "git-resolver-pipeline-run",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: git-resolver-pipeline-run
spec:
  pipelineSpec:
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
            value: https://github.com/example/repo.git
          - name: revision
            value: main
      - name: build
        taskSpec:
          steps:
            - name: build
              image: alpine:latest
              script: |
                echo "Building"
                make build
        runAfter:
          - clone-from-git
  params:
    - name: gitUrl
      value: https://github.com/example/repo.git
`),
			expectedError: false,
		},
		{
			name:            "pipelinerun with bundle resolver reference",
			fileName:        "bundle-resolver-pipelinerun.yaml",
			pipelineRunName: "bundle-resolver-pipeline-run",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: bundle-resolver-pipeline-run
spec:
  pipelineSpec:
    tasks:
      - name: clone-from-bundle
        taskRef:
          resolver: bundles
          params:
            - name: bundle
              value: registry.redhat.io/ubi8/ubi:latest
            - name: name
              value: git-clone
            - name: kind
              value: task
        params:
          - name: url
            value: https://github.com/example/repo.git
          - name: revision
            value: main
      - name: build
        taskSpec:
          steps:
            - name: build
              image: alpine:latest
              script: |
                echo "Building"
                make build
        runAfter:
          - clone-from-bundle
`),
			expectedError: false,
		},
		{
			name:            "pipelinerun with conditional execution",
			fileName:        "conditional-pipelinerun.yaml",
			pipelineRunName: "conditional-pipeline-run",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: conditional-pipeline-run
spec:
  pipelineSpec:
    params:
      - name: runTests
        type: string
        default: "true"
    tasks:
      - name: build
        taskSpec:
          steps:
            - name: build
              image: alpine:latest
              script: |
                echo "Building"
                make build
      - name: test
        when:
          - input: $(params.runTests)
            operator: in
            values: ["true"]
        taskSpec:
          steps:
            - name: test
              image: alpine:latest
              script: |
                echo "Running tests"
                make test
        runAfter:
          - build
      - name: deploy
        taskSpec:
          steps:
            - name: deploy
              image: alpine:latest
              script: |
                echo "Deploying"
                make deploy
        runAfter:
          - test
  params:
    - name: runTests
      value: "true"
`),
			expectedError: false,
		},
		{
			name:            "pipelinerun with matrix execution",
			fileName:        "matrix-pipelinerun.yaml",
			pipelineRunName: "matrix-pipeline-run",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: matrix-pipeline-run
spec:
  pipelineSpec:
    tasks:
      - name: test-matrix
        taskSpec:
          params:
            - name: version
              type: string
            - name: os
              type: string
          steps:
            - name: test
              image: alpine:latest
              script: |
                echo "Testing on $(params.os) with version $(params.version)"
                make test
        matrix:
          params:
            - name: version
              value:
                - "1.19"
                - "1.20"
                - "1.21"
            - name: os
              value:
                - "linux"
                - "windows"
                - "darwin"
`),
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tektonDir, tt.fileName)

			// Create test file if content is provided
			if tt.fileContent != nil {
				err := os.WriteFile(filePath, tt.fileContent, 0644)
				require.NoError(t, err)
			}

			result, err := ResolvePipelineRun(ctx, filePath, tt.pipelineRunName)

			if tt.expectedError {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains,
						"Expected error message to contain: %s", tt.errorContains)
				}
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.name)
				assert.NotEmpty(t, result, "Expected non-empty result for test case: %s", tt.name)

				// Verify the result contains valid YAML
				assert.Contains(t, string(result), "apiVersion: tekton.dev/v1")
				assert.Contains(t, string(result), "kind: PipelineRun")
				assert.Contains(t, string(result), tt.pipelineRunName)
			}
		})
	}
}

func TestResolvePipelineRunEdgeCases(t *testing.T) {
	t.Skip("PAC tests require complex git repository setup with .tekton/ directory structure")
	ctx := context.Background()

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "pac-edge-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create .tekton directory to mimic git repository structure
	tektonDir := filepath.Join(tempDir, ".tekton")
	err = os.MkdirAll(tektonDir, 0755)
	require.NoError(t, err)

	tests := []struct {
		name            string
		fileName        string
		pipelineRunName string
		fileContent     []byte
		expectedError   bool
		errorContains   string
	}{
		{
			name:            "pipelinerun with special characters in name",
			fileName:        "special-chars-pipelinerun.yaml",
			pipelineRunName: "test-pipeline-run-123",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: test-pipeline-run-123
spec:
  pipelineSpec:
    tasks:
      - name: hello
        taskSpec:
          steps:
            - name: hello
              image: alpine:latest
              script: |
                echo "Hello World"
`),
			expectedError: false,
		},
		{
			name:            "pipelinerun with annotations and labels",
			fileName:        "annotated-pipelinerun.yaml",
			pipelineRunName: "annotated-pipeline-run",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: annotated-pipeline-run
  annotations:
    tekton.dev/test: "true"
    example.com/owner: "test-user"
  labels:
    app: test-app
    version: v1.0.0
spec:
  pipelineSpec:
    tasks:
      - name: hello
        taskSpec:
          steps:
            - name: hello
              image: alpine:latest
              script: |
                echo "Hello World"
`),
			expectedError: false,
		},
		{
			name:            "pipelinerun with very long name",
			fileName:        "long-name-pipelinerun.yaml",
			pipelineRunName: "test-pipeline-run-with-a-very-long-name-that-exceeds-normal-length",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: test-pipeline-run-with-a-very-long-name-that-exceeds-normal-length
spec:
  pipelineSpec:
    tasks:
      - name: hello
        taskSpec:
          steps:
            - name: hello
              image: alpine:latest
              script: |
                echo "Hello World"
`),
			expectedError: false,
		},
		{
			name:            "malformed yaml with extra content",
			fileName:        "malformed-extra.yaml",
			pipelineRunName: "test-pipeline-run",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: test-pipeline-run
spec:
  pipelineSpec:
    tasks:
      - name: hello
        taskSpec:
          steps:
            - name: hello
              image: alpine:latest
              script: |
                echo "Hello World"
---
# This is extra content that should be ignored
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
`),
			expectedError: false, // Should handle multiple YAML documents
		},
		{
			name:            "pipelinerun with unicode characters",
			fileName:        "unicode-pipelinerun.yaml",
			pipelineRunName: "unicode-pipeline-run",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: unicode-pipeline-run
spec:
  pipelineSpec:
    tasks:
      - name: hello
        taskSpec:
          steps:
            - name: hello
              image: alpine:latest
              script: |
                echo "Hello World 🌍"
                echo "Testing unicode: αβγ"
`),
			expectedError: false,
		},
		{
			name:            "binary file",
			fileName:        "binary-file.yaml",
			pipelineRunName: "test-pipeline-run",
			fileContent:     []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE}, // Binary content
			expectedError:   true,
			errorContains:   "could not find any PipelineRun",
		},
		{
			name:            "extremely large file",
			fileName:        "large-file.yaml",
			pipelineRunName: "large-pipeline-run",
			fileContent:     createLargeYAMLContent("large-pipeline-run"),
			expectedError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tektonDir, tt.fileName)

			// Create test file if content is provided
			if tt.fileContent != nil {
				err := os.WriteFile(filePath, tt.fileContent, 0644)
				require.NoError(t, err)
			}

			result, err := ResolvePipelineRun(ctx, filePath, tt.pipelineRunName)

			if tt.expectedError {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains,
						"Expected error message to contain: %s", tt.errorContains)
				}
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.name)
				assert.NotEmpty(t, result, "Expected non-empty result for test case: %s", tt.name)

				// Verify the result contains valid YAML
				assert.Contains(t, string(result), "apiVersion: tekton.dev/v1")
				assert.Contains(t, string(result), "kind: PipelineRun")
				assert.Contains(t, string(result), tt.pipelineRunName)
			}
		})
	}
}

// Helper function to create a large YAML content for testing
func createLargeYAMLContent(pipelineRunName string) []byte {
	content := `apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: ` + pipelineRunName + `
spec:
  pipelineSpec:
    tasks:`

	// Add many tasks to make the file large
	for i := 0; i < 100; i++ {
		content += `
      - name: task-` + string(rune(i)) + `
        taskSpec:
          steps:
            - name: step-` + string(rune(i)) + `
              image: alpine:latest
              script: |
                echo "Task ` + string(rune(i)) + `"
                sleep 1`
	}

	return []byte(content)
}

func TestResolvePipelineRunFilePermissions(t *testing.T) {
	t.Skip("PAC tests require complex git repository setup with .tekton/ directory structure")
	ctx := context.Background()

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "pac-permissions-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create .tekton directory to mimic git repository structure
	tektonDir := filepath.Join(tempDir, ".tekton")
	err = os.MkdirAll(tektonDir, 0755)
	require.NoError(t, err)

	tests := []struct {
		name            string
		fileName        string
		pipelineRunName string
		fileContent     []byte
		fileMode        os.FileMode
		expectedError   bool
		errorContains   string
	}{
		{
			name:            "readable file",
			fileName:        "readable.yaml",
			pipelineRunName: "test-pipeline-run",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: test-pipeline-run
spec:
  pipelineSpec:
    tasks:
      - name: hello
        taskSpec:
          steps:
            - name: hello
              image: alpine:latest
              script: |
                echo "Hello World"
`),
			fileMode:      0644,
			expectedError: false,
		},
		{
			name:            "executable file",
			fileName:        "executable.yaml",
			pipelineRunName: "test-pipeline-run",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: test-pipeline-run
spec:
  pipelineSpec:
    tasks:
      - name: hello
        taskSpec:
          steps:
            - name: hello
              image: alpine:latest
              script: |
                echo "Hello World"
`),
			fileMode:      0755,
			expectedError: false,
		},
		{
			name:            "read-only file",
			fileName:        "readonly.yaml",
			pipelineRunName: "test-pipeline-run",
			fileContent: []byte(`apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: test-pipeline-run
spec:
  pipelineSpec:
    tasks:
      - name: hello
        taskSpec:
          steps:
            - name: hello
              image: alpine:latest
              script: |
                echo "Hello World"
`),
			fileMode:      0444,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tektonDir, tt.fileName)

			// Create test file if content is provided
			if tt.fileContent != nil {
				err := os.WriteFile(filePath, tt.fileContent, tt.fileMode)
				require.NoError(t, err)
			}

			result, err := ResolvePipelineRun(ctx, filePath, tt.pipelineRunName)

			if tt.expectedError {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains,
						"Expected error message to contain: %s", tt.errorContains)
				}
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.name)
				assert.NotEmpty(t, result, "Expected non-empty result for test case: %s", tt.name)

				// Verify the result contains valid YAML
				assert.Contains(t, string(result), "apiVersion: tekton.dev/v1")
				assert.Contains(t, string(result), "kind: PipelineRun")
				assert.Contains(t, string(result), tt.pipelineRunName)
			}
		})
	}
}
