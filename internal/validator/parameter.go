package validator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/go-multierror"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// paramRefRegex matches parameter references in the format $(params.param-name)
var paramRefRegex = regexp.MustCompile(`\$\(params\.([^)]*)\)`)

// ValidateParameterReferences validates that all parameter references in the pipeline YAML
// match the defined parameters in the pipeline spec
func ValidateParameterReferences(pipelineSpec v1.PipelineSpec, rawYAML []byte) error {
	var err error

	// Extract all parameter references from the YAML content
	paramRefs := extractParameterReferences(string(rawYAML))

	// Create a map of defined parameters for quick lookup
	definedParams := make(map[string]bool)
	for _, param := range pipelineSpec.Params {
		definedParams[param.Name] = true
	}

	// Check each parameter reference
	for _, paramRef := range paramRefs {
		if paramRef == "" {
			err = multierror.Append(err, fmt.Errorf(
				"parameter reference $(params.) not defined in pipeline spec"))
		} else if !definedParams[paramRef] {
			err = multierror.Append(err, fmt.Errorf(
				"parameter reference $(params.%s) not defined in pipeline spec",
				paramRef))
		}
	}

	return err
}

// extractParameterReferences extracts all unique parameter references from the YAML content
func extractParameterReferences(yamlContent string) []string {
	matches := paramRefRegex.FindAllStringSubmatch(yamlContent, -1)
	paramRefs := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 {
			paramName := strings.TrimSpace(match[1])
			// Include empty parameter names to catch validation errors
			paramRefs[paramName] = true
		}
	}

	// Convert map to slice for consistent ordering
	var result []string
	for paramName := range paramRefs {
		result = append(result, paramName)
	}

	return result
}

func ValidateParameters(params v1.Params, specs v1.ParamSpecs) error {
	return validatePipelineTaskParameters(params, specs)
}

func validatePipelineTaskParameters(pipelineTaskParams []v1.Param, taskParams []v1.ParamSpec) error {
	var err error
	for _, pipelineTaskParam := range pipelineTaskParams {
		taskParam, found := getTaskParam(pipelineTaskParam.Name, taskParams)
		if !found {
			err = multierror.Append(err, fmt.Errorf(
				"%q parameter is not defined by the Task",
				pipelineTaskParam.Name))
			continue
		}

		// Tekton uses the "string" type for parameters by default.
		taskParamType := string(taskParam.Type)
		if taskParamType == "" {
			taskParamType = "string"
		}
		pipelineTaskParamType := string(pipelineTaskParam.Value.Type)
		if pipelineTaskParamType == "" {
			pipelineTaskParamType = "string"
		}

		if pipelineTaskParamType != taskParamType {
			err = multierror.Append(err, fmt.Errorf(
				"%q parameter has the incorrect type, got %q, want %q",
				pipelineTaskParam.Name, pipelineTaskParamType, taskParamType))
		}
	}

	// Verify all "required" parameters are fulfilled.
	for _, taskParam := range taskParams {
		if taskParam.Default != nil {
			// Task parameters with a default value are not required.
			continue
		}
		if _, found := getPipelineTaskParam(taskParam.Name, pipelineTaskParams); !found {
			err = multierror.Append(err, fmt.Errorf("%q parameter is required", taskParam.Name))
		}
	}

	return err
}

func getPipelineTaskParam(name string, pipelineTaskParams []v1.Param) (v1.Param, bool) {
	for _, pipelineTaskParam := range pipelineTaskParams {
		if pipelineTaskParam.Name == name {
			return pipelineTaskParam, true
		}
	}
	return v1.Param{}, false
}

func getTaskParam(name string, taskParams []v1.ParamSpec) (v1.ParamSpec, bool) {
	for _, taskParam := range taskParams {
		if taskParam.Name == name {
			return taskParam, true
		}
	}
	return v1.ParamSpec{}, false
}
