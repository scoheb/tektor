package pac

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/templates"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/substitution"
	"go.uber.org/zap"
	"sigs.k8s.io/yaml"
)

/*
This whole package is a huge hack. It is basically trying to implement the behavior when running:
	tkn pac resolve -f <input> --no-generate-name -o <output>
This inlines Task definitions from other files in the repo into the PipelineRun.
The implementation in github.com/openshift-pipelines/pipelines-as-code is not done in a share-friendly
manner. As such, the majority of the code here was copied and pasted from that repo.
*/

// setupResolveContext handles the common setup for both pipeline and pipeline run resolution
func setupResolveContext(ctx context.Context, fname string, pacParams map[string]string) (*params.Run, map[string]string, string, error) {
	run := params.New()
	errc := run.Clients.NewClients(ctx, &run.Info)
	zaplog, err := zap.NewProduction(
		zap.IncreaseLevel(zap.FatalLevel),
	)
	if err != nil {
		return nil, nil, "", err
	}
	run.Clients.Log = zaplog.Sugar()

	if errc != nil {
		// Allow resolve to be run without a kubeconfig
		noConfigErr := strings.Contains(errc.Error(), "Couldn't get kubeConfiguration namespace")
		if !noConfigErr {
			return nil, nil, "", errc
		}
	} else {
		// It's OK  if pac is not installed, ignore the error
		_ = run.UpdatePACInfo(ctx)
	}

	pacConfig := map[string]string{}
	if err := settings.ConfigToSettings(run.Clients.Log, run.Info.Pac.Settings, pacConfig); err != nil {
		return nil, nil, "", err
	}

	// Start with git-derived parameters
	params := map[string]string{}

	gitinfo := git.GetGitInfo(path.Dir(fname))
	if gitinfo.SHA != "" {
		params["revision"] = gitinfo.SHA
	}
	if gitinfo.URL != "" {
		params["repo_url"] = gitinfo.URL
		repoOwner, err := formatting.GetRepoOwnerFromURL(gitinfo.URL)
		if err != nil {
			return nil, nil, "", fmt.Errorf("getting git repo owner: %w", err)
		}
		params["repo_owner"] = strings.Split(repoOwner, "/")[0]
		params["repo_name"] = strings.Split(repoOwner, "/")[1]
	}

	// Add runtime parameters, which will override git-derived ones if there are conflicts
	for key, value := range pacParams {
		params[key] = value
	}

	pacDir := path.Join(gitinfo.TopLevelPath, ".tekton")

	// Must change working dir to git repo so local fs resolver works
	if gitinfo.TopLevelPath != "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, nil, "", fmt.Errorf("getting current working directory: %w", err)
		}
		if err := os.Chdir(gitinfo.TopLevelPath); err != nil {
			return nil, nil, "", fmt.Errorf("changing working directory: %w", err)
		}
		defer func(wd string) {
			_ = os.Chdir(wd)
		}(wd)
	}

	return run, params, pacDir, nil
}

func ResolvePipelineRun(ctx context.Context, fname string, prName string, pacParams map[string]string) ([]byte, error) {
	run, params, pacDir, err := setupResolveContext(ctx, fname, pacParams)
	if err != nil {
		return nil, err
	}

	allTemplates := templates.ReplacePlaceHoldersVariables(enumerateFiles([]string{pacDir}), params)

	// We use github here but since we don't do remotetask we would not care
	providerintf := github.New()
	event := info.NewEvent()

	ropt := &resolve.Opts{RemoteTasks: true}
	prs, err := resolve.Resolve(ctx, run, run.Clients.Log, providerintf, event, allTemplates, ropt)
	if err != nil {
		return nil, err
	}
	var pr *v1.PipelineRun
	for _, somePR := range prs {
		if somePR.Name == prName {
			pr = somePR
			break
		}
	}
	if pr == nil {
		return nil, fmt.Errorf("unable to find %q pipelinerun after pac resolution", prName)
	}

	// Apply additional parameter substitutions to the PipelineRun structure
	applyParameterSubstitutionsToPipelineRun(pr, params)

	pr.APIVersion = v1.SchemeGroupVersion.String()
	pr.Kind = "PipelineRun"
	d, err := yaml.Marshal(pr)
	if err != nil {
		return nil, fmt.Errorf("marshaling pac resolved pipelinerun: %w", err)
	}

	return cleanRe.ReplaceAll(d, []byte("\n")), nil
}

func ResolvePipeline(ctx context.Context, fname string, pipelineName string, pacParams map[string]string) ([]byte, error) {

	_, params, pacDir, err := setupResolveContext(ctx, fname, pacParams)
	if err != nil {
		return nil, err
	}

	// Include both the .tekton directory and the actual pipeline file being validated
	var templatePaths []string
	if pacDir != "" {
		templatePaths = append(templatePaths, pacDir)
	}
	templatePaths = append(templatePaths, fname)

	allTemplates := templates.ReplacePlaceHoldersVariables(enumerateFiles(templatePaths), params)

	// Parse the templates to find Pipeline objects
	var pipeline *v1.Pipeline
	documents := strings.Split(allTemplates, "---")
	for _, doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		// Try to unmarshal as Pipeline
		var p v1.Pipeline
		if err := yaml.Unmarshal([]byte(doc), &p); err == nil {
			if p.Kind == "Pipeline" && p.Name == pipelineName {
				pipeline = &p
				break
			}
		}
	}

	if pipeline == nil {
		return nil, fmt.Errorf("unable to find %q pipeline in templates", pipelineName)
	}

	// Apply additional parameter substitutions to the pipeline structure
	// This handles cases where PaC template substitution didn't catch all parameter references
	applyParameterSubstitutionsToPipeline(pipeline, params)

	pipeline.APIVersion = v1.SchemeGroupVersion.String()
	pipeline.Kind = "Pipeline"
	d, err := yaml.Marshal(pipeline)
	if err != nil {
		return nil, fmt.Errorf("marshaling pac resolved pipeline: %w", err)
	}

	return cleanRe.ReplaceAll(d, []byte("\n")), nil
}

// applyParameterSubstitutionsToPipeline applies parameter substitutions to all string fields in the pipeline
func applyParameterSubstitutionsToPipeline(pipeline *v1.Pipeline, params map[string]string) {
	// Convert params to the format expected by ApplyReplacements
	replacements := make(map[string]string)
	for key, value := range params {
		replacements["params."+key] = value
	}

	// Apply substitutions to pipeline tasks
	for i := range pipeline.Spec.Tasks {
		task := &pipeline.Spec.Tasks[i]

		// Apply substitutions to task parameters
		for j := range task.Params {
			param := &task.Params[j]
			if param.Value.StringVal != "" {
				param.Value.StringVal = substitution.ApplyReplacements(param.Value.StringVal, replacements)
			}
		}

		// Apply substitutions to TaskRef parameters
		if task.TaskRef != nil && task.TaskRef.Params != nil {
			for j := range task.TaskRef.Params {
				param := &task.TaskRef.Params[j]
				if param.Value.StringVal != "" {
					param.Value.StringVal = substitution.ApplyReplacements(param.Value.StringVal, replacements)
				}
			}
		}
	}

	// Apply substitutions to finally tasks
	for i := range pipeline.Spec.Finally {
		task := &pipeline.Spec.Finally[i]

		// Apply substitutions to task parameters
		for j := range task.Params {
			param := &task.Params[j]
			if param.Value.StringVal != "" {
				param.Value.StringVal = substitution.ApplyReplacements(param.Value.StringVal, replacements)
			}
		}

		// Apply substitutions to TaskRef parameters
		if task.TaskRef != nil && task.TaskRef.Params != nil {
			for j := range task.TaskRef.Params {
				param := &task.TaskRef.Params[j]
				if param.Value.StringVal != "" {
					param.Value.StringVal = substitution.ApplyReplacements(param.Value.StringVal, replacements)
				}
			}
		}
	}
}

// applyParameterSubstitutionsToPipelineRun applies parameter substitutions to all string fields in the PipelineRun
func applyParameterSubstitutionsToPipelineRun(pr *v1.PipelineRun, params map[string]string) {
	// Convert params to the format expected by ApplyReplacements
	replacements := make(map[string]string)
	for key, value := range params {
		replacements["params."+key] = value
	}

	// Apply substitutions to PipelineRun parameters
	for i := range pr.Spec.Params {
		param := &pr.Spec.Params[i]
		if param.Value.StringVal != "" {
			param.Value.StringVal = substitution.ApplyReplacements(param.Value.StringVal, replacements)
		}
	}

	// Apply substitutions to pipeline tasks
	for i := range pr.Spec.PipelineSpec.Tasks {
		task := &pr.Spec.PipelineSpec.Tasks[i]

		// Apply substitutions to task parameters
		for j := range task.Params {
			param := &task.Params[j]
			if param.Value.StringVal != "" {
				param.Value.StringVal = substitution.ApplyReplacements(param.Value.StringVal, replacements)
			}
		}

		// Apply substitutions to TaskRef parameters
		if task.TaskRef != nil && task.TaskRef.Params != nil {
			for j := range task.TaskRef.Params {
				param := &task.TaskRef.Params[j]
				if param.Value.StringVal != "" {
					param.Value.StringVal = substitution.ApplyReplacements(param.Value.StringVal, replacements)
				}
			}
		}
	}

	// Apply substitutions to finally tasks
	for i := range pr.Spec.PipelineSpec.Finally {
		task := &pr.Spec.PipelineSpec.Finally[i]

		// Apply substitutions to task parameters
		for j := range task.Params {
			param := &task.Params[j]
			if param.Value.StringVal != "" {
				param.Value.StringVal = substitution.ApplyReplacements(param.Value.StringVal, replacements)
			}
		}

		// Apply substitutions to TaskRef parameters
		if task.TaskRef != nil && task.TaskRef.Params != nil {
			for j := range task.TaskRef.Params {
				param := &task.TaskRef.Params[j]
				if param.Value.StringVal != "" {
					param.Value.StringVal = substitution.ApplyReplacements(param.Value.StringVal, replacements)
				}
			}
		}
	}
}

// cleanedup regexp do as much as we can but really it's a lost game to try this
var cleanRe = regexp.MustCompile(`\n(\t|\s)*(creationTimestamp|spec|taskRunTemplate|metadata|computeResources):\s*(null|{})\n`)

func enumerateFiles(filenames []string) string {
	var yamlDoc string
	for _, paths := range filenames {
		if stat, err := os.Stat(paths); err == nil && !stat.IsDir() {
			yamlDoc += appendYaml(paths)
			continue
		}

		// walk dir getting all yamls
		err := filepath.Walk(paths, func(path string, fi os.FileInfo, err error) error {
			if filepath.Ext(path) == ".yaml" {
				yamlDoc += appendYaml(path)
			}
			return nil
		})
		if err != nil {
			log.Fatalf("Error enumerating files: %v", err)
		}
	}

	return yamlDoc
}

func appendYaml(filename string) string {
	b, err := os.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}
	s := string(b)
	if strings.HasPrefix(s, "---") {
		return s
	}
	return fmt.Sprintf("---\n%s", s)
}
