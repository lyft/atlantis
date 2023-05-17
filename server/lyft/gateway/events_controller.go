package gateway

import (
	"context"
	"net/http"

	"github.com/palantir/go-githubapp/githubapp"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/command"

	"github.com/runatlantis/atlantis/server/vcs/provider/github"

	events_controllers "github.com/runatlantis/atlantis/server/controllers/events"
	"github.com/runatlantis/atlantis/server/controllers/events/handlers"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/feature"
	"github.com/runatlantis/atlantis/server/neptune/gateway/config"
	"github.com/runatlantis/atlantis/server/neptune/gateway/deploy"
	"github.com/runatlantis/atlantis/server/neptune/gateway/deploy/requirement"
	gateway_handlers "github.com/runatlantis/atlantis/server/neptune/gateway/event"
	"github.com/runatlantis/atlantis/server/neptune/sync"
	converters "github.com/runatlantis/atlantis/server/vcs/provider/github/converter"
	"github.com/runatlantis/atlantis/server/vcs/provider/github/request"
	"github.com/uber-go/tally/v4"
	"go.temporal.io/sdk/client"
)

type scheduler interface {
	Schedule(ctx context.Context, f sync.Executor) error
}

func NewVCSEventsController(
	scope tally.Scope,
	webhookSecret []byte,
	allowDraftPRs bool,
	autoplanValidator *gateway_handlers.AutoplanValidator,
	snsWriter gateway_handlers.Writer,
	commentParser events.CommentParsing,
	repoAllowlistChecker *events.RepoAllowlistChecker,
	vcsClient vcs.Client,
	logger logging.Logger,
	supportedVCSProviders []models.VCSHostType,
	repoConverter converters.RepoConverter,
	pullConverter converters.PullConverter,
	githubClient converters.PullGetter,
	featureAllocator feature.Allocator,
	syncScheduler scheduler,
	asyncScheduler scheduler,
	temporalClient client.Client,
	rootDeployer *deploy.RootDeployer,
	rootConfigBuilder *config.Builder,
	deploySignaler *deploy.WorkflowSignaler,
	checkRunFetcher *github.CheckRunsFetcher,
	vcsStatusUpdater *command.VCSStatusUpdater,
	globalCfg valid.GlobalCfg,
	commentCreator *github.CommentCreator,
	clientCreator githubapp.ClientCreator,
) *VCSEventsController {
	pullEventWorkerProxy := gateway_handlers.NewPullEventWorkerProxy(
		snsWriter, logger,
	)

	asyncAutoplannerWorkerProxy := gateway_handlers.NewAutoplannerValidatorProxy(
		autoplanValidator, logger, pullEventWorkerProxy, asyncScheduler,
	)

	prHandler := handlers.NewPullRequestEventWithEventTypeHandlers(
		repoAllowlistChecker,
		asyncAutoplannerWorkerProxy,
		asyncAutoplannerWorkerProxy,
		pullEventWorkerProxy,
	)

	errorHandler := gateway_handlers.NewPREventErrorHandler(
		commentCreator,
		globalCfg,
		logger,
	)

	teamMemberFetcher := &github.TeamMemberFetcher{
		ClientCreator: clientCreator,

		// Using the policy set org for now, we should probably bundle team and org together in one struct though
		Org: globalCfg.PolicySets.Organization,
	}

	reviewFetcher := &github.PRReviewFetcher{
		ClientCreator: clientCreator,
	}

	requirementChecker := requirement.NewAggregate(globalCfg, teamMemberFetcher, reviewFetcher, checkRunFetcher, logger)
	commentHandler := handlers.NewCommentEventWithCommandHandler(
		commentParser,
		repoAllowlistChecker,
		vcsClient,
		gateway_handlers.NewCommentEventWorkerProxy(
			logger, scope.SubScope("comment"),
			snsWriter,
			featureAllocator, asyncScheduler,
			deploySignaler, vcsClient,
			vcsStatusUpdater, globalCfg,
			rootConfigBuilder, errorHandler,
			requirementChecker,
		),
		logger,
	)

	pushHandler := &gateway_handlers.PushHandler{
		Allocator:    featureAllocator,
		Scheduler:    asyncScheduler,
		Logger:       logger,
		RootDeployer: rootDeployer,
	}

	checkRunHandler := &gateway_handlers.CheckRunHandler{
		Logger:         logger,
		RootDeployer:   rootDeployer,
		SyncScheduler:  syncScheduler,
		AsyncScheduler: asyncScheduler,
		DeploySignaler: deploySignaler,
	}

	checkSuiteHandler := &gateway_handlers.CheckSuiteHandler{
		Logger:       logger,
		Scheduler:    asyncScheduler,
		RootDeployer: rootDeployer,
	}

	pullRequestReviewHandler := &gateway_handlers.PullRequestReviewWorkerProxy{
		Scheduler:       asyncScheduler,
		SnsWriter:       snsWriter,
		Logger:          logger,
		CheckRunFetcher: checkRunFetcher,
	}

	// lazy map of resolver providers to their resolver
	// laziness ensures we only instantiate the providers we support.
	providerResolverInitializer := map[models.VCSHostType]func() events_controllers.RequestResolver{
		models.Github: func() events_controllers.RequestResolver {
			return request.NewHandler(
				logger,
				scope,
				webhookSecret,
				commentHandler,
				prHandler,
				pushHandler,
				pullRequestReviewHandler,
				checkRunHandler,
				checkSuiteHandler,
				allowDraftPRs,
				repoConverter,
				pullConverter,
				githubClient,
			)
		},
	}

	router := &events_controllers.RequestRouter{
		Resolvers: events_controllers.NewRequestResolvers(providerResolverInitializer, supportedVCSProviders),
		Logger:    logger,
	}

	return &VCSEventsController{
		router: router,
	}
}

// TODO: remove this once event_controllers.VCSEventsController has the same function
// VCSEventsController handles all webhook requests which signify 'events' in the
// VCS host, ex. GitHub.
type VCSEventsController struct {
	router *events_controllers.RequestRouter
}

// Post handles POST webhook requests.
func (g *VCSEventsController) Post(w http.ResponseWriter, r *http.Request) {
	g.router.Route(w, r)
}
