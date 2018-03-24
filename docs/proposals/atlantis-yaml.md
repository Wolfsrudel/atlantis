We want to be able to control:
- what each step of a workflow does
    - use `steps` key and `workflows`
- the projects in this repo and their config

```yaml
version: 2
projects:
- dir: "."
  workspace: "staging" # not using an array to enable different workflows per workspace
  workflow: default
  plan:
    when_modified: ["*/**", "../modules/**"]
  apply:
    requirements:
    - mergeable
    auto_merge: true
  terraform_version: 0.11.3
  locking: false

# we always expect plan and apply workflows/steps to be defined
# these are "stages" in a larger terraform execution workflow.
# depending on the project, there may be custom steps that need to be defined
# since some people have multiple projects in a single repo, we want to be able
# to support multiple different configurations for each stage
# in the future, we may also want to support different
workflows:
  default:
    plan:
      steps:
      - init
      - plan
    apply:
      steps:
      - apply
  custom:
    apply:
    plan:
 ```

Hootsuite
```
version: 2
projects:
- dir: "."
  workspace: "staging"
  workflows: default # this would be the default
  auto_plan:
    when_modified: ["**/*.tf"] # this would be the default
    enabled: true # defaults to true
  apply_requirements:
  - mergeable # only one available right now
  terraform_version: 0.11.3
  locking: true # not implemented yet
- dir: "."
  workspace: "production"
  workflows: default
  auto_plan:
    when_modified: ["**/*.tf"]
  apply_requirements:
  - mergeable
  terraform_version: 0.11.3

workflows:
  default:
    # init always runs before plan. It only runs before apply if we've lost our workspace on disk
    init:
      steps:
      - init
    plan:
      steps:
      - init
      - plan:
          extra_args: ["-var-file=env/$WORKSPACE.tfvars"]
    apply:
      steps:
      - apply
```

## Server Side
Requirements
- apply same config for multiple repos
- handle repo-specific config
- whitelist specific repos?

```
repositories:
- id: github.com/runatlantis/atlantis
  projects:
  -   dir: "."
	  workspace: "staging"
	  workflows: default # this would be the default
	  plan:
	    when_modified: ["**/*.tf"] # this would be the default
	  apply:
	    requirements:
	    - mergeable
	  terraform_version: 0.11.3
	  locking: true
- id: /github.com\/runatlantis\/.*/ # if starts with / then is regular expression
  projects:
  -   dir: "."
	  workspace: "staging"
	  workflows: default # this would be the default
	  plan:
	    when_modified: ["**/*.tf"] # this would be the default
	  apply:
	    requirements:
	    - mergeable
	  terraform_version: 0.11.3
	  locking: true
```

## Implementation Plan:
- new format that has steps, projects and additional arguments. Will transform atlantis.yaml v1 to v2
- then autoplanning
- then apply requirements
- then server-side

## Spec

## To Do's
- can refactor terraform runner to be able to set it up with all the info it needs
  like the logger and workspace path, etc. and then should just be able to call it with Run("plan", "....")
- executors should become the "stage" executors that do setup before the steps
- rename AtlantisWorkspace and GetWorkspace to not use the words workspace since it's confusing
- if they're overriding the plan/apply steps they need to indicate which step's output we need to put on the pull request
- use extra_args or append_args?
- set defaults with UnmarshalYAML but don't try to do validation. Makes for bad error messages for the user.
- should we rename workflows to workflows?

### Notes
- some steps have setup that needs to be done outside the actual step execution.
  - for example the apply step needs to check if apply is required, etc.
  - now is it the step or the stage that needs to have stuff done outside of it?
  - if we can think of it as the latter then it will be easier since if someone subs
    out the steps for the apply stage we'll still want to do all the same setup

### Phase 1 - 0.4.0-alpha
- default to current dir when running `atlantis plan` comment. - DONE!
- all "steps":
  - init/get - DONE!
  - plan - DONE!
  - apply - DONE!
- parse new config file format and construct operation graph
- extra_arguments for steps - DONE!
- if a legacy atlantis.yaml is found, print out what it should look like in the new format
- use apply.requirements instead of --require-approval
- custom steps

### Phase 2
- on_push stuff
- always use "." for directory when running via comment

### Phase 3
- server-side
