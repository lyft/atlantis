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

package events

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	_ "embed"

	"github.com/Masterminds/sprig/v3"
	"github.com/runatlantis/atlantis/server/events/models"
)

var (
	planCommandTitle            = models.PlanCommand.TitleString()
	applyCommandTitle           = models.ApplyCommand.TitleString()
	policyCheckCommandTitle     = models.PolicyCheckCommand.TitleString()
	approvePoliciesCommandTitle = models.ApprovePoliciesCommand.TitleString()
	versionCommandTitle         = models.VersionCommand.TitleString()
	// maxUnwrappedLines is the maximum number of lines the Terraform output
	// can be before we wrap it in an expandable template.
	maxUnwrappedLines = 12
)

// MarkdownRenderer renders responses as markdown.
type MarkdownRenderer struct {
	// GitlabSupportsCommonMark is true if the version of GitLab we're
	// using supports the CommonMark markdown format.
	// If we're not configured with a GitLab client, this will be false.
	GitlabSupportsCommonMark bool
	DisableApplyAll          bool
	DisableApply             bool
	DisableMarkdownFolding   bool
	DisableRepoLocking       bool
	EnableDiffMarkdownFormat bool
}

// commonData is data that all responses have.
type commonData struct {
	Command                  string
	Verbose                  bool
	Log                      string
	PlansDeleted             bool
	DisableApplyAll          bool
	DisableApply             bool
	DisableRepoLocking       bool
	EnableDiffMarkdownFormat bool
}

// errData is data about an error response.
type errData struct {
	Error string
	commonData
}

// failureData is data about a failure response.
type failureData struct {
	Failure string
	commonData
}

// resultData is data about a successful response.
type resultData struct {
	Results []projectResultTmplData
	commonData
}

type planSuccessData struct {
	models.PlanSuccess
	PlanSummary              string
	PlanWasDeleted           bool
	DisableApply             bool
	DisableRepoLocking       bool
	EnableDiffMarkdownFormat bool
}

type policyCheckSuccessData struct {
	models.PolicyCheckSuccess
}

type projectResultTmplData struct {
	Workspace   string
	RepoRelDir  string
	ProjectName string
	Rendered    string
}

// Render formats the data into a markdown string.
// nolint: interfacer
func (m *MarkdownRenderer) Render(res CommandResult, cmdName models.CommandName, log string, verbose bool, vcsHost models.VCSHostType, templateOverrides map[string]string) string {
	commandStr := strings.Title(strings.Replace(cmdName.String(), "_", " ", -1))
	common := commonData{
		Command:                  commandStr,
		Verbose:                  verbose,
		Log:                      log,
		PlansDeleted:             res.PlansDeleted,
		DisableApplyAll:          m.DisableApplyAll || m.DisableApply,
		DisableApply:             m.DisableApply,
		DisableRepoLocking:       m.DisableRepoLocking,
		EnableDiffMarkdownFormat: m.EnableDiffMarkdownFormat,
	}
	if res.Error != nil {
		return m.renderTemplate(unwrappedErrWithLogTmpl, errData{res.Error.Error(), common})
	}
	if res.Failure != "" {
		return m.renderTemplate(failureWithLogTmpl, failureData{res.Failure, common})
	}
	return m.renderProjectResults(res.ProjectResults, common, vcsHost, templateOverrides)
}

func (m *MarkdownRenderer) renderProjectResults(results []models.ProjectResult, common commonData, vcsHost models.VCSHostType, templateOverrides map[string]string) string {
	var resultsTmplData []projectResultTmplData
	numPlanSuccesses := 0
	numPolicyCheckSuccesses := 0
	numVersionSuccesses := 0

	for _, result := range results {
		resultData := projectResultTmplData{
			Workspace:   result.Workspace,
			RepoRelDir:  result.RepoRelDir,
			ProjectName: result.ProjectName,
		}
		if result.Error != nil {
			tmpl := getUnwrappedErrTmpl(templateOverrides)
			if m.shouldUseWrappedTmpl(vcsHost, result.Error.Error()) {
				tmpl = getWrappedErrTmpl(templateOverrides)
			}
			resultData.Rendered = m.renderTemplate(tmpl, struct {
				Command string
				Error   string
			}{
				Command: common.Command,
				Error:   result.Error.Error(),
			})
		} else if result.Failure != "" {
			resultData.Rendered = m.renderTemplate(getFailureTmpl(templateOverrides), struct {
				Command string
				Failure string
			}{
				Command: common.Command,
				Failure: result.Failure,
			})
		} else if result.PlanSuccess != nil {
			if m.shouldUseWrappedTmpl(vcsHost, result.PlanSuccess.TerraformOutput) {
				resultData.Rendered = m.renderTemplate(getPlanSuccessWrappedTmpl(templateOverrides), planSuccessData{PlanSuccess: *result.PlanSuccess, PlanSummary: result.PlanSuccess.Summary(), PlanWasDeleted: common.PlansDeleted, DisableApply: common.DisableApply, DisableRepoLocking: common.DisableRepoLocking, EnableDiffMarkdownFormat: common.EnableDiffMarkdownFormat})
			} else {
				resultData.Rendered = m.renderTemplate(getPlanSuccessUnwrappedTmpl(templateOverrides), planSuccessData{PlanSuccess: *result.PlanSuccess, PlanWasDeleted: common.PlansDeleted, DisableApply: common.DisableApply, DisableRepoLocking: common.DisableRepoLocking, EnableDiffMarkdownFormat: common.EnableDiffMarkdownFormat})
			}
			numPlanSuccesses++
		} else if result.PolicyCheckSuccess != nil {
			if m.shouldUseWrappedTmpl(vcsHost, result.PolicyCheckSuccess.PolicyCheckOutput) {
				resultData.Rendered = m.renderTemplate(getPolicyCheckSuccessWrappedTmpl(templateOverrides), policyCheckSuccessData{PolicyCheckSuccess: *result.PolicyCheckSuccess})
			} else {
				resultData.Rendered = m.renderTemplate(getPolicyCheckSuccessUnwrappedTmpl(templateOverrides), policyCheckSuccessData{PolicyCheckSuccess: *result.PolicyCheckSuccess})
			}
			numPolicyCheckSuccesses++
		} else if result.ApplySuccess != "" {
			if m.shouldUseWrappedTmpl(vcsHost, result.ApplySuccess) {
				resultData.Rendered = m.renderTemplate(getApplyWrappedSuccessTmpl(templateOverrides), struct{ Output string }{result.ApplySuccess})
			} else {
				resultData.Rendered = m.renderTemplate(getApplyUnwrappedSuccessTmpl(templateOverrides), struct{ Output string }{result.ApplySuccess})
			}
		} else if result.VersionSuccess != "" {
			if m.shouldUseWrappedTmpl(vcsHost, result.VersionSuccess) {
				resultData.Rendered = m.renderTemplate(getVersionWrappedSuccessTmpl(templateOverrides), struct{ Output string }{result.VersionSuccess})
			} else {
				resultData.Rendered = m.renderTemplate(getVersionUnwrappedSuccessTmpl(templateOverrides), struct{ Output string }{result.VersionSuccess})
			}
			numVersionSuccesses++
		} else {
			resultData.Rendered = "Found no template. This is a bug!"
		}
		resultsTmplData = append(resultsTmplData, resultData)
	}

	var tmpl *template.Template
	switch {
	case len(resultsTmplData) == 1 && common.Command == planCommandTitle && numPlanSuccesses > 0:
		tmpl = getSingleProjectPlanSuccessTmpl(templateOverrides)
	case len(resultsTmplData) == 1 && common.Command == planCommandTitle && numPlanSuccesses == 0:
		tmpl = getSingleProjectPlanUnsuccessfulTmpl(templateOverrides)
	case len(resultsTmplData) == 1 && common.Command == policyCheckCommandTitle && numPolicyCheckSuccesses > 0:
		tmpl = getSingleProjectPlanSuccessTmpl(templateOverrides)
	case len(resultsTmplData) == 1 && common.Command == policyCheckCommandTitle && numPolicyCheckSuccesses == 0:
		tmpl = getSingleProjectPlanUnsuccessfulTmpl(templateOverrides)
	case len(resultsTmplData) == 1 && common.Command == versionCommandTitle && numVersionSuccesses > 0:
		tmpl = getSingleProjectVersionSuccessTmpl(templateOverrides)
	case len(resultsTmplData) == 1 && common.Command == versionCommandTitle && numVersionSuccesses == 0:
		tmpl = getSingleProjectVersionUnsuccessfulTmpl(templateOverrides)
	case len(resultsTmplData) == 1 && common.Command == applyCommandTitle:
		tmpl = getSingleProjectApplyTmpl(templateOverrides)
	case common.Command == planCommandTitle,
		common.Command == policyCheckCommandTitle:
		tmpl = getMultiProjectPlanTmpl(templateOverrides)
	case common.Command == approvePoliciesCommandTitle:
		tmpl = getApproveAllProjectsTmpl(templateOverrides)
	case common.Command == applyCommandTitle:
		tmpl = getMultiProjectApplyTmpl(templateOverrides)
	case common.Command == versionCommandTitle:
		tmpl = getMultiProjectVersionTmpl(templateOverrides)
	default:
		return "no template matched–this is a bug"
	}
	return m.renderTemplate(tmpl, resultData{resultsTmplData, common})
}

// shouldUseWrappedTmpl returns true if we should use the wrapped markdown
// templates that collapse the output to make the comment smaller on initial
// load. Some VCS providers or versions of VCS providers don't support this
// syntax.
func (m *MarkdownRenderer) shouldUseWrappedTmpl(vcsHost models.VCSHostType, output string) bool {
	if m.DisableMarkdownFolding {
		return false
	}

	// Bitbucket Cloud and Server don't support the folding markdown syntax.
	if vcsHost == models.BitbucketServer || vcsHost == models.BitbucketCloud {
		return false
	}

	if vcsHost == models.Gitlab && !m.GitlabSupportsCommonMark {
		return false
	}

	return strings.Count(output, "\n") > maxUnwrappedLines
}

func (m *MarkdownRenderer) renderTemplate(tmpl *template.Template, data interface{}) string {
	buf := &bytes.Buffer{}
	if err := tmpl.Execute(buf, data); err != nil {
		return fmt.Sprintf("Failed to render template, this is a bug: %v", err)
	}
	return buf.String()
}

func getUnwrappedErrTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["unwrappedErrTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return unwrappedErrTmpl
}

func getWrappedErrTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["wrappedErrTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return wrappedErrTmpl
}

func getFailureTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["failureTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return failureTmpl
}

func getPlanSuccessWrappedTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["planSuccessWrappedTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return template.Must(template.New("").Parse(planSuccessWrappedTmpl))
}

func getPlanSuccessUnwrappedTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["planSuccessUnwrappedTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return template.Must(template.New("").Parse(planSuccessUnwrappedTmpl))
}

func getPolicyCheckSuccessWrappedTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["policyCheckSuccessWrappedTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return policyCheckSuccessWrappedTmpl
}

func getPolicyCheckSuccessUnwrappedTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["policyCheckSuccessUnwrappedTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return policyCheckSuccessUnwrappedTmpl
}

func getApplyWrappedSuccessTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["applyWrappedSuccessTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return applyWrappedSuccessTmpl
}
func getApplyUnwrappedSuccessTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["applyUnwrappedSuccessTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return applyUnwrappedSuccessTmpl
}

func getVersionWrappedSuccessTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["versionWrappedSuccessTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return versionWrappedSuccessTmpl
}

func getVersionUnwrappedSuccessTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["versionUnwrappedSuccessTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return versionUnwrappedSuccessTmpl
}

func getSingleProjectPlanSuccessTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["singleProjectPlanSuccessTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return template.Must(template.New("").Parse(singleProjectPlanSuccessTmpl))
}

func getSingleProjectPlanUnsuccessfulTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["singleProjectPlanUnsuccessfulTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return template.Must(template.New("").Parse(singleProjectPlanUnsuccessfulTmpl))
}

func getSingleProjectVersionSuccessTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["singleProjectVersionSuccessTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return template.Must(template.New("").Parse(singleProjectVersionSuccessTmpl))
}

func getSingleProjectVersionUnsuccessfulTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["singleProjectVersionUnsuccessfulTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return template.Must(template.New("").Parse(singleProjectVersionUnsuccessfulTmpl))
}

func getSingleProjectApplyTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["singleProjectApplyTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return template.Must(template.New("").Parse(singleProjectApplyTmpl))
}

func getMultiProjectPlanTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["multiProjectPlanTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return template.Must(template.New("").Funcs(sprig.TxtFuncMap()).Parse(multiProjectPlanTmpl))
}

func getApproveAllProjectsTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["approveAllProjectsTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return template.Must(template.New("").Funcs(sprig.TxtFuncMap()).Parse(approveAllProjectsTmpl))
}

func getMultiProjectApplyTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["multiProjectApplyTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return template.Must(template.New("").Funcs(sprig.TxtFuncMap()).Parse(multiProjectApplyTmpl))
}

func getMultiProjectVersionTmpl(templateOverrides map[string]string) *template.Template {
	if val, ok := templateOverrides["multiProjectVersionTmpl"]; ok {
		return template.Must(template.ParseFiles(val))
	}
	return template.Must(template.New("").Funcs(sprig.TxtFuncMap()).Parse(multiProjectVersionTmpl))
}

//go:embed templates/singleProjectApply.tmpl
var singleProjectApplyTmpl string

//go:embed templates/singleProjectPlanSuccess.tmpl
var singleProjectPlanSuccessTmpl string

//go:embed templates/singleProjectPlanUnsuccessful.tmpl
var singleProjectPlanUnsuccessfulTmpl string

//go:embed templates/singleProjectVersionSuccess.tmpl
var singleProjectVersionSuccessTmpl string

//go:embed templates/singleProjectVersionUnsuccessful.tmpl
var singleProjectVersionUnsuccessfulTmpl string

//go:embed templates/singleProjectVersionUnsuccessful.tmpl
var approveAllProjectsTmpl string

//go:embed templates/multiProjectPlan.tmpl
var multiProjectPlanTmpl string

//go:embed templates/multiProjectApply.tmpl
var multiProjectApplyTmpl string

//go:embed templates/multiProjectApply.tmpl
var multiProjectVersionTmpl string

//go:embed templates/planSuccessUnwrapped.tmpl
var planSuccessUnwrappedTmpl string

//go:embed templates/planSuccessWrapped.tmpl
var planSuccessWrappedTmpl string

var policyCheckSuccessUnwrappedTmpl = template.Must(template.New("").Parse(
	"```diff\n" +
		"{{.PolicyCheckOutput}}\n" +
		"```\n\n" + policyCheckNextSteps +
		"{{ if .HasDiverged }}\n\n:warning: The branch we're merging into is ahead, it is recommended to pull new commits first.{{end}}"))

var policyCheckSuccessWrappedTmpl = template.Must(template.New("").Parse(
	"<details><summary>Show Output</summary>\n\n" +
		"```diff\n" +
		"{{.PolicyCheckOutput}}\n" +
		"```\n\n" +
		policyCheckNextSteps + "\n" +
		"</details>" +
		"{{ if .HasDiverged }}\n\n:warning: The branch we're merging into is ahead, it is recommended to pull new commits first.{{end}}"))

// policyCheckNextSteps are instructions appended after successful plans as to what
// to do next.
var policyCheckNextSteps = "* :arrow_forward: To **apply** this plan, comment:\n" +
	"    * `{{.ApplyCmd}}`\n" +
	"* :put_litter_in_its_place: To **delete** this plan click [here]({{.LockURL}})\n" +
	"* :repeat: To re-run policies **plan** this project again by commenting:\n" +
	"    * `{{.RePlanCmd}}`"

// planNextSteps are instructions appended after successful plans as to what
// to do next.
var planNextSteps = "{{ if .PlanWasDeleted }}This plan was not saved because one or more projects failed and automerge requires all plans pass.{{ else }}" +
	"{{ if not .DisableApply }}* :arrow_forward: To **apply** this plan, comment:\n" +
	"    * `{{.ApplyCmd}}`\n{{end}}" +
	"{{ if not .DisableRepoLocking }}* :put_litter_in_its_place: To **delete** this plan click [here]({{.LockURL}})\n{{end}}" +
	"* :repeat: To **plan** this project again, comment:\n" +
	"    * `{{.RePlanCmd}}`{{end}}"
var applyUnwrappedSuccessTmpl = template.Must(template.New("").Parse(
	"```diff\n" +
		"{{.Output}}\n" +
		"```"))
var applyWrappedSuccessTmpl = template.Must(template.New("").Parse(
	"<details><summary>Show Output</summary>\n\n" +
		"```diff\n" +
		"{{.Output}}\n" +
		"```\n" +
		"</details>"))
var versionUnwrappedSuccessTmpl = template.Must(template.New("").Parse("```\n{{.Output}}```"))
var versionWrappedSuccessTmpl = template.Must(template.New("").Parse(
	"<details><summary>Show Output</summary>\n\n" +
		"```\n" +
		"{{.Output}}" +
		"```\n" +
		"</details>"))
var unwrappedErrTmplText = "**{{.Command}} Error**\n" +
	"```\n" +
	"{{.Error}}\n" +
	"```" +
	"{{ if eq .Command \"Policy Check\" }}" +
	"\n* :heavy_check_mark: To **approve** failing policies either request an approval from approvers or address the failure by modifying the codebase.\n" +
	"{{ end }}"
var wrappedErrTmplText = "**{{.Command}} Error**\n" +
	"<details><summary>Show Output</summary>\n\n" +
	"```\n" +
	"{{.Error}}\n" +
	"```\n</details>"
var unwrappedErrTmpl = template.Must(template.New("").Parse(unwrappedErrTmplText))
var unwrappedErrWithLogTmpl = template.Must(template.New("").Parse(unwrappedErrTmplText + logTmpl))
var wrappedErrTmpl = template.Must(template.New("").Parse(wrappedErrTmplText))
var failureTmplText = "**{{.Command}} Failed**: {{.Failure}}"
var failureTmpl = template.Must(template.New("").Parse(failureTmplText))
var failureWithLogTmpl = template.Must(template.New("").Parse(failureTmplText + logTmpl))
var logTmpl = "{{if .Verbose}}\n<details><summary>Log</summary>\n  <p>\n\n```\n{{.Log}}```\n</p></details>{{end}}\n"
