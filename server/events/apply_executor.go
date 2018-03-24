// Copyright 2017 HootSuite Media Inc.
//
// Licensed under the Apache License, Version 2.0 (the License);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// Modified hereafter by contributors to runatlantis/atlantis.
//
package events

import (
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/repoconfig"
	"github.com/runatlantis/atlantis/server/events/run"
	"github.com/runatlantis/atlantis/server/events/terraform"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/events/webhooks"
)

// ApplyExecutor handles executing terraform apply.
type ApplyExecutor struct {
	VCSClient         vcs.ClientProxy
	Terraform         *terraform.DefaultClient
	RequireApproval   bool
	Run               *run.Run
	AtlantisWorkspace AtlantisWorkspace
	ProjectPreExecute *DefaultProjectPreExecutor
	RepoConfigReader  repoconfig.Reader
	Webhooks          webhooks.Sender
}

// Execute executes apply for the ctx.
func (a *ApplyExecutor) Execute(ctx *CommandContext) CommandResponse {
	repoDir, err := a.AtlantisWorkspace.GetWorkspace(ctx.BaseRepo, ctx.Pull, ctx.Command.Workspace)
	if err != nil {
		return CommandResponse{Failure: "No workspace found. Did you run plan?"}
	}
	ctx.Log.Info("found workspace in %q", repoDir)

	stage, err := a.RepoConfigReader.BuildApplyStage(ctx.Log, repoDir, ctx.Command.Workspace, ctx.Command.Dir, ctx.Command.Flags, ctx.User.Username)
	if err != nil {
		return CommandResponse{Error: err}
	}

	// check if we have the lock
	preExecute := a.ProjectPreExecute.Execute(ctx, models.NewProject(ctx.BaseRepo.FullName, ctx.Command.Dir))
	if preExecute.ProjectResult != (ProjectResult{}) {
		return CommandResponse{ProjectResults: []ProjectResult{preExecute.ProjectResult}}
	}

	// Check apply requirements.
	for _, req := range stage.ApplyRequirements {
		isMet, reason := req.IsMet()
		if !isMet {
			return CommandResponse{Failure: reason}
		}
	}

	out, err := stage.Run()

	// Send webhooks even if there's an error.
	a.Webhooks.Send(ctx.Log, webhooks.ApplyResult{ // nolint: errcheck
		Workspace: ctx.Command.Workspace,
		User:      ctx.User,
		Repo:      ctx.BaseRepo,
		Pull:      ctx.Pull,
		Success:   err == nil,
	})

	if err != nil {
		return CommandResponse{Error: err}
	}
	return CommandResponse{ProjectResults: []ProjectResult{{ApplySuccess: out}}}

	// todo: move this into its own ApplyRequirement impl
	//if len(cfg.ApplyRequirements) > 0 {
	//	approved, err := a.VCSClient.PullIsApproved(ctx.BaseRepo, ctx.Pull, ctx.VCSHost)
	//	if err != nil {
	//		return CommandResponse{Error: errors.Wrap(err, "checking if pull request was approved")}
	//	}
	//	if !approved {
	//		return CommandResponse{Failure: "Pull request must be approved before running apply."}
	//	}
	//	ctx.Log.Info("confirmed pull request was approved")
	//}

	// Refactor: This is wrong now, there will only be one plan to apply when we get to this level.

	// Plans are stored at project roots by their workspace names. We just
	// need to find them.
	//var plans []models.Plan
	// If they didn't specify a directory, we apply all plans we can find for
	// this workspace.
	//if ctx.Command.Dir == "" {
	//	err = filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
	//		if err != nil {
	//			return err
	//		}
	//		// Check if the plan is for the right workspace,
	//		if !info.IsDir() && info.Name() == ctx.Command.Workspace+".tfplan" {
	//			rel, _ := filepath.Rel(repoDir, filepath.Dir(path))
	//			plans = append(plans, models.Plan{
	//				Project:   models.NewProject(ctx.BaseRepo.FullName, rel),
	//				LocalPath: path,
	//			})
	//		}
	//		return nil
	//	})
	//	if err != nil {
	//		return CommandResponse{Error: errors.Wrap(err, "finding plans")}
	//	}
	//} else {
	// If they did specify a dir, we apply just the plan in that directory
	// for this workspace.
	//planPath := filepath.Join(repoDir, ctx.Command.Dir, ctx.Command.Workspace+".tfplan")
	//stat, err := os.Stat(planPath)
	//if err != nil || stat.IsDir() {
	//	return CommandResponse{Error: fmt.Errorf("no plan found at path %q and workspace %q–did you run plan?", ctx.Command.Dir, ctx.Command.Workspace)}
	//}
	//relProjectPath, _ := filepath.Rel(repoDir, filepath.Dir(planPath))
	//plan := models.Plan{
	//	Project:   models.NewProject(ctx.BaseRepo.FullName, relProjectPath),
	//	LocalPath: planPath,
	//}
	//}
	//if len(plans) == 0 {
	//	return CommandResponse{Failure: "No plans found for that workspace."}
	//}
	//var paths []string
	//for _, p := range plans {
	//	paths = append(paths, p.LocalPath)
	//}
	//ctx.Log.Info("found %d plan(s) in our workspace: %v", len(plans), paths)

	//ctx.Log.Info("running apply for project at path %q", plan.Project.Path)
	//result := a.apply(ctx, repoDir, plan, cfg.Workflow.Apply)
	//result.Path = plan.LocalPath
	//return CommandResponse{ProjectResults: []ProjectResult{result}}
}

//func (a *ApplyExecutor) apply(ctx *CommandContext, repoDir string, plan models.Plan, steps []repoconfig.Step) ProjectResult {
// Still need preexecute to ensure we have the lock.. I think?
//// todo: do we need to check for lock when applying?
//preExecute := a.ProjectPreExecute.Execute(ctx, repoDir, plan.Project)
//if preExecute.ProjectResult != (ProjectResult{}) {
//	return preExecute.ProjectResult
//}

//config := preExecute.ProjectConfig
//terraformVersion := preExecute.TerraformVersion
//stepCtx := repoconfig.StepMeta{
//	Log:                   ctx.Log,
//	Workspace:             ctx.Command.Workspace,
//	AbsolutePath:          filepath.Join(repoDir, plan.Project.Path),
//	DirRelativeToRepoRoot: plan.Project.Path,
//	TerraformVersion:      a.Terraform.Version(),
//	ExtraCommentArgs:      ctx.Command.Flags,
//	Username:              ctx.User.Username,
//}

//var outputs []string
//var err error
//for _, step := range steps {
//	// todo: should be logging each step
//	var output string
//	output, err = step.Run()
//	outputs = append(outputs, output)
//	if err != nil {
//		// todo: error message should include the step that was being run
//		break
//	}
//}

//applyExtraArgs := config.GetExtraArguments(ctx.Command.Name.String())
//absolutePath := filepath.Join(repoDir, plan.Project.Path)
//workspace := ctx.Command.Workspace
//tfApplyCmd := append(append(append([]string{"apply", "-no-color"}, applyExtraArgs...), ctx.Command.Flags...), plan.LocalPath)
//output, err := a.Terraform.RunCommandWithVersion(ctx.Log, absolutePath, tfApplyCmd, terraformVersion, workspace)

//
//	finalOutput := strings.Join(outputs, "\n")
//	if err != nil {
//		return ProjectResult{Error: fmt.Errorf("%s\n%s", err.Error(), finalOutput)}
//	}
//	ctx.Log.Info("apply succeeded")
//
//	return ProjectResult{ApplySuccess: finalOutput}
//}
