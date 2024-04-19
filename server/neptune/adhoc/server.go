package adhoc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/palantir/go-githubapp/githubapp"
	"github.com/runatlantis/atlantis/server/legacy/events/vcs"
	"github.com/runatlantis/atlantis/server/neptune/sync/crons"
	ghClient "github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/vcs/provider/github"
	"github.com/runatlantis/atlantis/server/vcs/provider/github/converter"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/metrics"
	adhoc "github.com/runatlantis/atlantis/server/neptune/adhoc/adhocexecutionhelpers"
	adhocconfig "github.com/runatlantis/atlantis/server/neptune/adhoc/config"
	root_config "github.com/runatlantis/atlantis/server/neptune/gateway/config"
	"github.com/runatlantis/atlantis/server/neptune/gateway/deploy"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event/preworkflow"
	neptune_http "github.com/runatlantis/atlantis/server/neptune/http"
	internalSync "github.com/runatlantis/atlantis/server/neptune/sync"
	"github.com/runatlantis/atlantis/server/neptune/temporal"
	neptune "github.com/runatlantis/atlantis/server/neptune/temporalworker/config"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/static"
	"github.com/uber-go/tally/v4"
	"github.com/urfave/negroni"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
)

type Server struct {
	Logger                logging.Logger
	CronScheduler         *internalSync.CronScheduler
	Crons                 []*internalSync.Cron
	HTTPServerProxy       *neptune_http.ServerProxy
	Port                  int
	StatsScope            tally.Scope
	StatsCloser           io.Closer
	TemporalClient        *temporal.ClientWrapper
	TerraformActivities   *activities.Terraform
	GithubActivities      *activities.Github
	TerraformTaskQueue    string
	RootConfigBuilder     *root_config.Builder
	Repo                  string
	PRNum                 int
	InstallationRetriever *github.InstallationRetriever
	PullFetcher           *github.PRFetcher
	PullConverter         converter.PullConverter
}

func NewServer(config *adhocconfig.Config) (*Server, error) {
	statsReporter, err := metrics.NewReporter(config.Metrics, config.CtxLogger)

	if err != nil {
		return nil, err
	}

	scope, statsCloser := metrics.NewScopeWithReporter(config.Metrics, config.CtxLogger, config.StatsNamespace, statsReporter)
	if err != nil {
		return nil, err
	}

	scope = scope.Tagged(map[string]string{
		"mode": "adhoc",
	})

	opts := &temporal.Options{
		StatsReporter: statsReporter,
	}
	opts = opts.WithClientInterceptors(temporal.NewMetricsInterceptor(scope))
	temporalClient, err := temporal.NewClient(config.CtxLogger, config.TemporalCfg, opts)
	if err != nil {
		return nil, errors.Wrap(err, "initializing temporal client")
	}

	router := mux.NewRouter()
	router.HandleFunc("/healthz", Healthz).Methods(http.MethodGet)
	router.PathPrefix("/static/").Handler(http.FileServer(&assetfs.AssetFS{Asset: static.Asset, AssetDir: static.AssetDir, AssetInfo: static.AssetInfo}))
	n := negroni.New(&negroni.Recovery{
		Logger:     log.New(os.Stdout, "", log.LstdFlags),
		PrintStack: false,
		StackAll:   false,
		StackSize:  1024 * 8,
	})
	n.UseHandler(router)
	httpServerProxy := &neptune_http.ServerProxy{
		SSLCertFile: config.AuthCfg.SslCertFile,
		SSLKeyFile:  config.AuthCfg.SslKeyFile,
		Server:      &http.Server{Addr: fmt.Sprintf(":%d", config.ServerCfg.Port), Handler: n, ReadHeaderTimeout: time.Second * 10},
		Logger:      config.CtxLogger,
	}

	terraformActivities, err := activities.NewTerraform(
		config.TerraformCfg,
		neptune.ValidationConfig{},
		config.App,
		config.DataDir,
		config.ServerCfg.URL,
		config.TemporalCfg.TerraformTaskQueue,
		config.GithubCfg.TemporalAppInstallationID,
		nil,
	)
	if err != nil {
		return nil, errors.Wrap(err, "initializing terraform activities")
	}
	clientCreator, err := githubapp.NewDefaultCachingClientCreator(
		config.App,
		githubapp.WithClientMiddleware(
			ghClient.ClientMetrics(scope.SubScope("app")),
		))
	if err != nil {
		return nil, errors.Wrap(err, "client creator")
	}

	installationFetcher := &github.InstallationRetriever{
		ClientCreator: clientCreator,
	}
	githubActivities, err := activities.NewGithub(
		clientCreator,
		config.GithubCfg.TemporalAppInstallationID,
		config.DataDir,
		nil,
	)
	if err != nil {
		return nil, errors.Wrap(err, "initializing github activities")
	}

	cronScheduler := internalSync.NewCronScheduler(config.CtxLogger)

	privateKey, err := os.ReadFile(config.GithubAppKeyFile)
	if err != nil {
		return nil, err
	}
	githubCredentials := &vcs.GithubAppCredentials{
		AppID:    config.GithubAppID,
		Key:      privateKey,
		Hostname: config.GithubHostname,
		AppSlug:  config.GithubAppSlug,
	}

	repoFetcher := &github.RepoFetcher{
		DataDir:           config.DataDir,
		GithubCredentials: githubCredentials,
		GithubHostname:    config.GithubHostname,
		Logger:            config.CtxLogger,
		Scope:             scope.SubScope("repo.fetch"),
	}

	hooksRunner := &preworkflow.HooksRunner{
		GlobalCfg: config.GlobalCfg,
		HookExecutor: &preworkflow.HookExecutor{
			Logger: config.CtxLogger,
		},
	}

	rootConfigBuilder := &root_config.Builder{
		RepoFetcher:     repoFetcher,
		HooksRunner:     hooksRunner,
		ParserValidator: &root_config.ParserValidator{GlobalCfg: config.GlobalCfg},
		Strategy: &root_config.ModifiedRootsStrategy{
			RootFinder:  &deploy.RepoRootFinder{Logger: config.CtxLogger},
			FileFetcher: &github.RemoteFileFetcher{ClientCreator: clientCreator},
		},
		GlobalCfg: config.GlobalCfg,
		Logger:    config.CtxLogger,
		Scope:     scope.SubScope("event.filters.root"),
	}

	pullFetcher := &github.PRFetcher{
		ClientCreator: clientCreator,
	}

	repoConverter := converter.RepoConverter{
		GithubUser:  config.GithubUser,
		GithubToken: config.GithubToken,
	}

	pullConverter := converter.PullConverter{
		RepoConverter: repoConverter,
	}

	config.CtxLogger.Info(fmt.Sprintf("Starting adhoc server, params are: repo: %s, root: %s, revision: %s", config.GlobalCfg.AdhocMode.Repo, config.GlobalCfg.AdhocMode.Root, config.GlobalCfg.AdhocMode.Revision))

	server := Server{
		Logger:        config.CtxLogger,
		CronScheduler: cronScheduler,
		Crons: []*internalSync.Cron{
			{
				Executor:  crons.NewRuntimeStats(scope).Run,
				Frequency: 1 * time.Minute,
			},
		},
		HTTPServerProxy:       httpServerProxy,
		Port:                  config.ServerCfg.Port,
		StatsScope:            scope,
		StatsCloser:           statsCloser,
		TemporalClient:        temporalClient,
		TerraformActivities:   terraformActivities,
		TerraformTaskQueue:    config.TemporalCfg.TerraformTaskQueue,
		GithubActivities:      githubActivities,
		RootConfigBuilder:     rootConfigBuilder,
		Repo:                  config.GlobalCfg.AdhocMode.Repo,
		PRNum:                 config.GlobalCfg.AdhocMode.PRNum,
		InstallationRetriever: installationFetcher,
		PullFetcher:           pullFetcher,
		PullConverter:         pullConverter,
	}
	return &server, nil
}

// This function constructs the request we want to send to the temporal client,
// then executes the Terraform workflow. Note normally this workflow is executed
// when a request is made to the server, but we are manually executing it here,
// since we don't care about requests in adhoc mode.
func (s Server) manuallyExecuteTerraformWorkflow(repo ghClient.Repo, revision string, root terraform.Root) (interface{}, error) {
	request := workflows.TerraformRequest{
		Revision:     revision,
		WorkflowMode: terraform.Adhoc,
		Root:         root,
		Repo:         repo,
	}
	options := client.StartWorkflowOptions{
		TaskQueue: s.TerraformTaskQueue,
		SearchAttributes: map[string]interface{}{
			"atlantis_repository": repo.GetFullName(),
			"atlantis_root":       root.Name,
		},
	}

	res, err := s.TemporalClient.ExecuteWorkflow(context.Background(), options, workflows.Terraform, request)
	if err != nil {
		s.Logger.Error(err.Error())
		return nil, err
	}
	return res, nil
}

func (s Server) Start() error {
	defer s.shutdown()

	ctx := context.Background()

	for _, cron := range s.Crons {
		s.CronScheduler.Schedule(cron)
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		terraformWorker := s.buildTerraformWorker()
		if err := terraformWorker.Run(worker.InterruptCh()); err != nil {
			log.Fatalln("unable to start terraform worker", err)
		}

		s.Logger.InfoContext(ctx, "Shutting down terraform worker, resource clean up may still be occurring in the background")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		adhocExecutionParams, err := adhoc.ConstructAdhocExecParams(ctx, s.Repo, s.PRNum, s.PullFetcher, s.PullConverter, s.InstallationRetriever, s.RootConfigBuilder)
		if err != nil {
			s.Logger.Error(err.Error())
			return
		}

		if len(adhocExecutionParams.TerraformRoots) == 0 {
			s.Logger.Info("no roots found")
		}

		for _, root := range adhocExecutionParams.TerraformRoots {
			s.Logger.Info("running terraform workflow for root " + root.Name)
			_, err := s.manuallyExecuteTerraformWorkflow(adhocExecutionParams.GithubRepo, adhocExecutionParams.Revision, root)
			if err != nil {
				s.Logger.Error(err.Error())
			}
		}
	}()

	// Ensure server gracefully drains connections when stopped.
	stop := make(chan os.Signal, 1)
	// Stop on SIGINTs and SIGTERMs.
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	s.Logger.Info(fmt.Sprintf("Atlantis started - listening on port %v", s.Port))

	go func() {
		err := s.HTTPServerProxy.ListenAndServe()

		if err != nil && err != http.ErrServerClosed {
			s.Logger.Error(err.Error())
		}
	}()

	<-stop
	wg.Wait()

	return nil
}

func (s Server) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.HTTPServerProxy.Shutdown(ctx); err != nil {
		s.Logger.Error(err.Error())
	}

	s.TemporalClient.Close()

	// flush stats before shutdown
	if err := s.StatsCloser.Close(); err != nil {
		s.Logger.Error(err.Error())
	}

	s.CronScheduler.Shutdown(5 * time.Second)
	s.Logger.Close()
}

// Note that we will need to do things similar to how gateway does it to get the metadata we need
// specifically the root

func (s Server) buildTerraformWorker() worker.Worker {
	// pass the underlying client otherwise this will panic()
	terraformWorker := worker.New(s.TemporalClient.Client, s.TerraformTaskQueue, worker.Options{
		Interceptors: []interceptor.WorkerInterceptor{
			temporal.NewWorkerInterceptor(),
		},
		MaxConcurrentActivityExecutionSize: 30,
	})
	terraformWorker.RegisterActivity(s.TerraformActivities)
	terraformWorker.RegisterActivity(s.GithubActivities)
	terraformWorker.RegisterWorkflow(workflows.Terraform)
	return terraformWorker
}

// TODO: eventually we can make it so the pod is ready when the repo is done cloning...

// Healthz returns the health check response. It always returns a 200 currently.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	data, err := json.MarshalIndent(&struct {
		Status string `json:"status"`
	}{
		Status: "ok",
	}, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error creating status json response: %s", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data) // nolint: errcheck
}
