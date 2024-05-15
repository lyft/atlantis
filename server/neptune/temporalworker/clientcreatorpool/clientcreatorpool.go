package clientcreatorpool

import (
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	ghClient "github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/uber-go/tally/v4"
)

type ClientCreatorPool struct {
	NamesToClientCreators     map[string]githubapp.ClientCreator
	NamesToRateLimitRemaining map[string]int
}

type ClientCreatorPoolConfig struct {
	name   string
	config githubapp.Config
}

func (c *ClientCreatorPool) Initialize(configs []ClientCreatorPoolConfig, scope tally.Scope) error {

	c.NamesToClientCreators = make(map[string]githubapp.ClientCreator)
	c.NamesToRateLimitRemaining = make(map[string]int)

	for _, config := range configs {
		clientCreator, err := githubapp.NewDefaultCachingClientCreator(
			config.config,
			githubapp.WithClientMiddleware(
				ghClient.ClientMetrics(scope.SubScope("app"+config.name)),
			))
		if err != nil {
			return errors.Wrap(err, "client creator")
		}
		c.NamesToClientCreators[config.name] = clientCreator
		// just needs to be non-zero, true value will be updated within 60 seconds by the cron that checks the rate limit
		c.NamesToRateLimitRemaining[config.name] = 100
	}

	return nil
}

func (c *ClientCreatorPool) GetClientCreatorWithMaxRemainingRateLimit(name string) (githubapp.ClientCreator, error) {
	maxSeenSoFar := 0
	for clientName, num := range c.NamesToRateLimitRemaining {
		if num > maxSeenSoFar {
			maxSeenSoFar = num
			name = clientName
		}
	}

	clientCreator, ok := c.NamesToClientCreators[name]
	if !ok {
		return nil, errors.New("client creator not found")
	}

	return clientCreator, nil
}

// this func will be used in the crons to update the rate limit remaining
func (c *ClientCreatorPool) SetRateLimitRemaining(name string, remaining int) {
	c.NamesToRateLimitRemaining[name] = remaining
}
