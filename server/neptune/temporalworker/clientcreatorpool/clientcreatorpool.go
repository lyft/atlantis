package clientcreatorpool

import (
	"fmt"

	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	ghClient "github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/uber-go/tally/v4"
)

type ClientCreatorPool struct {
	// keys are app ids aka integration ids
	// Note: It would make a lot more sense to use something like a name, or the slug, right?
	// The reason why that is not the case, is that then we would have to pass those around. We can't
	// modify the clientCreator (it is part of the githubapp library), and you can see that inside, its
	// fields are private, and there is no way to associate a given clientCreator with a name. There is
	// no slug inside the githubapp.Config, only private keys and app ids, and we don't want to use
	// private keys as keys.
	idToClientCreator     map[int]githubapp.ClientCreator
	idToRateLimitRemaning map[int]int
	// Note that integration id is NOT installation id. Those are 2 separate things.
}

type ClientCreatorPoolConfig struct {
	id     int
	config githubapp.Config
}

// This function is only called once, when the server starts
func (c *ClientCreatorPool) Initialize(configs []ClientCreatorPoolConfig, scope tally.Scope) error {
	c.idToClientCreator = make(map[int]githubapp.ClientCreator)
	c.idToRateLimitRemaning = make(map[int]int)

	for _, config := range configs {
		t := fmt.Sprintf("github.app.%d", config.id)
		clientCreator, err := githubapp.NewDefaultCachingClientCreator(
			config.config,
			githubapp.WithClientMiddleware(
				// combine the id with app
				ghClient.ClientMetrics(scope.SubScope(t)),
			))
		if err != nil {
			return errors.Wrap(err, "client creator")
		}
		c.idToClientCreator[config.id] = clientCreator
		// just needs to be non-zero, true value will be updated within 60 seconds by the cron that checks the rate limit
		c.idToRateLimitRemaning[config.id] = 100
	}

	return nil
}

func (c *ClientCreatorPool) GetClientCreatorWithMaxRemainingRateLimit() (githubapp.ClientCreator, error) {
	maxSeenSoFar := 0
	theId := 0
	for id, num := range c.idToRateLimitRemaning {
		if num > maxSeenSoFar {
			maxSeenSoFar = num
			theId = id
		}
	}

	clientCreator, ok := c.idToClientCreator[theId]
	if !ok {
		return nil, errors.New("client creator not found")
	}

	return clientCreator, nil
}

// this func will be used in the crons to update the rate limit remaining
func (c *ClientCreatorPool) SetRateLimitRemaining(id int, remaining int) {
	c.idToRateLimitRemaning[id] = remaining
}
