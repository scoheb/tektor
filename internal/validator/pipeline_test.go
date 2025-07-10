package validator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// Helper function to unmarshal YAML into Pipeline objects
func pipelineFromYAML(yamlContent string) (v1.Pipeline, error) {
	var pipeline v1.Pipeline
	err := yaml.Unmarshal([]byte(yamlContent), &pipeline)
	return pipeline, err
}

func TestValidatePipeline(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		pipelineYAML  string
		expectedError bool
		errorContains []string
	}{
		{
			name: "valid pipeline with embedded tasks",
			pipelineYAML: `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: valid-pipeline
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
        results:
          - name: commit
            type: string
        steps:
          - name: clone
            image: alpine/git:latest
            script: git clone $(params.url) -b $(params.revision)
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
            script: echo 'Building commit $(params.commit)'
      params:
        - name: commit
          value: $(tasks.clone.results.commit)
`,
			expectedError: false,
		},
		{
			name: "pipeline with parameter validation errors",
			pipelineYAML: `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: invalid-pipeline-params
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
            script: git clone $(params.url) -b $(params.revision)
      params:
        - name: url
          value: $(params.gitUrl)
        - name: undefinedParam
          value: value
`,
			expectedError: true,
			errorContains: []string{
				"\"revision\" parameter is required",
				"\"undefinedParam\" parameter is not defined by the Task",
			},
		},
		{
			name: "pipeline with result validation errors",
			pipelineYAML: `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: invalid-pipeline-results
spec:
  tasks:
    - name: clone
      taskSpec:
        results:
          - name: commit
            type: string
        steps:
          - name: clone
            image: alpine/git:latest
            script: echo 'cloning'
    - name: build
      taskSpec:
        params:
          - name: commit
            type: string
        steps:
          - name: build
            image: alpine:latest
            script: echo 'building'
      params:
        - name: commit
          value: $(tasks.nonexistent.results.commit)
    - name: test
      taskSpec:
        params:
          - name: info
            type: string
        steps:
          - name: test
            image: alpine:latest
            script: echo 'testing'
      params:
        - name: info
          value: $(tasks.clone.results.nonexistent)
`,
			expectedError: true,
			errorContains: []string{
				"commit result from non-existent nonexistent PipelineTask",
				"non-existent nonexistent result from clone PipelineTask",
			},
		},
		{
			name: "pipeline with workspace validation errors",
			pipelineYAML: `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: invalid-pipeline-workspaces
spec:
  workspaces:
    - name: source
      description: Source workspace
  tasks:
    - name: build
      taskSpec:
        workspaces:
          - name: source
            description: Source workspace
          - name: cache
            description: Cache workspace
        steps:
          - name: build
            image: alpine:latest
            script: echo 'building'
      workspaces:
        - name: source
          workspace: source
        - name: cache
          workspace: nonexistent-workspace
`,
			expectedError: true,
			errorContains: []string{
				"workspace binding \"cache\" references non-existent pipeline workspace \"nonexistent-workspace\"",
			},
		},
		{
			name: "pipeline with missing required workspace",
			pipelineYAML: `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: missing-required-workspace
spec:
  workspaces:
    - name: source
      description: Source workspace
  tasks:
    - name: build
      taskSpec:
        workspaces:
          - name: source
            description: Source workspace
          - name: cache
            description: Cache workspace
        steps:
          - name: build
            image: alpine:latest
            script: echo 'building'
      workspaces:
        - name: source
          workspace: source
`,
			expectedError: true,
			errorContains: []string{
				"required workspace \"cache\" is not provided",
			},
		},
		{
			name: "pipeline with circular dependency",
			pipelineYAML: `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: circular-dependency
spec:
  tasks:
    - name: task-a
      taskSpec:
        params:
          - name: input
            type: string
        results:
          - name: output
            type: string
        steps:
          - name: step1
            image: alpine:latest
            script: echo 'Task A'
      params:
        - name: input
          value: $(tasks.task-b.results.output)
    - name: task-b
      taskSpec:
        params:
          - name: input
            type: string
        results:
          - name: output
            type: string
        steps:
          - name: step1
            image: alpine:latest
            script: echo 'Task B'
      params:
        - name: input
          value: $(tasks.task-a.results.output)
`,
			expectedError: true,
			errorContains: []string{
				"cycle detected",
			},
		},

		{
			name: "pipeline with invalid git resolver params",
			pipelineYAML: `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: invalid-git-resolver
spec:
  tasks:
    - name: clone
      taskRef:
        resolver: git
        params:
          - name: url
            value: https://github.com/example/repo.git
          - name: invalidParam
            value: value
`,
			expectedError: true,
			errorContains: []string{
				"required parameter \"pathInRepo\" is missing",
			},
		},
		{
			name: "pipeline with missing required git resolver params",
			pipelineYAML: `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: missing-git-resolver-params
spec:
  tasks:
    - name: clone
      taskRef:
        resolver: git
        params:
          - name: url
            value: https://github.com/example/repo.git
`,
			expectedError: true,
			errorContains: []string{
				"required parameter \"pathInRepo\" is missing",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline, err := pipelineFromYAML(tt.pipelineYAML)
			require.NoError(t, err, "Failed to unmarshal YAML")

			err = ValidatePipeline(ctx, pipeline)

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

func TestValidatePipelineWithYAML(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		pipeline      v1.Pipeline
		rawYAML       []byte
		expectedError bool
		errorContains []string
	}{
		{
			name: "pipeline with parameter reference validation",
			pipeline: v1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "param-ref-pipeline",
				},
				Spec: v1.PipelineSpec{
					Params: []v1.ParamSpec{
						{Name: "gitUrl", Type: v1.ParamTypeString},
						{Name: "gitRevision", Type: v1.ParamTypeString},
					},
					Tasks: []v1.PipelineTask{
						{
							Name: "clone",
							TaskSpec: &v1.EmbeddedTask{
								TaskSpec: v1.TaskSpec{
									Params: []v1.ParamSpec{
										{Name: "url", Type: v1.ParamTypeString},
									},
									Steps: []v1.Step{
										{
											Name:   "clone",
											Image:  "alpine/git:latest",
											Script: "git clone $(params.url)",
										},
									},
								},
							},
							Params: []v1.Param{
								{Name: "url", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "$(params.gitUrl)"}},
							},
						},
					},
				},
			},
			rawYAML: []byte(`
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: param-ref-pipeline
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
        - name: undefinedParam
          value: $(params.undefinedParameter)
`),
			expectedError: true,
			errorContains: []string{
				"parameter reference $(params.undefinedParameter) not defined in pipeline spec",
			},
		},
		{
			name: "pipeline with valid parameter references",
			pipeline: v1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-param-ref-pipeline",
				},
				Spec: v1.PipelineSpec{
					Params: []v1.ParamSpec{
						{Name: "gitUrl", Type: v1.ParamTypeString},
						{Name: "gitRevision", Type: v1.ParamTypeString},
					},
					Tasks: []v1.PipelineTask{
						{
							Name: "clone",
							TaskSpec: &v1.EmbeddedTask{
								TaskSpec: v1.TaskSpec{
									Params: []v1.ParamSpec{
										{Name: "url", Type: v1.ParamTypeString},
										{Name: "revision", Type: v1.ParamTypeString},
									},
									Steps: []v1.Step{
										{
											Name:   "clone",
											Image:  "alpine/git:latest",
											Script: "git clone $(params.url) -b $(params.revision)",
										},
									},
								},
							},
							Params: []v1.Param{
								{Name: "url", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "$(params.gitUrl)"}},
								{Name: "revision", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "$(params.gitRevision)"}},
							},
						},
					},
				},
			},
			rawYAML: []byte(`
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: valid-param-ref-pipeline
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
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePipelineWithYAML(ctx, tt.pipeline, tt.rawYAML)

			if tt.expectedError {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				if len(tt.errorContains) > 0 {
					errStr := err.Error()
					for _, expectedErr := range tt.errorContains {
						assert.Contains(t, errStr, expectedErr, "Expected error message to contain: %s", expectedErr)
					}
				}
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.name)
			}
		})
	}
}

func TestValidatePipelineWithYAMLAndParams(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		pipeline      v1.Pipeline
		rawYAML       []byte
		runtimeParams map[string]string
		expectedError bool
		errorContains []string
	}{
		{
			name: "pipeline with runtime parameter substitution",
			pipeline: v1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "runtime-param-pipeline",
				},
				Spec: v1.PipelineSpec{
					Params: []v1.ParamSpec{
						{Name: "gitUrl", Type: v1.ParamTypeString},
						{Name: "gitRevision", Type: v1.ParamTypeString},
					},
					Tasks: []v1.PipelineTask{
						{
							Name: "clone",
							TaskSpec: &v1.EmbeddedTask{
								TaskSpec: v1.TaskSpec{
									Params: []v1.ParamSpec{
										{Name: "url", Type: v1.ParamTypeString},
										{Name: "revision", Type: v1.ParamTypeString},
									},
									Steps: []v1.Step{
										{
											Name:   "clone",
											Image:  "alpine/git:latest",
											Script: "git clone $(params.url) -b $(params.revision)",
										},
									},
								},
							},
							Params: []v1.Param{
								{Name: "url", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "$(params.gitUrl)"}},
								{Name: "revision", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "$(params.gitRevision)"}},
							},
						},
					},
				},
			},
			rawYAML: []byte(`
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: runtime-param-pipeline
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
			runtimeParams: map[string]string{
				"gitUrl":      "https://github.com/example/repo.git",
				"gitRevision": "main",
			},
			expectedError: false,
		},
		{
			name: "pipeline with missing runtime parameters",
			pipeline: v1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "missing-runtime-param-pipeline",
				},
				Spec: v1.PipelineSpec{
					Params: []v1.ParamSpec{
						{Name: "gitUrl", Type: v1.ParamTypeString},
						{Name: "gitRevision", Type: v1.ParamTypeString},
					},
					Tasks: []v1.PipelineTask{
						{
							Name: "clone",
							TaskSpec: &v1.EmbeddedTask{
								TaskSpec: v1.TaskSpec{
									Params: []v1.ParamSpec{
										{Name: "url", Type: v1.ParamTypeString},
									},
									Steps: []v1.Step{
										{
											Name:   "clone",
											Image:  "alpine/git:latest",
											Script: "git clone $(params.url)",
										},
									},
								},
							},
							Params: []v1.Param{
								{Name: "url", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "$(params.gitUrl)"}},
							},
						},
					},
				},
			},
			rawYAML: []byte(`
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: missing-runtime-param-pipeline
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
`),
			runtimeParams: map[string]string{
				"gitUrl": "https://github.com/example/repo.git",
				// Missing gitRevision parameter - should still be valid since it's not used
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePipelineWithYAMLAndParams(ctx, tt.pipeline, tt.rawYAML, tt.runtimeParams)

			if tt.expectedError {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				if len(tt.errorContains) > 0 {
					errStr := err.Error()
					for _, expectedErr := range tt.errorContains {
						assert.Contains(t, errStr, expectedErr, "Expected error message to contain: %s", expectedErr)
					}
				}
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.name)
			}
		})
	}
}

func TestPipelineWithGitResolver(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		pipeline      v1.Pipeline
		expectedError bool
		errorContains []string
		skipTest      bool // Skip tests that require network access or git resolver setup
	}{
		{
			name: "pipeline with git resolver - invalid params",
			pipeline: v1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "git-resolver-pipeline",
				},
				Spec: v1.PipelineSpec{
					Tasks: []v1.PipelineTask{
						{
							Name: "clone",
							TaskRef: &v1.TaskRef{
								ResolverRef: v1.ResolverRef{
									Resolver: "git",
									Params: v1.Params{
										{Name: "url", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "invalid-url"}},
										{Name: "pathInRepo", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "task.yaml"}},
									},
								},
							},
						},
					},
				},
			},
			expectedError: true,
			errorContains: []string{
				"git resolver parameter validation failed",
			},
		},
		{
			name: "pipeline with git resolver - missing required params",
			pipeline: v1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "git-resolver-missing-params",
				},
				Spec: v1.PipelineSpec{
					Tasks: []v1.PipelineTask{
						{
							Name: "clone",
							TaskRef: &v1.TaskRef{
								ResolverRef: v1.ResolverRef{
									Resolver: "git",
									Params: v1.Params{
										{Name: "url", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "https://github.com/example/repo.git"}},
										// Missing pathInRepo parameter
									},
								},
							},
						},
					},
				},
			},
			expectedError: true,
			errorContains: []string{
				"git resolver parameter validation failed",
				"required parameter \"pathInRepo\" is missing",
			},
		},
		{
			name: "pipeline with git resolver - valid params but skip actual resolution",
			pipeline: v1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "git-resolver-valid-params",
				},
				Spec: v1.PipelineSpec{
					Tasks: []v1.PipelineTask{
						{
							Name: "clone",
							TaskRef: &v1.TaskRef{
								ResolverRef: v1.ResolverRef{
									Resolver: "git",
									Params: v1.Params{
										{Name: "url", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "https://github.com/tektoncd/catalog.git"}},
										{Name: "pathInRepo", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "task/git-clone/0.9/git-clone.yaml"}},
										{Name: "revision", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "main"}},
									},
								},
							},
						},
					},
				},
			},
			skipTest: true, // Skip because it requires network access
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipTest {
				t.Skip("Skipping test that requires network access or external dependencies")
				return
			}

			err := ValidatePipeline(ctx, tt.pipeline)

			if tt.expectedError {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				if len(tt.errorContains) > 0 {
					errStr := err.Error()
					for _, expectedErr := range tt.errorContains {
						assert.Contains(t, errStr, expectedErr, "Expected error message to contain: %s", expectedErr)
					}
				}
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.name)
			}
		})
	}
}

func TestExtractResultReferencesFromValue(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected []*v1.ResultRef
	}{
		{
			name:  "single result reference",
			value: "$(tasks.clone.results.commit)",
			expected: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "commit"},
			},
		},
		{
			name:  "multiple result references",
			value: "$(tasks.clone.results.commit) and $(tasks.build.results.image)",
			expected: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "commit"},
				{PipelineTask: "build", Result: "image"},
			},
		},
		{
			name:     "no result references",
			value:    "static value",
			expected: []*v1.ResultRef{},
		},
		{
			name:  "result reference with array indexing",
			value: "$(tasks.clone.results.files[0])",
			expected: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "files"},
			},
		},
		{
			name:  "result reference with object property",
			value: "$(tasks.clone.results.metadata.author)",
			expected: []*v1.ResultRef{
				{PipelineTask: "clone", Result: "metadata"},
			},
		},
		{
			name:     "invalid result reference format",
			value:    "$(tasks.clone.result.commit)", // Missing 's' in 'results'
			expected: []*v1.ResultRef{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractResultReferencesFromValue(tt.value)
			assert.Equal(t, len(tt.expected), len(result), "Number of result references should match")

			for i, expected := range tt.expected {
				if i < len(result) {
					assert.Equal(t, expected.PipelineTask, result[i].PipelineTask, "PipelineTask should match")
					assert.Equal(t, expected.Result, result[i].Result, "Result should match")
				}
			}
		})
	}
}

func TestValidateGitResolverParams(t *testing.T) {
	tests := []struct {
		name          string
		params        v1.Params
		expectedError bool
		errorContains string
	}{
		{
			name: "valid git resolver params",
			params: v1.Params{
				{Name: "url", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "https://github.com/example/repo.git"}},
				{Name: "pathInRepo", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "task.yaml"}},
				{Name: "revision", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "main"}},
			},
			expectedError: false,
		},
		{
			name: "missing url parameter",
			params: v1.Params{
				{Name: "pathInRepo", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "task.yaml"}},
			},
			expectedError: true,
			errorContains: "required parameter \"url\" is missing",
		},
		{
			name: "missing pathInRepo parameter",
			params: v1.Params{
				{Name: "url", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "https://github.com/example/repo.git"}},
			},
			expectedError: true,
			errorContains: "required parameter \"pathInRepo\" is missing",
		},
		{
			name: "invalid url format",
			params: v1.Params{
				{Name: "url", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "invalid-url"}},
				{Name: "pathInRepo", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "task.yaml"}},
			},
			expectedError: true,
			errorContains: "invalid git URL format or parameter reference",
		},
		{
			name: "invalid path format",
			params: v1.Params{
				{Name: "url", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "https://github.com/example/repo.git"}},
				{Name: "pathInRepo", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "/absolute/path"}},
			},
			expectedError: true,
			errorContains: "invalid path format or parameter reference",
		},
		{
			name: "parameter reference in url",
			params: v1.Params{
				{Name: "url", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "$(params.gitUrl)"}},
				{Name: "pathInRepo", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "task.yaml"}},
			},
			expectedError: false, // Parameter references should be valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGitResolverParams(tt.params)

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
