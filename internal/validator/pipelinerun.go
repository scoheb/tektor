package validator

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ValidatePipelineRun(ctx context.Context, pr v1.PipelineRun) error {
	return ValidatePipelineRunWithYAML(ctx, pr, nil)
}

func ValidatePipelineRunWithYAML(ctx context.Context, pr v1.PipelineRun, rawYAML []byte) error {
	var allErrors error

	// Validate parameter references in the raw YAML content if pipeline spec is embedded
	if rawYAML != nil && pr.Spec.PipelineSpec != nil {
		if err := ValidateParameterReferences(*pr.Spec.PipelineSpec, rawYAML); err != nil {
			allErrors = multierror.Append(allErrors, fmt.Errorf("parameter reference validation: %w", err))
		}
	}

	if err := pr.Validate(ctx); err != nil {
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

	if pipelineSpec := pr.Spec.PipelineSpec; pipelineSpec != nil {
		p := v1.Pipeline{
			// Some name value is required for validation.
			ObjectMeta: metav1.ObjectMeta{Name: "noname"},
			Spec:       *pipelineSpec,
		}
		if err := ValidatePipelineWithYAML(ctx, p, rawYAML); err != nil {
			allErrors = multierror.Append(allErrors, err)
		}
	}
	return allErrors
}
