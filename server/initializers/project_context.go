package initializers

import (
	"github.com/runatlantis/atlantis/server/events"
	"github.com/uber-go/tally"
)

type projectContext struct {
	events.ProjectCommandContextBuilder
	commentBuilder events.CommentBuilder
}

func InitProjectContext(
	commentBuilder events.CommentBuilder,
) *projectContext {
	return &projectContext{
		events.NewProjectCommandContextBuilder(commentBuilder),
		commentBuilder,
	}
}

// InitPRProjectContext initializes context builder that uses
// pull_request_workflows used by platform mode
func InitPRProjectContext(
	commentBuilder events.CommentBuilder,
) *projectContext {
	return &projectContext{
		events.NewPRProjectCommandContextBuilder(commentBuilder),
		commentBuilder,
	}
}

func (p *projectContext) WithPolicyChecks() *projectContext {
	p.ProjectCommandContextBuilder = &events.PolicyCheckProjectContextBuilder{
		ProjectCommandContextBuilder: p.ProjectCommandContextBuilder,
		CommentBuilder:               p.commentBuilder,
	}
	return p
}

func (p *projectContext) WithInstrumentation(scope tally.Scope) *projectContext {
	p.ProjectCommandContextBuilder = &events.InstrumentedProjectCommandContextBuilder{
		ProjectCommandContextBuilder: p.ProjectCommandContextBuilder,
		ProjectCounter:               scope.Counter("projects"),
	}
	return p
}
