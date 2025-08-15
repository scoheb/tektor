# Using Tektor GitHub Action in Other Repositories

This document explains how to use the Tektor GitHub Action in other repositories without publishing it to the GitHub Marketplace.

## Method 1: Direct Repository Reference ⭐ Recommended

Reference the action directly from your workflow:

```yaml
name: Validate Tekton Resources
on:
  pull_request:
    paths:
      - '**/*.yaml'
      - '**/*.yml'

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Get changed files
        id: changed-files
        uses: tj-actions/changed-files@v44
        with:
          files: |
            **/*.yaml
            **/*.yml

      - name: Checkout Tektor action
        uses: actions/checkout@v4
        with:
          repository: lcarva/tektor  # Replace with actual repo
          path: .github/actions/tektor
          ref: main  # Use specific tag for stability

      - name: Validate Tekton resources
        uses: ./.github/actions/tektor
        env:
          CHANGED_FILES: ${{ steps.changed-files.outputs.all_changed_files }}
        with:
          fail-on-error: true
          verbose: true
          params: 'taskGitUrl=https://github.com/tektoncd/catalog,gitRevision=main'
          pac-params: 'revision=main,branch=development'
```

## Method 2: Reusable Workflow ⭐ Best Practice

Use the reusable workflow (requires the workflow to be in a public repo or same organization):

```yaml
name: Validate Tekton Resources
on:
  pull_request:
    paths:
      - '**/*.yaml'
      - '**/*.yml'

jobs:
  validate-tekton:
    uses: lcarva/tektor/.github/workflows/validate-tekton-reusable.yml@main
    with:
      fail-on-error: true
      verbose: true
      file-patterns: '**/*.yaml,**/*.yml'
```

## Method 3: Git Submodules

Add tektor as a submodule:

```bash
# In your repository root
git submodule add https://github.com/lcarva/tektor.git .github/actions/tektor
git add .gitmodules .github/actions/tektor
git commit -m "Add tektor validation action"
```

Workflow usage:
```yaml
name: Validate Tekton Resources
on: [pull_request]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout with submodules
        uses: actions/checkout@v4
        with:
          submodules: true

      - name: Get changed files
        id: changed-files
        uses: tj-actions/changed-files@v44
        with:
          files: |
            **/*.yaml
            **/*.yml

      - name: Validate Tekton resources
        uses: ./.github/actions/tektor
        env:
          CHANGED_FILES: ${{ steps.changed-files.outputs.all_changed_files }}
        with:
          fail-on-error: true
          params: 'taskGitUrl=https://github.com/tektoncd/catalog,gitRevision=main'
```

## Method 4: Copy Action Files

Copy the action files to your repository:

```
your-repo/
├── .github/
│   ├── actions/
│   │   └── tektor/
│   │       ├── action.yml
│   │       └── validate.sh
│   └── workflows/
│       └── validate.yml
```

Then use it as a local action:
```yaml
- name: Validate Tekton resources
  uses: ./.github/actions/tektor
  env:
    CHANGED_FILES: ${{ steps.changed-files.outputs.all_changed_files }}
```

## Method 5: Fork and Reference

1. Fork the tektor repository
2. Reference your fork in workflows:

```yaml
- name: Checkout Tektor action
  uses: actions/checkout@v4
  with:
    repository: your-org/tektor  # Your fork
    path: .github/actions/tektor

- name: Validate Tekton resources
  uses: ./.github/actions/tektor
```

## Parameter Support

Tektor supports two types of parameters to customize validation behavior:

### Runtime Parameters (`params`)

Runtime parameters are used for Tekton parameter substitution within your pipeline and task definitions. These help resolve parameter references like `$(params.paramName)` during validation.

```yaml
- name: Validate Tekton resources
  uses: ./.github/actions/tektor
  env:
    CHANGED_FILES: ${{ steps.changed-files.outputs.all_changed_files }}
  with:
    params: 'taskGitUrl=https://github.com/tektoncd/catalog,gitRevision=main,imageTag=v1.0.0'
```

### PaC Template Parameters (`pac-params`)

PaC (Pipelines as Code) template parameters are used for PaC template substitution and preprocessing before validation.

```yaml
- name: Validate Tekton resources
  uses: ./.github/actions/tektor
  env:
    CHANGED_FILES: ${{ steps.changed-files.outputs.all_changed_files }}
  with:
    pac-params: 'revision=main,branch=development,environment=staging'
```

### Using Both Parameter Types

You can use both parameter types together for comprehensive validation:

```yaml
- name: Validate Tekton resources with parameters
  uses: ./.github/actions/tektor
  env:
    CHANGED_FILES: ${{ steps.changed-files.outputs.all_changed_files }}
  with:
    fail-on-error: true
    verbose: true
    params: 'taskGitUrl=https://github.com/tektoncd/catalog,gitRevision=main'
    pac-params: 'revision=main,branch=development'
```

### Dynamic Parameter Values

Parameters can be dynamically set using GitHub Actions expressions:

```yaml
- name: Validate Tekton resources with dynamic params
  uses: ./.github/actions/tektor
  env:
    CHANGED_FILES: ${{ steps.changed-files.outputs.all_changed_files }}
  with:
    params: 'gitRevision=${{ github.sha }},branch=${{ github.ref_name }}'
    pac-params: 'revision=${{ github.event.pull_request.head.sha || github.sha }}'
```

## Advanced Usage Examples

### With Multiple File Patterns

```yaml
- name: Get changed Tekton files
  id: changed-files
  uses: tj-actions/changed-files@v44
  with:
    files: |
      pipelines/**/*.yaml
      tasks/**/*.yml
      .tekton/**/*.yaml

- name: Validate if Tekton files changed
  if: steps.changed-files.outputs.any_changed == 'true'
  uses: ./.github/actions/tektor
  env:
    CHANGED_FILES: ${{ steps.changed-files.outputs.all_changed_files }}
```

### With Custom Error Handling

```yaml
- name: Validate Tekton resources
  id: validate
  continue-on-error: true
  uses: ./.github/actions/tektor
  env:
    CHANGED_FILES: ${{ steps.changed-files.outputs.all_changed_files }}
  with:
    fail-on-error: false
    verbose: true
    params: 'gitRevision=${{ github.sha }},repoUrl=${{ github.event.repository.clone_url }}'

- name: Comment on PR if validation failed
  if: steps.validate.outputs.validation-errors != '0'
  uses: actions/github-script@v7
  with:
    script: |
      github.rest.issues.createComment({
        issue_number: context.issue.number,
        owner: context.repo.owner,
        repo: context.repo.repo,
        body: `⚠️ Tekton validation found ${steps.validate.outputs.validation-errors} error(s) in ${steps.validate.outputs.validated-files} file(s).`
      })
```

### Matrix Strategy for Multiple Directories

```yaml
strategy:
  matrix:
    directory: [pipelines, tasks, triggers]

steps:
  - name: Get changed files in ${{ matrix.directory }}
    id: changed-files
    uses: tj-actions/changed-files@v44
    with:
      files: ${{ matrix.directory }}/**/*.{yaml,yml}

  - name: Validate ${{ matrix.directory }}
    if: steps.changed-files.outputs.any_changed == 'true'
    uses: ./.github/actions/tektor
    env:
      CHANGED_FILES: ${{ steps.changed-files.outputs.all_changed_files }}
```

## Recommendations

1. **Use Method 1 (Direct Repository Reference)** for most cases - it's simple and reliable
2. **Use Method 2 (Reusable Workflow)** for organizations with multiple repositories
3. **Pin to specific commits or tags** for production workflows: `ref: v1.0.0` instead of `ref: main`
4. **Use path filters** in workflow triggers to avoid unnecessary runs
5. **Set up branch protection rules** to require the validation check to pass

## Security Considerations

- When using external repositories, always pin to specific commits or tags
- Review the action code before using it in production
- For sensitive repositories, consider copying the action files (Method 4) to avoid external dependencies
