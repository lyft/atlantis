package temporal

import (
	"os"
	"log"

	"github.com/runatlantis/atlantis/server/lyft/temporal/cmd"
)

func main() {
	worker := cmd.NewWorkerCmd(cmd.WorkerConfig{})

	if err := worker.Execute(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
