package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"sigs.k8s.io/yaml"
)

// Helper functions to unmarshal YAML into workspace-related objects
func pipelineSpecFromYAML(yamlContent string) (v1.PipelineSpec, error) {
	var spec v1.PipelineSpec
	err := yaml.Unmarshal([]byte(yamlContent), &spec)
	return spec, err
}

func taskSpecFromYAML(yamlContent string) (v1.TaskSpec, error) {
	var spec v1.TaskSpec
	err := yaml.Unmarshal([]byte(yamlContent), &spec)
	return spec, err
}

func TestValidateWorkspaces(t *testing.T) {
	tests := []struct {
		name             string
		pipelineSpecYAML string
		allTaskSpecs     map[string]*v1.TaskSpec
		expectedErrors   []string
		expectNoError    bool
	}{
		{
			name: "valid workspace usage",
			pipelineSpecYAML: `
workspaces:
  - name: source
    description: Source code workspace
  - name: cache
    description: Cache workspace
tasks:
  - name: clone
    workspaces:
      - name: output
        workspace: source
  - name: build
    workspaces:
      - name: source
        workspace: source
      - name: cache
        workspace: cache
`,
			allTaskSpecs: map[string]*v1.TaskSpec{
				"clone": {
					Workspaces: []v1.WorkspaceDeclaration{
						{Name: "output", Description: "Output workspace"},
					},
				},
				"build": {
					Workspaces: []v1.WorkspaceDeclaration{
						{Name: "source", Description: "Source workspace"},
						{Name: "cache", Description: "Cache workspace"},
					},
				},
			},
			expectNoError: true,
		},
		{
			name: "missing required workspace",
			pipelineSpecYAML: `
workspaces:
  - name: source
    description: Source code workspace
tasks:
  - name: build
    workspaces:
      - name: source
        workspace: source
`,
			allTaskSpecs: map[string]*v1.TaskSpec{
				"build": {
					Workspaces: []v1.WorkspaceDeclaration{
						{Name: "source", Description: "Source workspace"},
						{Name: "cache", Description: "Cache workspace"}, // Required (no Optional flag)
					},
				},
			},
			expectedErrors: []string{
				"required workspace \"cache\" is not provided",
			},
		},
		{
			name: "optional workspace not provided",
			pipelineSpecYAML: `
workspaces:
  - name: source
    description: Source code workspace
tasks:
  - name: build
    workspaces:
      - name: source
        workspace: source
`,
			allTaskSpecs: map[string]*v1.TaskSpec{
				"build": {
					Workspaces: []v1.WorkspaceDeclaration{
						{Name: "source", Description: "Source workspace"},
						{Name: "cache", Description: "Cache workspace", Optional: true},
					},
				},
			},
			expectNoError: true,
		},
		{
			name: "workspace binding references non-existent pipeline workspace",
			pipelineSpecYAML: `
workspaces:
  - name: source
    description: Source code workspace
tasks:
  - name: build
    workspaces:
      - name: source
        workspace: source
      - name: cache
        workspace: nonexistent-workspace
`,
			allTaskSpecs: map[string]*v1.TaskSpec{
				"build": {
					Workspaces: []v1.WorkspaceDeclaration{
						{Name: "source", Description: "Source workspace"},
						{Name: "cache", Description: "Cache workspace"},
					},
				},
			},
			expectedErrors: []string{
				"workspace binding \"cache\" references non-existent pipeline workspace \"nonexistent-workspace\"",
			},
		},
		{
			name: "workspace binding references non-existent task workspace",
			pipelineSpecYAML: `
workspaces:
  - name: source
    description: Source code workspace
tasks:
  - name: build
    workspaces:
      - name: nonexistent-task-workspace
        workspace: source
`,
			allTaskSpecs: map[string]*v1.TaskSpec{
				"build": {
					Workspaces: []v1.WorkspaceDeclaration{
						{Name: "source", Description: "Source workspace"},
					},
				},
			},
			expectedErrors: []string{
				"workspace binding \"nonexistent-task-workspace\" does not match any task workspace declaration",
			},
		},
		{
			name: "unused pipeline workspace",
			pipelineSpecYAML: `
workspaces:
  - name: source
    description: Source code workspace
  - name: unused
    description: Unused workspace
tasks:
  - name: build
    workspaces:
      - name: source
        workspace: source
`,
			allTaskSpecs: map[string]*v1.TaskSpec{
				"build": {
					Workspaces: []v1.WorkspaceDeclaration{
						{Name: "source", Description: "Source workspace"},
					},
				},
			},
			expectedErrors: []string{
				"pipeline workspace \"unused\" is declared but never used",
			},
		},
		{
			name: "workspace with same pipeline workspace referenced multiple times",
			pipelineSpecYAML: `
workspaces:
  - name: source
    description: Source code workspace
tasks:
  - name: build
    workspaces:
      - name: ws1
        workspace: source
      - name: ws2
        workspace: source
`,
			allTaskSpecs: map[string]*v1.TaskSpec{
				"build": {
					Workspaces: []v1.WorkspaceDeclaration{
						{Name: "ws1", Description: "Workspace 1"},
						{Name: "ws2", Description: "Workspace 2"},
					},
				},
			},
			expectNoError: true, // This is actually valid - same pipeline workspace can be used multiple times
		},
		{
			name: "multiple workspace validation errors",
			pipelineSpecYAML: `
workspaces:
  - name: source
    description: Source code workspace
tasks:
  - name: build
    workspaces:
      - name: source
        workspace: source
      - name: cache
        workspace: nonexistent-workspace
`,
			allTaskSpecs: map[string]*v1.TaskSpec{
				"build": {
					Workspaces: []v1.WorkspaceDeclaration{
						{Name: "source", Description: "Source workspace"},
						{Name: "cache", Description: "Cache workspace"},
						{Name: "missing", Description: "Missing workspace"}, // Required but not provided
					},
				},
			},
			expectedErrors: []string{
				"workspace binding \"cache\" references non-existent pipeline workspace \"nonexistent-workspace\"",
				"required workspace \"missing\" is not provided",
			},
		},
		{
			name: "valid workspace with finally tasks",
			pipelineSpecYAML: `
workspaces:
  - name: source
    description: Source code workspace
tasks:
  - name: build
    workspaces:
      - name: source
        workspace: source
finally:
  - name: cleanup
    workspaces:
      - name: source
        workspace: source
`,
			allTaskSpecs: map[string]*v1.TaskSpec{
				"build": {
					Workspaces: []v1.WorkspaceDeclaration{
						{Name: "source", Description: "Source workspace"},
					},
				},
				"cleanup": {
					Workspaces: []v1.WorkspaceDeclaration{
						{Name: "source", Description: "Source workspace"},
					},
				},
			},
			expectNoError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipelineSpec, err := pipelineSpecFromYAML(tt.pipelineSpecYAML)
			require.NoError(t, err, "Failed to unmarshal YAML")

			err = ValidateWorkspaces(pipelineSpec, tt.allTaskSpecs)

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

func TestValidateTaskWorkspaces(t *testing.T) {
	tests := []struct {
		name               string
		pipelineTask       v1.PipelineTask
		taskSpec           *v1.TaskSpec
		pipelineWorkspaces map[string]v1.PipelineWorkspaceDeclaration
		expectedErrors     []string
		expectNoError      bool
	}{
		{
			name: "valid task workspace bindings",
			pipelineTask: v1.PipelineTask{
				Name: "build",
				Workspaces: []v1.WorkspacePipelineTaskBinding{
					{Name: "source", Workspace: "source"},
					{Name: "cache", Workspace: "cache"},
				},
			},
			taskSpec: &v1.TaskSpec{
				Workspaces: []v1.WorkspaceDeclaration{
					{Name: "source", Description: "Source workspace"},
					{Name: "cache", Description: "Cache workspace"},
				},
			},
			pipelineWorkspaces: map[string]v1.PipelineWorkspaceDeclaration{
				"source": {Name: "source", Description: "Source workspace"},
				"cache":  {Name: "cache", Description: "Cache workspace"},
			},
			expectNoError: true,
		},
		{
			name: "missing required workspace binding",
			pipelineTask: v1.PipelineTask{
				Name: "build",
				Workspaces: []v1.WorkspacePipelineTaskBinding{
					{Name: "source", Workspace: "source"},
				},
			},
			taskSpec: &v1.TaskSpec{
				Workspaces: []v1.WorkspaceDeclaration{
					{Name: "source", Description: "Source workspace"},
					{Name: "cache", Description: "Cache workspace"}, // Required
				},
			},
			pipelineWorkspaces: map[string]v1.PipelineWorkspaceDeclaration{
				"source": {Name: "source", Description: "Source workspace"},
				"cache":  {Name: "cache", Description: "Cache workspace"},
			},
			expectedErrors: []string{
				"required workspace \"cache\" is not provided",
			},
		},
		{
			name: "optional workspace not bound",
			pipelineTask: v1.PipelineTask{
				Name: "build",
				Workspaces: []v1.WorkspacePipelineTaskBinding{
					{Name: "source", Workspace: "source"},
				},
			},
			taskSpec: &v1.TaskSpec{
				Workspaces: []v1.WorkspaceDeclaration{
					{Name: "source", Description: "Source workspace"},
					{Name: "cache", Description: "Cache workspace", Optional: true},
				},
			},
			pipelineWorkspaces: map[string]v1.PipelineWorkspaceDeclaration{
				"source": {Name: "source", Description: "Source workspace"},
				"cache":  {Name: "cache", Description: "Cache workspace"},
			},
			expectNoError: true,
		},
		{
			name: "workspace binding references non-existent pipeline workspace",
			pipelineTask: v1.PipelineTask{
				Name: "build",
				Workspaces: []v1.WorkspacePipelineTaskBinding{
					{Name: "source", Workspace: "nonexistent"},
				},
			},
			taskSpec: &v1.TaskSpec{
				Workspaces: []v1.WorkspaceDeclaration{
					{Name: "source", Description: "Source workspace"},
				},
			},
			pipelineWorkspaces: map[string]v1.PipelineWorkspaceDeclaration{
				"source": {Name: "source", Description: "Source workspace"},
			},
			expectedErrors: []string{
				"workspace binding \"source\" references non-existent pipeline workspace \"nonexistent\"",
			},
		},
		{
			name: "workspace binding references non-existent task workspace",
			pipelineTask: v1.PipelineTask{
				Name: "build",
				Workspaces: []v1.WorkspacePipelineTaskBinding{
					{Name: "nonexistent", Workspace: "source"},
				},
			},
			taskSpec: &v1.TaskSpec{
				Workspaces: []v1.WorkspaceDeclaration{
					{Name: "source", Description: "Source workspace"},
				},
			},
			pipelineWorkspaces: map[string]v1.PipelineWorkspaceDeclaration{
				"source": {Name: "source", Description: "Source workspace"},
			},
			expectedErrors: []string{
				"workspace binding \"nonexistent\" does not match any task workspace declaration",
			},
		},
		{
			name: "workspace with mount path and subpath conflict",
			pipelineTask: v1.PipelineTask{
				Name: "build",
				Workspaces: []v1.WorkspacePipelineTaskBinding{
					{Name: "source", Workspace: "source", SubPath: "/absolute/path"},
				},
			},
			taskSpec: &v1.TaskSpec{
				Workspaces: []v1.WorkspaceDeclaration{
					{Name: "source", Description: "Source workspace", MountPath: "/workspace/source"},
				},
			},
			pipelineWorkspaces: map[string]v1.PipelineWorkspaceDeclaration{
				"source": {Name: "source", Description: "Source workspace"},
			},
			expectedErrors: []string{
				"workspace \"source\": task declares mountPath \"/workspace/source\" but binding uses absolute subPath \"/absolute/path\" which may cause conflicts",
			},
		},
		{
			name: "workspace with relative subpath",
			pipelineTask: v1.PipelineTask{
				Name: "build",
				Workspaces: []v1.WorkspacePipelineTaskBinding{
					{Name: "source", Workspace: "source", SubPath: "relative/path"},
				},
			},
			taskSpec: &v1.TaskSpec{
				Workspaces: []v1.WorkspaceDeclaration{
					{Name: "source", Description: "Source workspace", MountPath: "/workspace/source"},
				},
			},
			pipelineWorkspaces: map[string]v1.PipelineWorkspaceDeclaration{
				"source": {Name: "source", Description: "Source workspace"},
			},
			expectNoError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTaskWorkspaces(tt.pipelineTask, tt.taskSpec, tt.pipelineWorkspaces)

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

func TestValidateWorkspaceRequirements(t *testing.T) {
	tests := []struct {
		name          string
		declaration   v1.WorkspaceDeclaration
		binding       v1.WorkspacePipelineTaskBinding
		expectedError string
		expectNoError bool
	}{
		{
			name: "valid workspace with no conflicts",
			declaration: v1.WorkspaceDeclaration{
				Name:        "source",
				Description: "Source workspace",
				MountPath:   "/workspace/source",
			},
			binding: v1.WorkspacePipelineTaskBinding{
				Name:      "source",
				Workspace: "source",
				SubPath:   "relative/path",
			},
			expectNoError: true,
		},
		{
			name: "workspace with mount path and absolute subpath conflict",
			declaration: v1.WorkspaceDeclaration{
				Name:        "source",
				Description: "Source workspace",
				MountPath:   "/workspace/source",
			},
			binding: v1.WorkspacePipelineTaskBinding{
				Name:      "source",
				Workspace: "source",
				SubPath:   "/absolute/path",
			},
			expectedError: "workspace \"source\": task declares mountPath \"/workspace/source\" but binding uses absolute subPath \"/absolute/path\" which may cause conflicts",
		},
		{
			name: "workspace without mount path",
			declaration: v1.WorkspaceDeclaration{
				Name:        "source",
				Description: "Source workspace",
			},
			binding: v1.WorkspacePipelineTaskBinding{
				Name:      "source",
				Workspace: "source",
				SubPath:   "/absolute/path",
			},
			expectNoError: true,
		},
		{
			name: "workspace with mount path and no subpath",
			declaration: v1.WorkspaceDeclaration{
				Name:        "source",
				Description: "Source workspace",
				MountPath:   "/workspace/source",
			},
			binding: v1.WorkspacePipelineTaskBinding{
				Name:      "source",
				Workspace: "source",
			},
			expectNoError: true,
		},
		{
			name: "workspace with empty mount path and subpath",
			declaration: v1.WorkspaceDeclaration{
				Name:        "source",
				Description: "Source workspace",
				MountPath:   "",
			},
			binding: v1.WorkspacePipelineTaskBinding{
				Name:      "source",
				Workspace: "source",
				SubPath:   "",
			},
			expectNoError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWorkspaceRequirements(tt.declaration, tt.binding)

			if tt.expectNoError {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.name)
			} else {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				assert.Contains(t, err.Error(), tt.expectedError, "Expected error message to contain: %s", tt.expectedError)
			}
		})
	}
}

func TestValidateUnusedPipelineWorkspaces(t *testing.T) {
	tests := []struct {
		name               string
		pipelineSpec       v1.PipelineSpec
		pipelineWorkspaces map[string]v1.PipelineWorkspaceDeclaration
		expectedErrors     []string
		expectNoError      bool
	}{
		{
			name: "all workspaces used",
			pipelineSpec: v1.PipelineSpec{
				Tasks: []v1.PipelineTask{
					{
						Name: "build",
						Workspaces: []v1.WorkspacePipelineTaskBinding{
							{Name: "source", Workspace: "source"},
							{Name: "cache", Workspace: "cache"},
						},
					},
				},
			},
			pipelineWorkspaces: map[string]v1.PipelineWorkspaceDeclaration{
				"source": {Name: "source", Description: "Source workspace"},
				"cache":  {Name: "cache", Description: "Cache workspace"},
			},
			expectNoError: true,
		},
		{
			name: "some workspaces unused",
			pipelineSpec: v1.PipelineSpec{
				Tasks: []v1.PipelineTask{
					{
						Name: "build",
						Workspaces: []v1.WorkspacePipelineTaskBinding{
							{Name: "source", Workspace: "source"},
						},
					},
				},
			},
			pipelineWorkspaces: map[string]v1.PipelineWorkspaceDeclaration{
				"source": {Name: "source", Description: "Source workspace"},
				"cache":  {Name: "cache", Description: "Cache workspace"},
				"unused": {Name: "unused", Description: "Unused workspace"},
			},
			expectedErrors: []string{
				"pipeline workspace \"cache\" is declared but never used",
				"pipeline workspace \"unused\" is declared but never used",
			},
		},
		{
			name: "workspaces used in finally tasks",
			pipelineSpec: v1.PipelineSpec{
				Tasks: []v1.PipelineTask{
					{
						Name: "build",
						Workspaces: []v1.WorkspacePipelineTaskBinding{
							{Name: "source", Workspace: "source"},
						},
					},
				},
				Finally: []v1.PipelineTask{
					{
						Name: "cleanup",
						Workspaces: []v1.WorkspacePipelineTaskBinding{
							{Name: "cache", Workspace: "cache"},
						},
					},
				},
			},
			pipelineWorkspaces: map[string]v1.PipelineWorkspaceDeclaration{
				"source": {Name: "source", Description: "Source workspace"},
				"cache":  {Name: "cache", Description: "Cache workspace"},
			},
			expectNoError: true,
		},
		{
			name: "no workspaces declared",
			pipelineSpec: v1.PipelineSpec{
				Tasks: []v1.PipelineTask{
					{
						Name:       "build",
						Workspaces: []v1.WorkspacePipelineTaskBinding{},
					},
				},
			},
			pipelineWorkspaces: map[string]v1.PipelineWorkspaceDeclaration{},
			expectNoError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUnusedPipelineWorkspaces(tt.pipelineSpec, tt.pipelineWorkspaces)

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

func TestValidateWorkspaceBindings(t *testing.T) {
	tests := []struct {
		name                string
		pipelineTask        v1.PipelineTask
		taskSpec            *v1.TaskSpec
		availableWorkspaces map[string]v1.PipelineWorkspaceDeclaration
		expectedErrors      []string
		expectNoError       bool
	}{
		{
			name: "valid workspace bindings",
			pipelineTask: v1.PipelineTask{
				Name: "build",
				Workspaces: []v1.WorkspacePipelineTaskBinding{
					{Name: "source", Workspace: "source"},
				},
			},
			taskSpec: &v1.TaskSpec{
				Workspaces: []v1.WorkspaceDeclaration{
					{Name: "source", Description: "Source workspace"},
				},
			},
			availableWorkspaces: map[string]v1.PipelineWorkspaceDeclaration{
				"source": {Name: "source", Description: "Source workspace"},
			},
			expectNoError: true,
		},
		{
			name: "invalid workspace bindings",
			pipelineTask: v1.PipelineTask{
				Name: "build",
				Workspaces: []v1.WorkspacePipelineTaskBinding{
					{Name: "source", Workspace: "nonexistent"},
				},
			},
			taskSpec: &v1.TaskSpec{
				Workspaces: []v1.WorkspaceDeclaration{
					{Name: "source", Description: "Source workspace"},
				},
			},
			availableWorkspaces: map[string]v1.PipelineWorkspaceDeclaration{
				"source": {Name: "source", Description: "Source workspace"},
			},
			expectedErrors: []string{
				"workspace binding \"source\" references non-existent pipeline workspace \"nonexistent\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkspaceBindings(tt.pipelineTask, tt.taskSpec, tt.availableWorkspaces)

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

