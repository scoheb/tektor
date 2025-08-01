package validate

import (
	"context"
	"fmt"
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
	runtimeParams []string
)

var ValidateCmd = &cobra.Command{
	Use:     "validate",
	Short:   "Validate a Tekton resource",
	Example: "tektor validate /tmp/pipeline.yaml --param taskGitUrl=https://github.com/example/repo",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		params, err := parseRuntimeParams(runtimeParams)
		if err != nil {
			return fmt.Errorf("parsing runtime parameters: %w", err)
		}
		return run(cmd.Context(), args[0], params)
	},
}

func init() {
	ValidateCmd.Flags().StringArrayVar(&runtimeParams, "param", []string{}, "Runtime parameters in format key=value (can be specified multiple times)")
}

func parseRuntimeParams(params []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, param := range params {
		parts := strings.SplitN(param, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid parameter format: %s (expected key=value)", param)
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}

func run(ctx context.Context, fname string, runtimeParams map[string]string) error {
	fmt.Printf("Validating %s\n", fname)
	f, err := os.ReadFile(fname)
	if err != nil {
		return fmt.Errorf("reading %s: %w", fname, err)
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
		if err := validator.ValidatePipeline(ctx, p, runtimeParams); err != nil {
			return err
		}
	case "tekton.dev/v1/PipelineRun":
		f, err = pac.ResolvePipelineRun(ctx, fname, o.Name)
		if err != nil {
			return fmt.Errorf("resolving with PAC: %w", err)
		}

		var pr v1.PipelineRun
		if err := yaml.Unmarshal(f, &pr); err != nil {
			return fmt.Errorf("unmarshalling %s as %s: %w", fname, key, err)
		}

		if err := validator.ValidatePipelineRun(ctx, pr, runtimeParams); err != nil {
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

	return nil
}
