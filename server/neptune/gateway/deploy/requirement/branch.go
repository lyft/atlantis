package requirement

import (
	"context"
	"fmt"

	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/template"
)

const (
	DefaultTemplate = "See error details below:"
)

type errGenerator[T any] interface {
	GenerateForbiddenError(ctx context.Context, key template.Key, repo models.Repo, data T, msg string, format ...any) ForbiddenError
}

type errorGenerator[T any] struct {
	loader template.Loader[T]
	logger logging.Logger
}

func (g errorGenerator[T]) GenerateForbiddenError(ctx context.Context, key template.Key, repo models.Repo, data T, msg string, format ...any) ForbiddenError {
	content, err := g.loader.Load(key, repo, data)
	if err != nil {
		g.logger.WarnContext(ctx, fmt.Sprintf("unable to load template %s", key))
		content = DefaultTemplate
	}

	return ForbiddenError{
		template: content,
		details:  fmt.Sprintf(msg, format...),
	}
}

type ForbiddenError struct {
	details  string
	template string
}

func NewForbiddenError(msg string, format ...any) ForbiddenError {
	return ForbiddenError{template: DefaultTemplate, details: fmt.Sprintf(msg, format...)}
}

// ErrorTemplate returns a human readable formatted error which
// is appropriate to surface to the client
func (e ForbiddenError) ErrorTemplate() string {
	return e.template
}

func (e ForbiddenError) Error() string {
	return e.details
}

type branchRestriction struct {
	cfg            valid.GlobalCfg
	errorGenerator errGenerator[template.BranchForbiddenData]
}

func (r *branchRestriction) Check(ctx context.Context, criteria Criteria) error {
	match := r.cfg.MatchingRepo(criteria.Repo.ID())

	if match.ApplySettings.BranchRestriction == valid.DefaultBranchRestriction && criteria.Repo.DefaultBranch != criteria.Branch {
		return r.errorGenerator.GenerateForbiddenError(
			ctx,
			template.BranchForbidden, criteria.Repo,
			template.BranchForbiddenData{
				DefaultBranch: criteria.Repo.DefaultBranch,
			},
			"deploys are forbidden on %s branch", criteria.Branch,
		)
	}

	return nil
}
