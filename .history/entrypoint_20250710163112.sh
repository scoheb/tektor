#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# GitHub Action inputs
INPUT_FILES=${INPUT_FILES:-""}
INPUT_FILE_PATTERNS=${INPUT_FILE_PATTERNS:-"**/*.yaml,**/*.yml,**/*.json"}
INPUT_EXCLUDE_PATTERNS=${INPUT_EXCLUDE_PATTERNS:-".github/**,docs/**,README.md,**/README.md"}
INPUT_FAIL_ON_ERROR=${INPUT_FAIL_ON_ERROR:-"true"}
INPUT_VERBOSE=${INPUT_VERBOSE:-"false"}
INPUT_PARAMETERS=${INPUT_PARAMETERS:-""}
INPUT_DETECT_TEKTON_FILES=${INPUT_DETECT_TEKTON_FILES:-"true"}
INPUT_CHANGED_FILES_ONLY=${INPUT_CHANGED_FILES_ONLY:-"true"}

# Counters
ERROR_COUNT=0
WARNING_COUNT=0
VALIDATED_FILES=()

# Logging functions
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

log_debug() {
    if [[ "$INPUT_VERBOSE" == "true" ]]; then
        echo -e "${BLUE}[DEBUG]${NC} $1"
    fi
}

# Function to check if a file is a Tekton resource
is_tekton_resource() {
    local file="$1"
    
    # Check if file exists and is readable
    if [[ ! -r "$file" ]]; then
        log_debug "File $file is not readable"
        return 1
    fi
    
    # Check if file contains Tekton API version
    if grep -q "apiVersion.*tekton.dev" "$file" 2>/dev/null; then
        log_debug "File $file contains Tekton apiVersion"
        return 0
    fi
    
    return 1
}

# Function to get changed files in PR
get_changed_files() {
    local changed_files=()
    
    if [[ "$GITHUB_EVENT_NAME" == "pull_request" ]]; then
        log_info "Detecting changed files in pull request..."
        
        # Get the base and head SHA
        local base_sha="${GITHUB_BASE_REF:-main}"
        local head_sha="${GITHUB_SHA}"
        
        # Get changed files using git diff
        if git rev-parse --verify "origin/$base_sha" >/dev/null 2>&1; then
            log_debug "Using git diff to find changed files"
            while IFS= read -r -d '' file; do
                if [[ -f "$file" ]]; then
                    changed_files+=("$file")
                fi
            done < <(git diff --name-only --diff-filter=AM "origin/$base_sha"...HEAD -z 2>/dev/null || true)
        else
            log_warning "Could not find base branch origin/$base_sha, falling back to GitHub API"
            
            # Fallback: use GitHub API if available
            if [[ -n "$GITHUB_TOKEN" ]]; then
                local pr_number
                pr_number=$(jq -r '.number' "$GITHUB_EVENT_PATH" 2>/dev/null || echo "")
                
                if [[ -n "$pr_number" && "$pr_number" != "null" ]]; then
                    log_debug "Using GitHub API to get changed files for PR #$pr_number"
                    local api_url="https://api.github.com/repos/$GITHUB_REPOSITORY/pulls/$pr_number/files"
                    
                    while IFS= read -r file; do
                        if [[ -f "$file" && "$file" != "null" ]]; then
                            changed_files+=("$file")
                        fi
                    done < <(curl -s -H "Authorization: token $GITHUB_TOKEN" "$api_url" | jq -r '.[].filename' 2>/dev/null || true)
                fi
            fi
        fi
    else
        log_info "Not a pull request event, will scan all matching files"
    fi
    
    printf '%s\n' "${changed_files[@]}"
}

# Function to find files matching patterns
find_matching_files() {
    local files_to_check=()
    
    if [[ -n "$INPUT_FILES" ]]; then
        # Use explicitly provided files
        log_info "Using explicitly provided files: $INPUT_FILES"
        IFS=',' read -ra file_list <<< "$INPUT_FILES"
        for file in "${file_list[@]}"; do
            file=$(echo "$file" | xargs) # trim whitespace
            if [[ -f "$file" ]]; then
                files_to_check+=("$file")
            else
                log_warning "Provided file does not exist: $file"
            fi
        done
    elif [[ "$INPUT_CHANGED_FILES_ONLY" == "true" && "$GITHUB_EVENT_NAME" == "pull_request" ]]; then
        # Get changed files from PR
        log_info "Detecting changed files in pull request..."
        while IFS= read -r file; do
            if [[ -n "$file" ]]; then
                files_to_check+=("$file")
            fi
        done < <(get_changed_files)
    else
        # Find all files matching patterns
        log_info "Scanning all files matching patterns..."
        IFS=',' read -ra patterns <<< "$INPUT_FILE_PATTERNS"
        for pattern in "${patterns[@]}"; do
            pattern=$(echo "$pattern" | xargs) # trim whitespace
            while IFS= read -r -d '' file; do
                files_to_check+=("$file")
            done < <(find . -name "$pattern" -type f -print0 2>/dev/null || true)
        done
    fi
    
    # Filter out excluded patterns
    local filtered_files=()
    IFS=',' read -ra exclude_patterns <<< "$INPUT_EXCLUDE_PATTERNS"
    
    for file in "${files_to_check[@]}"; do
        local exclude_file=false
        
        for exclude_pattern in "${exclude_patterns[@]}"; do
            exclude_pattern=$(echo "$exclude_pattern" | xargs) # trim whitespace
            if [[ "$file" == $exclude_pattern ]]; then
                log_debug "Excluding file $file (matches pattern: $exclude_pattern)"
                exclude_file=true
                break
            fi
        done
        
        if [[ "$exclude_file" == "false" ]]; then
            filtered_files+=("$file")
        fi
    done
    
    printf '%s\n' "${filtered_files[@]}"
}

# Function to validate a single file with tektor
validate_file() {
    local file="$1"
    local temp_output
    temp_output=$(mktemp)
    
    log_info "Validating $file..."
    
    # Prepare tektor command
    local tektor_cmd="tektor validate"
    
    # Add verbose flag if enabled
    if [[ "$INPUT_VERBOSE" == "true" ]]; then
        tektor_cmd="$tektor_cmd --verbose"
    fi
    
    # Add parameters if provided
    if [[ -n "$INPUT_PARAMETERS" ]]; then
        while IFS= read -r param; do
            param=$(echo "$param" | xargs) # trim whitespace
            if [[ -n "$param" ]]; then
                tektor_cmd="$tektor_cmd --param \"$param\""
            fi
        done <<< "$INPUT_PARAMETERS"
    fi
    
    # Add the file to validate
    tektor_cmd="$tektor_cmd \"$file\""
    
    log_debug "Running: $tektor_cmd"
    
    # Run tektor validation
    if eval "$tektor_cmd" > "$temp_output" 2>&1; then
        log_success "✅ $file - Validation passed"
        VALIDATED_FILES+=("$file")
        
        # Show output if verbose
        if [[ "$INPUT_VERBOSE" == "true" ]]; then
            cat "$temp_output"
        fi
    else
        log_error "❌ $file - Validation failed"
        ERROR_COUNT=$((ERROR_COUNT + 1))
        
        # Always show error output
        echo "Error details:"
        cat "$temp_output"
        echo ""
    fi
    
    rm -f "$temp_output"
}

# Main execution
main() {
    log_info "🚀 Starting Tektor validation..."
    log_info "Repository: $GITHUB_REPOSITORY"
    log_info "Event: $GITHUB_EVENT_NAME"
    log_info "SHA: $GITHUB_SHA"
    
    # Check if tektor is available
    if ! command -v tektor &> /dev/null; then
        log_error "tektor command not found!"
        exit 1
    fi
    
    log_info "Tektor version: $(tektor --version 2>/dev/null || echo 'unknown')"
    
    # Find files to validate
    local files_to_validate=()
    while IFS= read -r file; do
        if [[ -n "$file" ]]; then
            files_to_validate+=("$file")
        fi
    done < <(find_matching_files)
    
    if [[ ${#files_to_validate[@]} -eq 0 ]]; then
        log_warning "No files found to validate"
        if [[ -n "$GITHUB_OUTPUT" ]]; then
            echo "validated-files=" >> "$GITHUB_OUTPUT"
            echo "validation-results=No files found" >> "$GITHUB_OUTPUT"
            echo "error-count=0" >> "$GITHUB_OUTPUT"
            echo "warning-count=0" >> "$GITHUB_OUTPUT"
        fi
        exit 0
    fi
    
    log_info "Found ${#files_to_validate[@]} files to check"
    
    # Filter for Tekton resources if auto-detection is enabled
    local tekton_files=()
    if [[ "$INPUT_DETECT_TEKTON_FILES" == "true" ]]; then
        log_info "Auto-detecting Tekton resource files..."
        for file in "${files_to_validate[@]}"; do
            if is_tekton_resource "$file"; then
                tekton_files+=("$file")
                log_debug "Detected Tekton resource: $file"
            else
                log_debug "Skipping non-Tekton file: $file"
            fi
        done
    else
        tekton_files=("${files_to_validate[@]}")
    fi
    
    if [[ ${#tekton_files[@]} -eq 0 ]]; then
        log_warning "No Tekton resource files found to validate"
        if [[ -n "$GITHUB_OUTPUT" ]]; then
            echo "validated-files=" >> "$GITHUB_OUTPUT"
            echo "validation-results=No Tekton resources found" >> "$GITHUB_OUTPUT"
            echo "error-count=0" >> "$GITHUB_OUTPUT"
            echo "warning-count=0" >> "$GITHUB_OUTPUT"
        fi
        exit 0
    fi
    
    log_info "Found ${#tekton_files[@]} Tekton resource files to validate:"
    for file in "${tekton_files[@]}"; do
        log_info "  - $file"
    done
    
    # Validate each file
    for file in "${tekton_files[@]}"; do
        validate_file "$file"
    done
    
    # Output results
    log_info ""
    log_info "📊 Validation Summary:"
    log_info "  Files validated: ${#VALIDATED_FILES[@]}"
    log_info "  Errors: $ERROR_COUNT"
    log_info "  Warnings: $WARNING_COUNT"
    
    # Set GitHub outputs
    echo "validated-files=$(IFS=,; echo "${VALIDATED_FILES[*]}")" >> "$GITHUB_OUTPUT"
    echo "error-count=$ERROR_COUNT" >> "$GITHUB_OUTPUT"
    echo "warning-count=$WARNING_COUNT" >> "$GITHUB_OUTPUT"
    
    if [[ $ERROR_COUNT -eq 0 ]]; then
        log_success "🎉 All validations passed!"
        echo "validation-results=All validations passed" >> "$GITHUB_OUTPUT"
    else
        log_error "💥 $ERROR_COUNT validation error(s) found"
        echo "validation-results=$ERROR_COUNT validation error(s) found" >> "$GITHUB_OUTPUT"
        
        if [[ "$INPUT_FAIL_ON_ERROR" == "true" ]]; then
            log_error "Failing action due to validation errors"
            exit 1
        else
            log_warning "Validation errors found but not failing action (fail-on-error=false)"
        fi
    fi
}

# Run main function
main "$@" 
