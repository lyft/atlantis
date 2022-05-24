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

	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
)

// MarkdownRenderer renders responses as markdown.
type MarkdownRenderer struct {
	TemplateResolver TemplateResolver

	DisableApplyAll          bool
	DisableApply             bool
	EnableDiffMarkdownFormat bool
}

// commonData is data that all responses have.
type commonData struct {
	Command                  string
	DisableApplyAll          bool
	DisableApply             bool
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

type projectResultTmplData struct {
	Workspace   string
	RepoRelDir  string
	ProjectName string
	Rendered    string
}

// Render formats the data into a markdown string.
// nolint: interfacer
func (m *MarkdownRenderer) Render(res command.Result, cmdName command.Name, baseRepo models.Repo) string {
	commandStr := strings.Title(strings.Replace(cmdName.String(), "_", " ", -1))
	common := commonData{
		Command:                  commandStr,
		DisableApplyAll:          m.DisableApplyAll || m.DisableApply,
		DisableApply:             m.DisableApply,
		EnableDiffMarkdownFormat: m.EnableDiffMarkdownFormat,
	}
	if res.Error != nil {
		return m.renderTemplate(template.Must(template.New("").Parse(unwrappedErrWithLogTmpl)), errData{res.Error.Error(), common})
	}
	if res.Failure != "" {
		return m.renderTemplate(template.Must(template.New("").Parse(failureWithLogTmpl)), failureData{res.Failure, common})
	}
	return m.renderProjectResults(res.ProjectResults, common, baseRepo)
}

func (m *MarkdownRenderer) renderProjectResults(results []command.ProjectResult, common commonData, baseRepo models.Repo) string {
	// render project results
	var prjResultTmplData []projectResultTmplData
	for _, result := range results {
		template, templateData := m.TemplateResolver.ResolveProject(result, baseRepo, common)
		renderedOutput := m.renderTemplate(template, templateData)
		prjResultTmplData = append(prjResultTmplData, projectResultTmplData{
			Workspace:   result.Workspace,
			RepoRelDir:  result.RepoRelDir,
			ProjectName: result.ProjectName,
			Rendered:    renderedOutput,
		})
	}

	// render aggregate operation result
	numPlanSuccesses, numPolicyCheckSuccesses, numVersionSuccesses := m.countSuccesses(results)
	tmpl := m.TemplateResolver.Resolve(common, baseRepo, len(results), numPlanSuccesses, numPolicyCheckSuccesses, numVersionSuccesses)
	if tmpl == nil {
		return "no template matched–this is a bug"
	}
	return m.renderTemplate(tmpl, resultData{prjResultTmplData, common})
}

func (m *MarkdownRenderer) countSuccesses(results []command.ProjectResult) (numPlanSuccesses, numPolicyCheckSuccesses, numVersionSuccesses int) {
	for _, result := range results {
		switch {
		case result.PlanSuccess != nil:
			numPlanSuccesses += 1
		case result.PolicyCheckSuccess != nil:
			numPolicyCheckSuccesses += 1
		case result.VersionSuccess != "":
			numVersionSuccesses += 1
		}
	}
	return
}

func (m *MarkdownRenderer) renderTemplate(tmpl *template.Template, data interface{}) string {
	buf := &bytes.Buffer{}
	if err := tmpl.Execute(buf, data); err != nil {
		return fmt.Sprintf("Failed to render template, this is a bug: %v", err)
	}
	return buf.String()
}
