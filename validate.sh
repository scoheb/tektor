#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Initialize counters
VALIDATED_COUNT=0
ERROR_COUNT=0
CHECKED_COUNT=0

# Function to log messages
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if a file is a supported Tekton resource
is_tekton_resource() {
    local file="$1"

    # Skip if file doesn't exist
    if [[ ! -f "$file" ]]; then
        return 1
    fi

    # Check if it's a YAML file by extension
    if [[ ! "$file" =~ \.(yaml|yml)$ ]]; then
        return 1
    fi

    # Check if the file contains tekton.dev apiVersion
    if ! grep -q "apiVersion:.*tekton\.dev" "$file" 2>/dev/null; then
        return 1
    fi

    # Extract apiVersion and kind to verify it's a supported resource
    local api_version=$(grep "apiVersion:" "$file" | head -1 | sed 's/.*apiVersion:[[:space:]]*\(.*\)/\1/' | tr -d '"')
    local kind=$(grep "kind:" "$file" | head -1 | sed 's/.*kind:[[:space:]]*\(.*\)/\1/' | tr -d '"')

    # Check if it's a supported resource type
    local resource_key="${api_version}/${kind}"
    case "$resource_key" in
        "tekton.dev/v1/Pipeline"|"tekton.dev/v1/PipelineRun"|"tekton.dev/v1/Task"|"tekton.dev/v1beta1/Task")
            return 0
            ;;
        *)
            if [[ "$VERBOSE" == "true" ]]; then
                log_warning "Skipping unsupported resource: $resource_key in $file"
            fi
            return 1
            ;;
    esac
}

# Function to build parameter arguments for tektor
build_param_args() {
    local param_args=()

    # Process TEKTOR_PARAMS (runtime parameters)
    if [[ -n "$TEKTOR_PARAMS" ]]; then
        IFS=',' read -ra params <<< "$TEKTOR_PARAMS"
        for param in "${params[@]}"; do
            # Trim whitespace
            param=$(echo "$param" | xargs)
            if [[ -n "$param" ]]; then
                param_args+=("--param" "$param")
            fi
        done
    fi

    # Process TEKTOR_PAC_PARAMS (PaC template parameters)
    if [[ -n "$TEKTOR_PAC_PARAMS" ]]; then
        IFS=',' read -ra pac_params <<< "$TEKTOR_PAC_PARAMS"
        for param in "${pac_params[@]}"; do
            # Trim whitespace
            param=$(echo "$param" | xargs)
            if [[ -n "$param" ]]; then
                param_args+=("--pac-param" "$param")
            fi
        done
    fi

    echo "${param_args[@]}"
}

# Function to validate a single file
validate_file() {
    local file="$1"

    log_info "Validating: $file"

    # Build parameter arguments
    local param_args
    param_args=$(build_param_args)

    if [[ "$VERBOSE" == "true" && -n "$param_args" ]]; then
        log_info "Using parameters: $param_args"
    fi

    # Execute tektor with parameters
    if [[ -n "$param_args" ]]; then
        # Use eval to properly handle the parameter array
        if eval "$TEKTOR_BINARY validate $param_args \"$file\""; then
            log_success "✓ $file"
            VALIDATED_COUNT=$((VALIDATED_COUNT + 1))
            return 0
        else
            log_error "✗ $file"
            ERROR_COUNT=$((ERROR_COUNT + 1))
            return 1
        fi
    else
        # No parameters, use original command
        if "$TEKTOR_BINARY" validate "$file"; then
            log_success "✓ $file"
            VALIDATED_COUNT=$((VALIDATED_COUNT + 1))
            return 0
        else
            log_error "✗ $file"
            ERROR_COUNT=$((ERROR_COUNT + 1))
            return 1
        fi
    fi
}

# Main validation logic
main() {
    log_info "Starting Tekton resource validation..."

    # Check if CHANGED_FILES is set
    if [[ -z "$CHANGED_FILES" ]]; then
        log_error "CHANGED_FILES environment variable is not set"
        exit 1
    fi

    # Check if tektor binary exists
    TEKTOR_BINARY="${GITHUB_ACTION_PATH:-./}/build/tektor"
    if [[ ! -f "$TEKTOR_BINARY" ]]; then
        log_error "tektor binary not found at $TEKTOR_BINARY"
        exit 1
    fi

    # Convert space-separated CHANGED_FILES to array
    read -ra files <<< "$CHANGED_FILES"

    log_info "Found ${#files[@]} changed files to examine"

    if [[ "$VERBOSE" == "true" ]]; then
        log_info "Changed files: ${files[*]}"
    fi

    # Filter and validate Tekton resources
    local validation_failed=false
    for file in "${files[@]}"; do
        CHECKED_COUNT=$((CHECKED_COUNT + 1))

        if [[ "$VERBOSE" == "true" ]]; then
            log_info "Checking file: $file"
        fi

        if is_tekton_resource "$file"; then
            if [[ "$VERBOSE" == "true" ]]; then
                log_info "File $file is a Tekton resource, validating..."
            fi

            if ! validate_file "$file"; then
                validation_failed=true
            fi
        else
            if [[ "$VERBOSE" == "true" ]]; then
                log_info "Skipping non-Tekton file: $file"
            fi
        fi
    done

    # Output summary
    echo ""
    log_info "=== Validation Summary ==="
    log_info "Files checked: $CHECKED_COUNT"
    log_info "Tekton resources validated: $VALIDATED_COUNT"

    if [[ $ERROR_COUNT -eq 0 ]]; then
        log_success "All validations passed! ✓"
    else
        log_error "Found $ERROR_COUNT validation error(s) ✗"
    fi

    # Set outputs for GitHub Actions
    if [[ -n "$GITHUB_OUTPUT" ]]; then
        echo "validated-files=$VALIDATED_COUNT" >> "$GITHUB_OUTPUT"
        echo "validation-errors=$ERROR_COUNT" >> "$GITHUB_OUTPUT"
    fi

    # Exit with error if there were validation failures and fail-on-error is true
    if [[ "$validation_failed" == "true" && "$FAIL_ON_ERROR" == "true" ]]; then
        log_error "Validation failed. Set fail-on-error to false to continue on validation errors."
        exit 1
    fi

    log_success "Validation completed successfully"
}

# Run main function
main "$@"
