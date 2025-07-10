package validator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/go-multierror"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// resultUsageContext represents the context where a result is being used
type resultUsageContext struct {
	Location     string // Description of where the result is used
	ExpectedType string // Expected type based on usage context
	ActualUsage  string // The actual usage string for context
}

func ValidateResults(resultRefs []*v1.ResultRef, allTaskResults map[string][]v1.TaskResult) error {
	return ValidateResultsWithContext(resultRefs, allTaskResults, make(map[string]resultUsageContext))
}

func ValidateResultsWithContext(resultRefs []*v1.ResultRef, allTaskResults map[string][]v1.TaskResult, usageContexts map[string]resultUsageContext) error {
	var err error

	for _, resultRef := range resultRefs {
		results, found := allTaskResults[resultRef.PipelineTask]
		if !found {
			err = multierror.Append(err, fmt.Errorf("%s result from non-existent %s PipelineTask", resultRef.Result, resultRef.PipelineTask))
			continue
		}
		var result *v1.TaskResult
		for _, r := range results {
			if r.Name == resultRef.Result {
				result = &r
				break
			}
		}
		if result == nil {
			err = multierror.Append(err, fmt.Errorf("non-existent %s result from %s PipelineTask", resultRef.Result, resultRef.PipelineTask))
			continue
		}

		// Validate result type usage
		if typeErr := validateResultTypeUsage(resultRef, result, usageContexts); typeErr != nil {
			err = multierror.Append(err, typeErr)
		}
	}
	return err
}

// validateResultTypeUsage validates that a result is used according to its defined type
func validateResultTypeUsage(resultRef *v1.ResultRef, result *v1.TaskResult, usageContexts map[string]resultUsageContext) error {
	// Get the defined type, default to "string" if not specified
	definedType := string(result.Type)
	if definedType == "" {
		definedType = "string"
	}

	// Create a unique key for this result reference
	refKey := fmt.Sprintf("%s.%s", resultRef.PipelineTask, resultRef.Result)

	// Check if we have usage context information
	if context, hasContext := usageContexts[refKey]; hasContext {
		// Validate type compatibility based on context
		if !isResultTypeCompatible(definedType, context.ExpectedType, context.ActualUsage) {
			return fmt.Errorf("result type mismatch: %s result from %s PipelineTask is defined as type %q but used as type %q in %s (usage: %s)",
				resultRef.Result, resultRef.PipelineTask, definedType, context.ExpectedType, context.Location, context.ActualUsage)
		}
	}

	return nil
}

// isResultTypeCompatible checks if the defined result type is compatible with the expected usage
func isResultTypeCompatible(definedType, expectedType, actualUsage string) bool {
	// If no expected type specified, assume compatibility
	if expectedType == "" {
		return true
	}

	// Exact match is always compatible
	if definedType == expectedType {
		return true
	}

	// Check for specific compatibility rules
	switch definedType {
	case "string":
		// String results can only be used as strings
		return expectedType == "string"
	case "array":
		// Array results can be used as:
		// - arrays (when passing the whole array)
		// - strings (when indexing like array[0])
		if expectedType == "array" {
			return true
		}
		if expectedType == "string" && isArrayIndexUsage(actualUsage) {
			return true
		}
		return false
	case "object":
		// Object results can be used as:
		// - objects (when passing the whole object)
		// - strings (when accessing properties like object.property)
		if expectedType == "object" {
			return true
		}
		if expectedType == "string" && isObjectPropertyUsage(actualUsage) {
			return true
		}
		return false
	default:
		// Unknown types default to string behavior
		return expectedType == "string"
	}
}

// isArrayIndexUsage checks if the usage appears to be array indexing (e.g., $(tasks.task.results.array[0]))
func isArrayIndexUsage(usage string) bool {
	// Match patterns like $(tasks.task.results.name[0]) or $(tasks.task.results.name[*])
	arrayIndexPattern := regexp.MustCompile(`\[(\d+|\*)\]`)
	return arrayIndexPattern.MatchString(usage)
}

// isObjectPropertyUsage checks if the usage appears to be object property access (e.g., $(tasks.task.results.obj.property))
func isObjectPropertyUsage(usage string) bool {
	// Match patterns like $(tasks.task.results.name.property)
	// Count the dots after "results." to see if accessing a property
	resultsIndex := strings.Index(usage, "results.")
	if resultsIndex == -1 {
		return false
	}

	// Get the part after "results."
	afterResults := usage[resultsIndex+len("results."):]

	// Remove any closing parentheses and whitespace
	afterResults = strings.TrimRight(afterResults, " )")

	// Count dots in the result name part - if more than 0, it's likely property access
	dotCount := strings.Count(afterResults, ".")
	return dotCount > 0
}

// ValidateResultsWithRawYAML validates results with additional context from raw YAML
func ValidateResultsWithRawYAML(resultRefs []*v1.ResultRef, allTaskResults map[string][]v1.TaskResult, rawYAML []byte, location string) error {
	if rawYAML == nil {
		return ValidateResults(resultRefs, allTaskResults)
	}

	// Extract usage contexts from raw YAML
	usageContexts := extractResultUsageContexts(rawYAML, location)

	return ValidateResultsWithContext(resultRefs, allTaskResults, usageContexts)
}

// extractResultUsageContexts extracts result usage contexts from raw YAML
func extractResultUsageContexts(rawYAML []byte, location string) map[string]resultUsageContext {
	contexts := make(map[string]resultUsageContext)
	yamlContent := string(rawYAML)

	// Pattern to match result references: $(tasks.taskname.results.resultname...)
	// This pattern captures: tasks.taskname.results.resultname and any suffix (like [0] or .property)
	resultPattern := regexp.MustCompile(`\$\(tasks\.([^.]+)\.results\.([^).\[\s]+)([^)]*)\)`)

	matches := resultPattern.FindAllStringSubmatch(yamlContent, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			taskName := match[1]
			resultName := match[2]
			fullUsage := match[0]
			suffix := ""
			if len(match) > 3 {
				suffix = match[3]
			}

			refKey := fmt.Sprintf("%s.%s", taskName, resultName)

			// Determine expected type based on usage pattern
			expectedType := determineExpectedTypeFromUsage(fullUsage, suffix)

			contexts[refKey] = resultUsageContext{
				Location:     location,
				ExpectedType: expectedType,
				ActualUsage:  fullUsage,
			}
		}
	}

	return contexts
}

// determineExpectedTypeFromUsage determines the expected type based on how the result is used
func determineExpectedTypeFromUsage(fullUsage, suffix string) string {
	// Check for array indexing patterns like [0], [1], [*]
	// When indexing an array, the result is a string (the indexed element)
	if strings.Contains(fullUsage, "[") && strings.Contains(fullUsage, "]") {
		return "string" // Array indexing returns string elements
	}

	// Check for object property access patterns like .property
	// When accessing object properties, the result is a string
	if strings.Contains(suffix, ".") {
		return "string" // Object property access returns string values
	}

	// Default to string for simple usage
	return "string"
}
