package events

import (
	"github.com/runatlantis/atlantis/server/events/models"
)

func NewPolicyCheckProjectCommandBuilder(p *DefaultProjectCommandBuilder) ProjectCommandBuilder {
	return &DefaultProjectCommandBuilder{
		ParserValidator:       p.ParserValidator,
		ProjectFinder:         p.ProjectFinder,
		VCSClient:             p.VCSClient,
		WorkingDir:            p.WorkingDir,
		WorkingDirLocker:      p.WorkingDirLocker,
		CommentBuilder:        p.CommentBuilder,
		GlobalCfg:             p.GlobalCfg,
		ProjectContextBuilder: &models.PolicyCheckProjectContextBuilder{},
	}
}
