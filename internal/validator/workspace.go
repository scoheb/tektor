package validator

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// ValidateWorkspaces validates workspace usage across the pipeline
func ValidateWorkspaces(pipelineSpec v1.PipelineSpec, allTaskSpecs map[string]*v1.TaskSpec) error {
	var err error

	// Create a map of pipeline workspaces for quick lookup
	pipelineWorkspaces := make(map[string]v1.PipelineWorkspaceDeclaration)
	for _, workspace := range pipelineSpec.Workspaces {
		pipelineWorkspaces[workspace.Name] = workspace
	}

	// Validate workspace usage in each pipeline task
	allTasks := append(pipelineSpec.Tasks, pipelineSpec.Finally...)
	for _, pipelineTask := range allTasks {
		taskSpec, exists := allTaskSpecs[pipelineTask.Name]
		if !exists {
			// Skip if we don't have the task spec (already handled by other validators)
			continue
		}

		// Validate task workspace requirements
		if taskErr := validateTaskWorkspaces(pipelineTask, taskSpec, pipelineWorkspaces); taskErr != nil {
			err = multierror.Append(err, fmt.Errorf("task %s workspace validation: %w", pipelineTask.Name, taskErr))
		}
	}

	// Validate that declared pipeline workspaces are actually used
	if unusedErr := validateUnusedPipelineWorkspaces(pipelineSpec, pipelineWorkspaces); unusedErr != nil {
		err = multierror.Append(err, unusedErr)
	}

	return err
}

// validateTaskWorkspaces validates workspace usage for a specific task
func validateTaskWorkspaces(pipelineTask v1.PipelineTask, taskSpec *v1.TaskSpec, pipelineWorkspaces map[string]v1.PipelineWorkspaceDeclaration) error {
	var err error

	// Create maps for quick lookup
	taskWorkspaceBindings := make(map[string]v1.WorkspacePipelineTaskBinding)
	for _, binding := range pipelineTask.Workspaces {
		taskWorkspaceBindings[binding.Name] = binding
	}

	taskWorkspaceDeclarations := make(map[string]v1.WorkspaceDeclaration)
	for _, decl := range taskSpec.Workspaces {
		taskWorkspaceDeclarations[decl.Name] = decl
	}

	// Check that all task workspace declarations have corresponding bindings
	for _, workspaceDecl := range taskSpec.Workspaces {
		binding, hasBinding := taskWorkspaceBindings[workspaceDecl.Name]
		if !hasBinding {
			// Check if workspace is optional
			if workspaceDecl.Optional {
				continue // Optional workspaces don't need bindings
			}
			err = multierror.Append(err, fmt.Errorf("required workspace %q is not provided", workspaceDecl.Name))
			continue
		}

		// Validate that the referenced pipeline workspace exists
		if binding.Workspace != "" {
			if _, exists := pipelineWorkspaces[binding.Workspace]; !exists {
				err = multierror.Append(err, fmt.Errorf("workspace binding %q references non-existent pipeline workspace %q", workspaceDecl.Name, binding.Workspace))
			}
		}

		// Validate workspace requirements (readOnly, mountPath conflicts, etc.)
		if reqErr := validateWorkspaceRequirements(workspaceDecl, binding); reqErr != nil {
			err = multierror.Append(err, reqErr)
		}
	}

	// Check that all workspace bindings reference valid task workspaces
	for _, binding := range pipelineTask.Workspaces {
		if _, exists := taskWorkspaceDeclarations[binding.Name]; !exists {
			err = multierror.Append(err, fmt.Errorf("workspace binding %q does not match any task workspace declaration", binding.Name))
		}
	}

	return err
}

// validateWorkspaceRequirements validates specific workspace requirements
func validateWorkspaceRequirements(decl v1.WorkspaceDeclaration, binding v1.WorkspacePipelineTaskBinding) error {
	var err error

	// Validate readOnly requirements (check if binding has readOnly field and compare)
	// Note: In Tekton v1, readOnly is typically handled at runtime, but we can check for basic compatibility

	// Validate mountPath conflicts - if task declares a mountPath and binding also has subPath
	if decl.MountPath != "" && binding.SubPath != "" {
		// This could potentially cause path conflicts
		if strings.HasPrefix(binding.SubPath, "/") {
			err = multierror.Append(err, fmt.Errorf("workspace %q: task declares mountPath %q but binding uses absolute subPath %q which may cause conflicts", decl.Name, decl.MountPath, binding.SubPath))
		}
	}

	return err
}

// validateUnusedPipelineWorkspaces checks for declared but unused pipeline workspaces
func validateUnusedPipelineWorkspaces(pipelineSpec v1.PipelineSpec, pipelineWorkspaces map[string]v1.PipelineWorkspaceDeclaration) error {
	var err error

	// Track which pipeline workspaces are actually used
	usedWorkspaces := make(map[string]bool)

	// Check all tasks (including finally tasks)
	allTasks := append(pipelineSpec.Tasks, pipelineSpec.Finally...)
	for _, pipelineTask := range allTasks {
		for _, binding := range pipelineTask.Workspaces {
			if binding.Workspace != "" {
				usedWorkspaces[binding.Workspace] = true
			}
		}
	}

	// Report unused pipeline workspaces as warnings (not errors)
	for workspaceName := range pipelineWorkspaces {
		if !usedWorkspaces[workspaceName] {
			// Note: This is more of a warning than an error, but we'll report it
			err = multierror.Append(err, fmt.Errorf("pipeline workspace %q is declared but never used", workspaceName))
		}
	}

	return err
}

// ValidateWorkspaceBindings validates workspace bindings in pipeline tasks against task specifications
func ValidateWorkspaceBindings(pipelineTask v1.PipelineTask, taskSpec *v1.TaskSpec, availableWorkspaces map[string]v1.PipelineWorkspaceDeclaration) error {
	var err error

	// Quick validation for a single task - used by the main pipeline validator
	if taskErr := validateTaskWorkspaces(pipelineTask, taskSpec, availableWorkspaces); taskErr != nil {
		err = multierror.Append(err, taskErr)
	}

	return err
}
