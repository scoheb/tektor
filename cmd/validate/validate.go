package validate

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/lcarva/tektor/internal/pac"
	"github.com/lcarva/tektor/internal/validator"
)

var (
	paramValues []string
	verbose     bool
)

var ValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a Tekton resource",
	Long: `Validate a Tekton resource including:
- Pipeline parameter validation
- Task parameter validation  
- Git resolver support for remote task references
- Bundle resolver support for OCI-based tasks
- Result reference validation
- Result type validation
- Workspace usage validation

You can provide runtime parameter values to substitute parameter references during validation.`,
	Example: `  # Validate a pipeline with embedded tasks
  tektor validate /tmp/pipeline.yaml

  # Validate a pipeline using git resolver
  tektor validate /tmp/pipeline-with-git-tasks.yaml
  
  # Validate a pipeline run
  tektor validate /tmp/pipelinerun.yaml
  
  # Validate with runtime parameters
  tektor validate /tmp/pipeline.yaml --param taskGitUrl=https://github.com/example/repo.git --param taskGitRevision=main`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		params, err := parseParamValues(paramValues)
		if err != nil {
			return fmt.Errorf("error parsing parameter values: %w", err)
		}
		return run(cmd.Context(), args[0], params)
	},
}

func init() {
	ValidateCmd.Flags().StringArrayVarP(&paramValues, "param", "p", []string{},
		"Parameter values in the format key=value (can be specified multiple times)")
	ValidateCmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"Enable verbose logging output")
}

// parseParamValues parses command-line parameter values in key=value format
func parseParamValues(paramStrs []string) (map[string]string, error) {
	params := make(map[string]string)
	for _, paramStr := range paramStrs {
		parts := strings.SplitN(paramStr, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid parameter format %q, expected key=value", paramStr)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, fmt.Errorf("empty parameter key in %q", paramStr)
		}
		params[key] = value
	}
	return params, nil
}

// substituteParameters replaces parameter references in YAML content with provided values
func substituteParameters(yamlContent []byte, params map[string]string) []byte {
	content := string(yamlContent)

	for key, value := range params {
		// Replace $(params.key) with the actual value
		paramRef := fmt.Sprintf("$(params.%s)", key)
		content = strings.ReplaceAll(content, paramRef, value)
	}

	return []byte(content)
}

func run(ctx context.Context, fname string, runtimeParams map[string]string) error {
	// Configure logging based on verbose flag
	if !verbose {
		log.SetOutput(os.Stderr)
		log.SetFlags(0) // Remove timestamp for cleaner output
	}

	log.Printf("Validating %s", fname)
	if len(runtimeParams) > 0 {
		logRuntimeParameters(runtimeParams)
	}

	f, err := os.ReadFile(fname)
	if err != nil {
		return fmt.Errorf("reading %s: %w", fname, err)
	}

	// Substitute runtime parameters if provided
	originalContent := f
	if len(runtimeParams) > 0 {
		f = substituteParameters(f, runtimeParams)
	}

	var o metav1.PartialObjectMetadata
	if err := yaml.Unmarshal(f, &o); err != nil {
		return fmt.Errorf("unmarshalling %s as k8s resource: %w", fname, err)
	}

	key := fmt.Sprintf("%s/%s", o.APIVersion, o.Kind)
	switch key {
	case "tekton.dev/v1/Pipeline":
		var p v1.Pipeline
		if err := yaml.Unmarshal(f, &p); err != nil {
			return fmt.Errorf("unmarshalling %s as %s: %w", fname, key, err)
		}
		if err := validator.ValidatePipelineWithYAMLAndParams(ctx, p, originalContent, runtimeParams); err != nil {
			return err
		}
	case "tekton.dev/v1/PipelineRun":
		f, err = pac.ResolvePipelineRun(ctx, fname, o.Name)
		if err != nil {
			return fmt.Errorf("resolving with PAC: %w", err)
		}

		// Apply parameter substitution to resolved content too
		if len(runtimeParams) > 0 {
			f = substituteParameters(f, runtimeParams)
		}

		var pr v1.PipelineRun
		if err := yaml.Unmarshal(f, &pr); err != nil {
			return fmt.Errorf("unmarshalling %s as %s: %w", fname, key, err)
		}

		if err := validator.ValidatePipelineRunWithYAML(ctx, pr, originalContent); err != nil {
			return err
		}
	case "tekton.dev/v1/Task":
		var t v1.Task
		if err := yaml.Unmarshal(f, &t); err != nil {
			return fmt.Errorf("unmarshaling %s as %s: %w", fname, key, err)
		}
		if err := validator.ValidateTaskV1(ctx, t); err != nil {
			return err
		}
	case "tekton.dev/v1beta1/Task":
		var t v1beta1.Task
		if err := yaml.Unmarshal(f, &t); err != nil {
			return fmt.Errorf("unmarshaling %s as %s: %w", fname, key, err)
		}
		if err := validator.ValidateTaskV1Beta1(ctx, t); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s is not supported", key)
	}

	log.Printf("✅ Validation successful for %s", fname)
	return nil
}

// logRuntimeParameters logs runtime parameters in a verbose and pretty format
func logRuntimeParameters(params map[string]string) {
	if len(params) == 1 {
		for key, value := range params {
			log.Printf("Using runtime parameter: %s=%s", key, value)
		}
		return
	}

	log.Printf("Using %d runtime parameters:", len(params))
	for key, value := range params {
		log.Printf("  • %s: %s", key, value)
	}
}
