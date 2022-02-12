package sqs_test

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/mocks"
	"github.com/runatlantis/atlantis/server/events/mocks/matchers"
	"github.com/runatlantis/atlantis/server/lyft/aws/sqs"
	. "github.com/runatlantis/atlantis/testing"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"

	"encoding/json"
	"testing"
)

func TestAtlantisMessageHandler_AutoSuccess(t *testing.T) {
	RegisterMockTestingT(t)
	testScope := tally.NewTestScope("test", nil)
	message := map[string]interface{}{
		"trigger": events.Auto,
	}
	commandRunner := mocks.NewMockCommandRunner()
	handler := &sqs.MessageHandler{
		CommandRunner: commandRunner,
		Scope:         testScope,
		TestingMode:   true,
	}

	err := handler.ProcessMessage(toSqsMessage(t, message))
	assert.NoError(t, err)
	commandRunner.VerifyWasCalledOnce().RunAutoplanCommand(
		matchers.AnyModelsRepo(),
		matchers.AnyModelsRepo(),
		matchers.AnyModelsPullRequest(),
		matchers.AnyModelsUser(),
		matchers.AnyTimeTime())
	Assert(t, testScope.Snapshot().Counters()["test.success+"].Value() == 1, "message handler was successful")
}

func TestAtlantisMessageHandler_CommentSuccess(t *testing.T) {
	RegisterMockTestingT(t)
	testScope := tally.NewTestScope("test", nil)
	message := map[string]interface{}{
		"trigger": events.Comment,
	}
	commandRunner := mocks.NewMockCommandRunner()
	handler := &sqs.MessageHandler{
		CommandRunner: commandRunner,
		Scope:         testScope,
		TestingMode:   true,
	}

	err := handler.ProcessMessage(toSqsMessage(t, message))
	assert.NoError(t, err)
	commandRunner.VerifyWasCalledOnce().RunCommentCommand(
		matchers.AnyModelsRepo(),
		matchers.AnyPtrToModelsRepo(),
		matchers.AnyPtrToModelsPullRequest(),
		matchers.AnyModelsUser(),
		EqInt(0),
		matchers.AnyPtrToEventsCommentCommand(),
		matchers.AnyTimeTime())
	Assert(t, testScope.Snapshot().Counters()["test.success+"].Value() == 1, "message handler was successful")
}

func TestAtlantisMessageHandler_Error(t *testing.T) {
	RegisterMockTestingT(t)
	testScope := tally.NewTestScope("test", nil)
	commandRunner := mocks.NewMockCommandRunner()
	handler := &sqs.MessageHandler{
		CommandRunner: commandRunner,
		Scope:         testScope,
		TestingMode:   true,
	}

	message := map[string]interface{}{
		"trigger": events.Auto,
	}
	msgBytes, err := json.Marshal(message)
	assert.NoError(t, err)
	//remove some bytes from the message to make it unmarshallable
	invalidMessage := types.Message{
		Body: aws.String(string(msgBytes[0 : len(msgBytes)-1])),
	}

	err = handler.ProcessMessage(invalidMessage)
	assert.Error(t, err)
	commandRunner.VerifyWasCalled(Never()).RunAutoplanCommand(
		matchers.AnyModelsRepo(),
		matchers.AnyModelsRepo(),
		matchers.AnyModelsPullRequest(),
		matchers.AnyModelsUser(),
		matchers.AnyTimeTime())
	Assert(t, testScope.Snapshot().Counters()["test.error+"].Value() == 1, "message handler was not successful")
}

func toSqsMessage(t *testing.T, msg map[string]interface{}) types.Message {
	msgBytes, err := json.Marshal(msg)
	assert.NoError(t, err)

	return types.Message{
		Body: aws.String(string(msgBytes)),
	}
}
