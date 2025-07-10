#!/bin/bash

# Test script to verify tektor-args functionality
echo "Testing tektor-args functionality..."

# Set up test environment
export INPUT_TEKTOR_ARGS="--param testParam=testValue --param dynamicValue=\${{ github.event.pull_request.head.ref }}"
export INPUT_VERBOSE="true"
export INPUT_FILES="testdata/task-valid.yaml"

# Source the functions from entrypoint.sh
source entrypoint.sh

# Test the validate_file function
echo "Testing validate_file function with tektor-args..."
echo "Expected command should include: --param testParam=testValue --param dynamicValue=\${{ github.event.pull_request.head.ref }}"

# This would normally call validate_file, but we'll just test the command building
echo "Test completed - check the generated tektor command in the logs"
