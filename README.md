# Tektor

Why does this thing exist? Because I'm tired of finding out about problems with my Pipeline *after*
I run it.

It is written in go because that is the language used by the Tekton code base. It makes us not have
to re-invent the wheel to perform certain checks.

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

