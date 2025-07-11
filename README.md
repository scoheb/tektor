# Tektor

Why does this thing exist? Because I'm tired of finding out about problems with my Pipeline *after*
I run it.

It is written in go because that is the language used by the Tekton code base. It makes us not have
to re-invent the wheel to perform certain checks.

<!-- Trigger build workflow -->

It currently supports the following:

* Verify PipelineTasks pass all required parameters to Tasks.
* Verify PipelineTasks pass known parameters to Tasks.
* Verify PipelineTasks pass parameters of expected types to Tasks.
* Verify PipelineTasks use known Task results.
* Verify results are used according to their defined types.
* Verify workspace usage and requirements.
* Resolve remote/local Tasks via
  [PaC resolver](https://docs.openshift.com/pipelines/1.11/pac/using-pac-resolver.html),
  [git resolver](https://tekton.dev/docs/pipelines/git-resolver/).
  [Bundles resolver](https://tekton.dev/docs/pipelines/bundle-resolver/), and embedded Task
  definitions.
* Provide runtime parameters when invoking Tektor.
  * Helpful in cases where a parameter value is used as a field in a git resolver.

Future work:

* Verify PipelineRun parameters match parameters from Pipeline definition.

## GitHub Action

Tektor can be used as a GitHub Action to automatically validate Tekton resources in pull requests. The action will:

- 🔍 **Auto-detect** Tekton resource files (Pipeline, Task, PipelineRun)
- 📝 **Validate only changed files** in pull requests (configurable)
- ✅ **Comprehensive validation** using the full power of Tektor
- 📊 **Detailed reporting** with validation results and error counts
- 🎯 **Flexible configuration** with multiple input options
- 🚀 **Fast execution** using pre-built container image

### Container Image

The GitHub Action uses a pre-built container image hosted on Quay.io:
- **Registry**: `quay.io/scoheb/tektor-action`
- **Tags**: `latest` (main branch), version tags (e.g., `v1.0.0`)
- **Platforms**: linux/amd64, linux/arm64

You can also run the container directly:
```bash
docker run --rm -v $(pwd):/workspace quay.io/scoheb/tektor-action:latest tektor validate /workspace/your-file.yaml
```

### Quick Start

Create `.github/workflows/tekton-validation.yml`:

```yaml
name: Validate Tekton Resources

on:
  pull_request:
    paths:
      - '**/*.yaml'
      - '**/*.yml'

jobs:
  validate-tekton:
    runs-on: ubuntu-latest
    name: Validate Tekton Resources
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Get changed files
        uses: tj-actions/changed-files@ed68ef82c095e0d48ec87eccea555d944a631a4c  # v45.0.6
        id: changed-files
        with:
          files: |
            **/*.yaml
            **/*.yml

      - name: Validate Tekton Resources
        uses: ./  # Use this action
        with:
          changed-files: ${{ steps.changed-files.outputs.all_changed_files }}
          fail-on-error: true
          verbose: false
```

### Input Parameters

| Parameter | Description | Required | Default |
|-----------|-------------|----------|---------|
| `files` | Comma-separated list of files to validate. If not provided, changed-files must be provided | No | `""` |
| `file-patterns` | File patterns to match for Tekton resources (only used for exclusion filtering) | No | `**/*.yaml,**/*.yml,**/*.json` |
| `exclude-patterns` | File patterns to exclude from validation | No | `.github/**,docs/**,README.md,**/README.md` |
| `fail-on-error` | Whether to fail the action if validation errors are found | No | `true` |
| `verbose` | Enable verbose logging output | No | `false` |
| `parameters` | Runtime parameters for validation in key=value format, one per line | No | `""` |
| `detect-tekton-files` | Auto-detect Tekton resource files by content (apiVersion: tekton.dev) | No | `true` |
| `changed-files` | Newline-separated list of changed files to validate (required, use tj-actions/changed-files) | Yes | `""` |
| `tektor-args` | Additional arguments to pass to tektor command | No | `""` |

### Output Parameters

| Parameter | Description |
|-----------|-------------|
| `validated-files` | List of files that were validated |
| `validation-results` | Summary of validation results |
| `error-count` | Number of validation errors found |
| `warning-count` | Number of validation warnings found |

### Usage Examples

#### Basic Usage

```yaml
- name: Get changed files
  uses: tj-actions/changed-files@ed68ef82c095e0d48ec87eccea555d944a631a4c  # v45.0.6
  id: changed-files
  with:
    files: |
      **/*.yaml
      **/*.yml

- name: Validate Tekton Resources
  uses: your-org/tektor@v1
  with:
    changed-files: ${{ steps.changed-files.outputs.all_changed_files }}
```

#### Validate Specific Files

```yaml
- name: Validate Specific Files
  uses: your-org/tektor@v1
  with:
    files: 'pipelines/build.yaml,tasks/test.yaml'
    verbose: true
```

#### Validate All Files in Directory

```yaml
- name: Get all Tekton files
  uses: tj-actions/changed-files@ed68ef82c095e0d48ec87eccea555d944a631a4c  # v45.0.6
  id: all-files
  with:
    files: |
      tekton/**/*.yaml
      tekton/**/*.yml
    files_ignore: |
      tekton/experimental/**
    include_all_files: true

- name: Validate All Tekton Files
  uses: your-org/tektor@v1
  with:
    changed-files: ${{ steps.all-files.outputs.all_changed_files }}
```

#### With Runtime Parameters

```yaml
- name: Validate with Parameters
  uses: scoheb/tektor@v1
  with:
    parameters: |
      gitUrl=https://github.com/example/repo.git
      gitRevision=main
      imageTag=latest
```

#### With Pull Request Context

Pass PR information to tektor using the `parameters` input:

```yaml
- name: Validate Tekton Resources
  uses: scoheb/tektor@v1
  with:
    parameters: |
      gitUrl=https://github.com/${{ github.repository }}.git
      gitRevision=${{ github.head_ref || github.ref_name }}
      pr-url=${{ github.event.pull_request.html_url }}
      pr-branch=${{ github.event.pull_request.head.ref }}
      pr-base-branch=${{ github.event.pull_request.base.ref }}
```

Or use `tektor-args` for more control:

```yaml
- name: Validate with PR Context
  uses: scoheb/tektor@v1
  with:
    tektor-args: |
      --param pr-url="${{ github.event.pull_request.html_url }}"
      --param pr-branch="${{ github.event.pull_request.head.ref }}"
      --param pr-base-branch="${{ github.event.pull_request.base.ref }}"
```

#### With Custom Tektor Arguments

Pass additional arguments directly to the tektor command:

```yaml
- name: Validate with Custom Arguments
  uses: scoheb/tektor@v1
  with:
    tektor-args: '--param taskGitRevision=${{ github.event.pull_request.head.ref }} --param imageTag=latest'
```

```yaml
- name: Advanced Validation
  uses: scoheb/tektor@v1
  with:
    tektor-args: |
      --param taskGitRevision=${{ github.event.pull_request.head.ref || github.ref_name }}
      --param taskGitUrl=https://github.com/${{ github.repository }}.git
      --param buildId=${{ github.run_id }}
      --param prNumber=${{ github.event.pull_request.number }}
```

The `tektor-args` parameter gives you full control over tektor's command-line arguments and is especially useful for:
- Dynamic parameter values based on GitHub context
- Advanced tektor features not covered by other inputs
- Custom validation logic requiring specific tektor flags

**Note:** `tektor-args` is added to the command after other inputs, so you can combine it with `parameters`, `verbose`, etc.:

```yaml
- name: Combined Parameters
  uses: scoheb/tektor@v1
  with:
    verbose: true
    parameters: |
      gitUrl=https://github.com/${{ github.repository }}.git
    tektor-args: '--param taskGitRevision=${{ github.event.pull_request.head.ref }}'
```

This generates: `tektor validate --verbose --param "gitUrl=..." --param taskGitRevision=feature-branch file.yaml`

#### Custom File Patterns

```yaml
- name: Get custom pattern files
  uses: tj-actions/changed-files@ed68ef82c095e0d48ec87eccea555d944a631a4c  # v45.0.6
  id: custom-files
  with:
    files: |
      ci/tekton/**/*.yml
      pipelines/**/*.yaml
    files_ignore: |
      ci/tekton/experimental/**
      **/*-template.yaml

- name: Validate Custom Patterns
  uses: your-org/tektor@v1
  with:
    changed-files: ${{ steps.custom-files.outputs.all_changed_files }}
```

#### Don't Fail on Errors (Warning Mode)

```yaml
- name: Get changed files
  uses: tj-actions/changed-files@ed68ef82c095e0d48ec87eccea555d944a631a4c  # v45.0.6
  id: changed-files
  with:
    files: |
      **/*.yaml
      **/*.yml

- name: Validate (Warning Mode)
  uses: your-org/tektor@v1
  with:
    changed-files: ${{ steps.changed-files.outputs.all_changed_files }}
    fail-on-error: false
```

### Advanced Workflow Example

```yaml
name: Tekton Validation

on:
  pull_request:
    paths:
      - 'tekton/**/*.yaml'
      - 'pipelines/**/*.yml'
  push:
    branches: [main]

jobs:
  validate-tekton:
    runs-on: ubuntu-latest
    name: Validate Tekton Resources
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Get changed files
        uses: tj-actions/changed-files@ed68ef82c095e0d48ec87eccea555d944a631a4c  # v45.0.6
        id: changed-files
        with:
          files: |
            tekton/**/*.yaml
            pipelines/**/*.yml
          files_ignore: |
            tekton/experimental/**
            **/*-template.yaml

      - name: Validate Tekton Resources
        id: validate
        uses: your-org/tektor@v1
        with:
          changed-files: ${{ steps.changed-files.outputs.all_changed_files }}
          verbose: true
          parameters: |
            defaultGitUrl=https://github.com/${{ github.repository }}.git
            defaultGitRevision=${{ github.head_ref || github.ref_name }}

      - name: Comment on PR
        if: github.event_name == 'pull_request'
        uses: actions/github-script@v7
        with:
          script: |
            const { data: comments } = await github.rest.issues.listComments({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
            });
            
            const botComment = comments.find(comment => 
              comment.user.type === 'Bot' && comment.body.includes('Tektor Validation')
            );
            
            const validationResults = `${{ steps.validate.outputs.validation-results }}`;
            const errorCount = `${{ steps.validate.outputs.error-count }}`;
            const validatedFiles = `${{ steps.validate.outputs.validated-files }}`;
            
            const body = `## 🔍 Tektor Validation Results
            
            **Status:** ${errorCount === '0' ? '✅ Passed' : '❌ Failed'}
            **Files validated:** ${validatedFiles.split(',').length}
            **Errors:** ${errorCount}
            
            ${validatedFiles ? `
            ### Validated Files:
            ${validatedFiles.split(',').map(f => `- \`${f}\``).join('\n')}
            ` : ''}
            
            **Details:** ${validationResults}
            `;
            
            if (botComment) {
              await github.rest.issues.updateComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                comment_id: botComment.id,
                body: body
              });
            } else {
              await github.rest.issues.createComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                issue_number: context.issue.number,
                body: body
              });
            }
```

### File Detection Logic

The action processes files in the following order:

1. **Explicit file list**: When `files` input is provided, only those files are validated
2. **Changed files**: When `changed-files` input is provided (from tj-actions/changed-files), those files are validated
3. **Tekton resource detection**: Automatically identifies files containing `apiVersion: tekton.dev`
4. **Pattern filtering**: Uses exclude patterns to filter out unwanted files

**Note:** Either `files` or `changed-files` input must be provided. The action requires tj-actions/changed-files to detect which files to validate.

### Error Handling

- **Validation errors**: Tektor validation failures are reported with detailed error messages
- **File access errors**: Missing or unreadable files are logged as warnings
- **Git detection errors**: Fallback mechanisms for changed file detection
- **Action failures**: Configurable failure behavior with `fail-on-error` parameter

### Performance Considerations

- Only validates files provided by tj-actions/changed-files (efficient for PRs)
- Skips non-Tekton files automatically
- Parallel validation support (when multiple files)
- Leverages tj-actions/changed-files for efficient file detection

## CLI Usage

### Installation

```bash
go install github.com/lcarva/tektor@latest
```

### Basic Usage

```bash
# Validate a single file
tektor validate pipeline.yaml

# Validate with runtime parameters
tektor validate pipeline.yaml \
  --param gitUrl=https://github.com/example/repo.git \
  --param gitRevision=main

# Enable verbose output
tektor validate --verbose pipeline.yaml
```

### Examples

```bash
# Validate a pipeline with embedded tasks
tektor validate /tmp/pipeline.yaml

# Validate a pipeline using git resolver
tektor validate /tmp/pipeline-with-git-tasks.yaml

# Validate a pipeline run
tektor validate /tmp/pipelinerun.yaml

# Validate with runtime parameters
tektor validate /tmp/pipeline.yaml \
  --param taskGitUrl=https://github.com/example/repo.git \
  --param taskGitRevision=main
```

## Development

### Building

```bash
make build
```

### Testing

```bash
make test
```

### Container Build

```bash
# Fast build with golang base image
docker build -f Containerfile.golang -t tektor:latest .

# Fedora-based build
docker build -f Containerfile -t tektor:latest .
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.

