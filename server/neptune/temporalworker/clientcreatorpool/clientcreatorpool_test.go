package clientcreatorpool

import (
	"testing"

	"github.com/palantir/go-githubapp/githubapp"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/metrics"
	"github.com/stretchr/testify/assert"
)

func initialize(t *testing.T) ClientCreatorPool {
	configs := []ClientCreatorPoolConfig{
		{
			id:     564754,
			config: githubapp.Config{},
		},
		{
			id:     243643,
			config: githubapp.Config{},
		},
	}

	configs[0].config.App.IntegrationID = 564754
	configs[0].config.App.PrivateKey = "key1"
	configs[0].config.App.WebhookSecret = "secret1"

	configs[1].config.App.IntegrationID = 243643
	configs[1].config.App.PrivateKey = "key2"
	configs[1].config.App.WebhookSecret = "secret2"

	c := ClientCreatorPool{}
	ctxLogger := logging.NewNoopCtxLogger(t)
	scope, _, _ := metrics.NewLoggingScope(ctxLogger, "null")
	c.Initialize(configs, scope)
	return c
}

func TestInitialize(t *testing.T) {
	configs := []ClientCreatorPoolConfig{
		{
			id:     1,
			config: githubapp.Config{},
		},
		{
			id:     2,
			config: githubapp.Config{},
		},
	}

	configs[0].config.App.IntegrationID = 1
	configs[0].config.App.PrivateKey = "key1"
	configs[0].config.App.WebhookSecret = "secret1"

	configs[1].config.App.IntegrationID = 2
	configs[1].config.App.PrivateKey = "key2"
	configs[1].config.App.WebhookSecret = "secret2"

	c := ClientCreatorPool{}
	ctxLogger := logging.NewNoopCtxLogger(t)
	scope, _, _ := metrics.NewLoggingScope(ctxLogger, "null")
	err := c.Initialize(configs, scope)
	assert.NoError(t, err)
}

func TestGetClientCreatorWithMaxRemainingRateLimit(t *testing.T) {
	c := initialize(t)
	c.SetRateLimitRemaining(564754, 9000)
	clientCreator, err := c.GetClientCreatorWithMaxRemainingRateLimit()
	assert.NoError(t, err)
	assert.NotNil(t, clientCreator)
	assert.Equal(t, 9000, c.GetRateLimitRemaining(564754))

}
