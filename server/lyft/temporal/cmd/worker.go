package cmd

import (
	"log"

	"github.com/runatlantis/atlantis/server/lyft/temporal/activities"
	"github.com/runatlantis/atlantis/server/lyft/temporal/workflows"
	"github.com/spf13/cobra"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

type WorkerConfig struct {
	GhUser  string
	GhToken string
}

func NewWorkerCmd(config WorkerConfig) *cobra.Command {
	c := &cobra.Command{
		Use:           "worker",
		Short:         "Start the temporal worker",
		Long:          `Start the atlantis deployment temporal worker`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := NewWorker(config)

			if err := w.Run(worker.InterruptCh()); err != nil {
				log.Fatalln("unable to start Worker", err)
				return err
			}

			return nil
		},
	}

	c.PersistentFlags().StringVarP(&config.GhUser, "ghuser", "", "", "github user")
	c.PersistentFlags().StringVarP(&config.GhUser, "ghtoken", "", "", "github token")
	c.PersistentFlags().StringVarP(&config.GhUser, "repo-name", "", "", "repository name")
	c.PersistentFlags().StringVarP(&config.GhUser, "repo-owner", "", "", "repository owner")
	c.PersistentFlags().StringVarP(&config.GhUser, "repo-branch", "", "", "repository branch")

	return c
}

func NewWorker(config WorkerConfig) worker.Worker {
	workflowClient, err := client.NewClient(client.Options{})

	if err != nil {
		log.Fatal(err.Error())
	}

	worker := worker.New(workflowClient, workflows.TaskQueue, worker.Options{
		// ensures that sessions are preserved on the same worker
		EnableSessionWorker: true,
	})

	vcsClient := activities.NewVCSClientWrapper(config.GhUser, config.GhToken)

	worker.RegisterWorkflow(workflows.Deploy)
	worker.RegisterActivity(vcsClient.GetRepository)
	worker.RegisterActivity(activities.Clone)
	worker.RegisterActivity(activities.Init)
	worker.RegisterActivity(activities.Plan)
	worker.RegisterActivity(activities.Apply)

	return worker
}
