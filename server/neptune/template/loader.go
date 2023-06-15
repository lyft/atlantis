package template

import (
	"bytes"
	"fmt"
	"html/template"
	"os"

	_ "embed" // embedding files

	"github.com/Masterminds/sprig/v3"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/models"
)

type Key string

// input for the template to be loaded
// add fields here as necessary
type Input struct {
}

// list of all valid template ids
const (
	PRComment             = Key("pr_comment")
	BranchForbidden       = Key("branch_forbidden")
	UserForbidden         = Key("user_forbidden")
	ApprovalRequired      = Key("approval_required")
	PlanValidationSuccess = Key("plan_validation_success")
)

var defaultTemplates = map[Key]string{
	PRComment:             prCommentTemplate,
	BranchForbidden:       branchForbiddenTemplate,
	UserForbidden:         userForbiddenTemplate,
	ApprovalRequired:      approvalRequiredTemplate,
	PlanValidationSuccess: planValidationSuccessTemplate,
}

type PRCommentData struct {
	ForbiddenError         bool
	ForbiddenErrorTemplate string
	InternalError          bool
	Command                string
	ErrorDetails           string
}

type CheckRun struct {
	Name string
	URL  string
}

type PlanValidationSuccessData struct {
	CheckRuns []CheckRun
}

type BranchForbiddenData struct {
	DefaultBranch string
}

type UserForbiddenData struct {
	User string
	Team string
	Org  string
}

//go:embed templates/pr_comment.tmpl
var prCommentTemplate string

//go:embed templates/branch_forbidden.tmpl
var branchForbiddenTemplate string

//go:embed templates/user_forbidden.tmpl
var userForbiddenTemplate string

//go:embed templates/approval_required.tmpl
var approvalRequiredTemplate string

//go:embed templates/plan_validation_success.tmpl
var planValidationSuccessTemplate string

type Loader[T any] struct {
	GlobalCfg valid.GlobalCfg
}

func NewLoader[T any](globalCfg valid.GlobalCfg) Loader[T] {
	return Loader[T]{
		GlobalCfg: globalCfg,
	}
}

func (l Loader[T]) Load(id Key, repo models.Repo, data T) (string, error) {
	tmpl := template.Must(l.getTemplate(id, repo))

	buf := &bytes.Buffer{}
	if err := tmpl.Execute(buf, data); err != nil {
		return "", fmt.Errorf("Failed to render template: %v", err)
	}
	return buf.String(), nil
}

func (l Loader[T]) getTemplate(id Key, repo models.Repo) (*template.Template, error) {
	var templateOverrides map[string]string

	repoCfg := l.GlobalCfg.MatchingRepo(repo.ID())
	if repoCfg != nil {
		templateOverrides = repoCfg.TemplateOverrides
	}

	template := template.New("").Funcs(sprig.TxtFuncMap())
	if fileName, ok := templateOverrides[string(id)]; ok {
		if content, err := os.ReadFile(fileName); err == nil {
			return template.Parse(string(content))
		}
	}

	return template.Parse(defaultTemplates[id])
}
