package repoconfig

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/logging"
	"gopkg.in/yaml.v2"
)

const AtlantisYAMLFilename = "atlantis.yaml"
const PlanStageName = "plan"
const ApplyStageName = "apply"

type Reader struct {
	TerraformExecutor TerraformExec
	DefaultTFVersion  *version.Version
}

// ReadConfig returns the parsed and validated config for repoDir.
// If there was no config, it returns a nil pointer. If there was an error
// in parsing it returns the error.
func (r *Reader) ReadConfig(repoDir string) (*RepoConfig, error) {
	configFile := filepath.Join(repoDir, AtlantisYAMLFilename)
	configData, err := ioutil.ReadFile(configFile)

	// If the file doesn't exist return nil.
	if err != nil && os.IsNotExist(err) {
		return nil, nil
	}

	// If it exists but we couldn't read it return an error.
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read %s file", AtlantisYAMLFilename)
	}

	// If the config file exists, parse it.
	config, err := r.ParseAndValidate(configData)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing %s", AtlantisYAMLFilename)
	}
	return &config, err
}

func (r *Reader) BuildPlanStage(log *logging.SimpleLogger, repoDir string, workspace string, relProjectPath string, extraCommentArgs []string, username string) (*PlanStage, error) {
	defaults := r.defaultPlanSteps(log, repoDir, workspace, relProjectPath, extraCommentArgs, username)
	steps, err := r.BuildStage(PlanStageName, log, repoDir, workspace, relProjectPath, extraCommentArgs, username, defaults)
	if err != nil {
		return nil, err
	}
	return &PlanStage{
		Steps: steps,
	}, nil
}

func (r *Reader) BuildStage(stageName string, log *logging.SimpleLogger, repoDir string, workspace string, relProjectPath string, extraCommentArgs []string, username string, defaults []Step) ([]Step, error) {
	config, err := r.ReadConfig(repoDir)
	if err != nil {
		return nil, err
	}

	// If there's no config file, use defaults.
	if config == nil {
		log.Info("no %s file found––continuing with defaults", AtlantisYAMLFilename)
		return defaults, nil
	}

	// Get this project's configuration.
	for _, p := range config.Projects {
		if p.Dir == relProjectPath && p.Workspace == workspace {
			workflowName := p.Workflow

			// If they didn't specify a workflow, use the default.
			if workflowName == "" {
				log.Info("no %s workflow set––continuing with defaults", AtlantisYAMLFilename)
				return defaults, nil
			}

			// If they did specify a workflow, find it.
			workflow, exists := config.Workflows[workflowName]
			if !exists {
				return nil, fmt.Errorf("no workflow with key %q defined", workflowName)
			}

			// We have a workflow defined, so now we need to build it.
			meta := r.buildMeta(log, repoDir, workspace, relProjectPath, extraCommentArgs, username)
			var steps []Step
			var stepsConfig []StepConfig
			if stageName == PlanStageName {
				stepsConfig = workflow.Plan.Steps
			} else {
				stepsConfig = workflow.Apply.Steps
			}
			for _, stepConfig := range stepsConfig {
				var step Step
				switch stepConfig.StepType {
				case "init":
					step = &InitStep{
						Meta:      meta,
						ExtraArgs: stepConfig.ExtraArgs,
					}
				case "plan":
					step = &PlanStep{
						Meta:      meta,
						ExtraArgs: stepConfig.ExtraArgs,
					}
				case "apply":
					step = &ApplyStep{
						Meta:      meta,
						ExtraArgs: stepConfig.ExtraArgs,
					}
				}
				// todo: custom step
				steps = append(steps, step)
			}
			return steps, nil
		}
	}
	return nil, fmt.Errorf("no project with dir %q and workspace %q defined", relProjectPath, workspace)
}

func (r *Reader) BuildApplyStage(log *logging.SimpleLogger, repoDir string, workspace string, relProjectPath string, extraCommentArgs []string, username string) (*ApplyStage, error) {
	defaults := r.defaultApplySteps(log, repoDir, workspace, relProjectPath, extraCommentArgs, username)
	steps, err := r.BuildStage(ApplyStageName, log, repoDir, workspace, relProjectPath, extraCommentArgs, username, defaults)
	if err != nil {
		return nil, err
	}
	return &ApplyStage{
		Steps: steps,
	}, nil
}

func (r *Reader) buildMeta(log *logging.SimpleLogger, repoDir string, workspace string, relProjectPath string, extraCommentArgs []string, username string) StepMeta {
	return StepMeta{
		Log:                   log,
		Workspace:             workspace,
		AbsolutePath:          filepath.Join(repoDir, relProjectPath),
		DirRelativeToRepoRoot: relProjectPath,
		// If there's no config then we should use the default tf version.
		TerraformVersion:  r.DefaultTFVersion,
		TerraformExecutor: r.TerraformExecutor,
		ExtraCommentArgs:  extraCommentArgs,
		Username:          username,
	}
}

func (r *Reader) defaultPlanSteps(log *logging.SimpleLogger, repoDir string, workspace string, relProjectPath string, extraCommentArgs []string, username string) []Step {
	meta := r.buildMeta(log, repoDir, workspace, relProjectPath, extraCommentArgs, username)
	return []Step{
		&InitStep{
			ExtraArgs: nil,
			Meta:      meta,
		},
		&PlanStep{
			ExtraArgs: nil,
			Meta:      meta,
		},
	}
}
func (r *Reader) defaultApplySteps(log *logging.SimpleLogger, repoDir string, workspace string, relProjectPath string, extraCommentArgs []string, username string) []Step {
	meta := r.buildMeta(log, repoDir, workspace, relProjectPath, extraCommentArgs, username)
	return []Step{
		&ApplyStep{
			ExtraArgs: nil,
			Meta:      meta,
		},
	}
}

func (r *Reader) ParseAndValidate(configData []byte) (RepoConfig, error) {
	var repoConfig RepoConfig
	if err := yaml.UnmarshalStrict(configData, &repoConfig); err != nil {
		// Unmarshal error messages aren't fit for user output. We need to
		// massage them.
		// todo: fix "field autoplan not found in struct repoconfig.alias" errors
		return repoConfig, errors.New(strings.Replace(err.Error(), " into repoconfig.RepoConfig", "", -1))
	}

	// Validate version.
	if repoConfig.Version != 2 {
		// todo: this will fail old atlantis.yaml files, we should deal with them in a better way.
		return repoConfig, errors.New("unknown version: must have \"version: 2\" set")
	}

	// Validate projects.
	if len(repoConfig.Projects) == 0 {
		return repoConfig, errors.New("'projects' key must exist and contain at least one element")
	}

	for i, project := range repoConfig.Projects {
		if project.Dir == "" {
			return repoConfig, fmt.Errorf("project at index %d invalid: dir key must be set and non-empty", i)
		}
	}
	return repoConfig, nil
}
