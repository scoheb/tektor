package validator

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/hashicorp/go-multierror"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/bundle"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/git"
	"sigs.k8s.io/yaml"
)

func ValidatePipeline(ctx context.Context, p v1.Pipeline) error {
	return ValidatePipelineWithYAML(ctx, p, nil)
}

func ValidatePipelineWithYAML(ctx context.Context, p v1.Pipeline, rawYAML []byte) error {
	return ValidatePipelineWithYAMLAndParams(ctx, p, rawYAML, nil)
}

func ValidatePipelineWithYAMLAndParams(ctx context.Context, p v1.Pipeline, rawYAML []byte, runtimeParams map[string]string) error {
	var allErrors error

	// Validate parameter references in the raw YAML content
	if rawYAML != nil {
		if err := ValidateParameterReferences(p.Spec, rawYAML); err != nil {
			allErrors = multierror.Append(allErrors, fmt.Errorf("parameter reference validation: %w", err))
		}
	}

	if err := p.Validate(ctx); err != nil {
		var validationErrors error
		for _, e := range err.WrappedErrors() {
			details := e.Details
			if len(details) > 0 {
				details = " " + details
			}
			message := strings.TrimSuffix(e.Message, ": ")
			for _, p := range e.Paths {
				validationErrors = multierror.Append(validationErrors, fmt.Errorf("%v: %v%v", message, p, details))
			}
			if len(e.Paths) == 0 {
				validationErrors = multierror.Append(validationErrors, fmt.Errorf("%v: %v", message, details))
			}
		}
		allErrors = multierror.Append(allErrors, validationErrors)
	}

	allTaskResults := map[string][]v1.TaskResult{}
	allTaskResultRefs := map[string][]*v1.ResultRef{}
	allTaskSpecs := map[string]*v1.TaskSpec{}

	pipelineTasks := make([]v1.PipelineTask, 0, len(p.Spec.Tasks)+len(p.Spec.Finally))
	pipelineTasks = append(pipelineTasks, p.Spec.Tasks...)
	pipelineTasks = append(pipelineTasks, p.Spec.Finally...)

	// Collect parameter type information for result validation
	parameterTypeContexts := make(map[string]resultUsageContext)

	for i, pipelineTask := range pipelineTasks {
		log.Printf("Processing pipeline task %d: %s", i, pipelineTask.Name)
		allTaskResultRefs[pipelineTask.Name] = v1.PipelineTaskResultRefs(&pipelineTask)
		params := pipelineTask.Params

		taskSpec, err := taskSpecFromPipelineTaskWithParams(ctx, pipelineTask, p.Spec.Params, runtimeParams)
		if err != nil {
			allErrors = multierror.Append(allErrors, fmt.Errorf("retrieving task spec from %s pipeline task: %w", pipelineTask.Name, err))
			continue
		}

		paramSpecs := taskSpec.Params
		allTaskResults[pipelineTask.Name] = taskSpec.Results
		allTaskSpecs[pipelineTask.Name] = taskSpec

		if err := ValidateParameters(params, paramSpecs); err != nil {
			allErrors = multierror.Append(allErrors, fmt.Errorf("ERROR: %s PipelineTask: %s", pipelineTask.Name, err))
		}

		// Check each parameter in this task for result type validation
		for _, param := range pipelineTask.Params {
			if param.Value.StringVal != "" {
				// Extract result references from parameter values
				resultRefs := extractResultReferencesFromValue(param.Value.StringVal)
				for _, resultRef := range resultRefs {
					refKey := fmt.Sprintf("%s.%s", resultRef.PipelineTask, resultRef.Result)

					// Find the parameter spec to get the expected type
					for _, paramSpec := range taskSpec.Params {
						if paramSpec.Name == param.Name {
							expectedType := string(paramSpec.Type)
							if expectedType == "" {
								expectedType = "string" // Default type
							}

							parameterTypeContexts[refKey] = resultUsageContext{
								Location:     fmt.Sprintf("PipelineTask %s parameter %s", pipelineTask.Name, param.Name),
								ExpectedType: expectedType,
								ActualUsage:  param.Value.StringVal,
							}
							break
						}
					}
				}
			}
		}
	}

	// Validate workspace usage
	if workspaceErr := ValidateWorkspaces(p.Spec, allTaskSpecs); workspaceErr != nil {
		allErrors = multierror.Append(allErrors, fmt.Errorf("workspace validation: %w", workspaceErr))
	}

	// Verify result references in PipelineTasks are valid.
	for pipelineTaskName, resultRefs := range allTaskResultRefs {
		if err := ValidateResultsWithContext(resultRefs, allTaskResults, parameterTypeContexts); err != nil {
			allErrors = multierror.Append(allErrors, fmt.Errorf("%s PipelineTask results: %w", pipelineTaskName, err))
		}
	}

	// Verify result references in Pipeline are valid.
	for _, pipelineResult := range p.Spec.Results {
		expressions, _ := pipelineResult.GetVarSubstitutionExpressions()
		resultRefs := v1.NewResultRefs(expressions)

		// Pipeline results are always strings
		pipelineResultContexts := make(map[string]resultUsageContext)
		for _, resultRef := range resultRefs {
			refKey := fmt.Sprintf("%s.%s", resultRef.PipelineTask, resultRef.Result)
			pipelineResultContexts[refKey] = resultUsageContext{
				Location:     fmt.Sprintf("Pipeline result %s", pipelineResult.Name),
				ExpectedType: "string", // Pipeline results are always strings
				ActualUsage:  pipelineResult.Value.StringVal,
			}
		}

		if err := ValidateResultsWithContext(resultRefs, allTaskResults, pipelineResultContexts); err != nil {
			allErrors = multierror.Append(allErrors, fmt.Errorf("pipeline results: %w", err))
		}
	}

	return allErrors
}

func taskSpecFromPipelineTask(ctx context.Context, pipelineTask v1.PipelineTask) (*v1.TaskSpec, error) {
	return taskSpecFromPipelineTaskWithParams(ctx, pipelineTask, nil, nil)
}

func taskSpecFromPipelineTaskWithParams(ctx context.Context, pipelineTask v1.PipelineTask, pipelineParams []v1.ParamSpec, runtimeParams map[string]string) (*v1.TaskSpec, error) {
	// Embedded task spec
	if pipelineTask.TaskSpec != nil {
		// Custom Tasks are not supported
		if pipelineTask.TaskSpec.IsCustomTask() {
			return nil, errors.New("custom Tasks are not supported")
		}
		return &pipelineTask.TaskSpec.TaskSpec, nil
	}

	if pipelineTask.TaskRef != nil && pipelineTask.TaskRef.Resolver == "bundles" {
		opts, err := bundleResolverOptions(ctx, pipelineTask.TaskRef.Params)
		if err != nil {
			return nil, err
		}
		resolvedResource, err := bundle.GetEntry(ctx, authn.DefaultKeychain, opts)
		if err != nil {
			return nil, err
		}

		var t v1.Task
		if err := yaml.Unmarshal(resolvedResource.Data(), &t); err != nil {
			return nil, err
		}

		return &t.Spec, nil
	}

	if pipelineTask.TaskRef != nil && pipelineTask.TaskRef.Resolver == "git" {
		// Validate required parameters for git resolver
		if err := validateGitResolverParams(pipelineTask.TaskRef.Params); err != nil {
			return nil, fmt.Errorf("git resolver parameter validation failed: %w", err)
		}

		// Substitute parameter references with runtime values
		resolverParams := substituteParametersInParams(pipelineTask.TaskRef.Params, pipelineParams, runtimeParams)

		params, err := git.PopulateDefaultParams(ctx, resolverParams)
		if err != nil {
			return nil, fmt.Errorf("failed to populate git resolver parameters: %w", err)
		}

		resolvedResource, err := git.ResolveAnonymousGit(ctx, params)
		if err != nil {
			// Extract URL and revision from params for better error messaging
			var url, revision string
			if urlParam := getParamValue(resolverParams, "url"); urlParam != "" {
				url = urlParam
			}
			if revParam := getParamValue(resolverParams, "revision"); revParam != "" {
				revision = revParam
			} else {
				revision = "default"
			}

			return nil, fmt.Errorf("failed to resolve task from git repository (url: %s, revision: %s): %w", url, revision, err)
		}

		var t v1.Task
		if err := yaml.Unmarshal(resolvedResource.Data(), &t); err != nil {
			return nil, fmt.Errorf("failed to unmarshal task from git repository: %w", err)
		}

		return &t.Spec, nil
	}

	return nil, errors.New("unable to retrieve spec for pipeline task")
}

// substituteParametersInParams substitutes parameter references in resolver parameters
func substituteParametersInParams(params v1.Params, pipelineParams []v1.ParamSpec, runtimeParams map[string]string) v1.Params {
	var result v1.Params

	for _, param := range params {
		newParam := param.DeepCopy()

		// Substitute parameter references in the value
		if newParam.Value.StringVal != "" {
			newParam.Value.StringVal = substituteParameterReferences(newParam.Value.StringVal, pipelineParams, runtimeParams)
		}

		result = append(result, *newParam)
	}

	return result
}

// substituteParameterReferences substitutes $(params.param-name) with actual values
func substituteParameterReferences(value string, pipelineParams []v1.ParamSpec, runtimeParams map[string]string) string {
	// First try runtime parameters
	if runtimeParams != nil {
		for key, val := range runtimeParams {
			paramRef := fmt.Sprintf("$(params.%s)", key)
			value = strings.ReplaceAll(value, paramRef, val)
		}
	}

	// Then try default values from pipeline parameter specs
	if pipelineParams != nil {
		for _, paramSpec := range pipelineParams {
			paramRef := fmt.Sprintf("$(params.%s)", paramSpec.Name)
			if strings.Contains(value, paramRef) && paramSpec.Default != nil {
				defaultVal := ""
				if paramSpec.Default.StringVal != "" {
					defaultVal = paramSpec.Default.StringVal
				}
				value = strings.ReplaceAll(value, paramRef, defaultVal)
			}
		}
	}

	return value
}

func bundleResolverOptions(ctx context.Context, params v1.Params) (bundle.RequestOptions, error) {
	var allParams v1.Params

	// The "serviceAccount" param is required by the resolver, but it's rarely ever set on Pipeline
	// definitions. Add a default value if one is not set.
	hasSAParam := false
	for _, p := range params {
		if p.Name == bundle.ParamServiceAccount {
			hasSAParam = true
			break
		}
	}
	if !hasSAParam {
		allParams = append(allParams, v1.Param{
			Name: bundle.ParamServiceAccount, Value: *v1.NewStructuredValues("none"),
		})
	}

	allParams = append(allParams, params...)
	return bundle.OptionsFromParams(ctx, allParams)
}

// validateGitResolverParams validates the required parameters for git resolver
func validateGitResolverParams(params v1.Params) error {
	var err error

	// Check for required parameters
	requiredParams := []string{"url", "pathInRepo"}
	providedParams := make(map[string]bool)

	for _, param := range params {
		providedParams[param.Name] = true
	}

	for _, required := range requiredParams {
		if !providedParams[required] {
			err = multierror.Append(err, fmt.Errorf("required parameter %q is missing", required))
		}
	}

	// Validate URL parameter if provided
	if urlParam := getParamValue(params, "url"); urlParam != "" {
		if !isValidGitURLOrParamRef(urlParam) {
			err = multierror.Append(err, fmt.Errorf("invalid git URL format or parameter reference: %s", urlParam))
		}
	}

	// Validate pathInRepo parameter if provided
	if pathParam := getParamValue(params, "pathInRepo"); pathParam != "" {
		if !isValidPathOrParamRef(pathParam) {
			err = multierror.Append(err, fmt.Errorf("invalid path format or parameter reference: %s", pathParam))
		}
	}

	return err
}

// getParamValue retrieves the value of a parameter by name
func getParamValue(params v1.Params, name string) string {
	for _, param := range params {
		if param.Name == name {
			return param.Value.StringVal
		}
	}
	return ""
}

// isValidGitURLOrParamRef checks if the provided value is a valid Git repository URL or parameter reference
func isValidGitURLOrParamRef(value string) bool {
	// Check if it's a parameter reference
	if isParameterReference(value) {
		return true
	}

	// Otherwise validate as a regular Git URL
	return isValidGitURL(value)
}

// isValidPathOrParamRef checks if the provided value is a valid path or parameter reference
func isValidPathOrParamRef(value string) bool {
	// Check if it's a parameter reference
	if isParameterReference(value) {
		return true
	}

	// Otherwise validate as a regular path
	return isValidPath(value)
}

// isParameterReference checks if a value is a Tekton parameter reference
func isParameterReference(value string) bool {
	// Match patterns like $(params.param-name), $(context.pipelineRun.name), etc.
	paramRefPattern := `^\$\([^)]+\)$`
	matched, _ := regexp.MatchString(paramRefPattern, value)
	return matched
}

// isValidGitURL checks if the provided URL is a valid Git repository URL
func isValidGitURL(url string) bool {
	// Basic validation - should start with http/https/git or ssh
	if strings.HasPrefix(url, "http://") ||
		strings.HasPrefix(url, "https://") ||
		strings.HasPrefix(url, "git@") ||
		strings.HasPrefix(url, "ssh://") {
		return true
	}
	return false
}

// isValidPath checks if the provided path is valid
func isValidPath(path string) bool {
	// Basic validation - should not be empty and should not start with /
	if path == "" || strings.HasPrefix(path, "/") {
		return false
	}
	return true
}

// extractResultReferencesFromValue extracts result references from a parameter value string
func extractResultReferencesFromValue(value string) []*v1.ResultRef {
	var resultRefs []*v1.ResultRef

	// Pattern to match result references: $(tasks.taskname.results.resultname...)
	resultPattern := regexp.MustCompile(`\$\(tasks\.([^.]+)\.results\.([^).\[\s]+)`)

	matches := resultPattern.FindAllStringSubmatch(value, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			taskName := match[1]
			resultName := match[2]

			resultRefs = append(resultRefs, &v1.ResultRef{
				PipelineTask: taskName,
				Result:       resultName,
			})
		}
	}

	return resultRefs
}
