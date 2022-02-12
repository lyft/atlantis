package sqs

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/uber-go/tally"
	"time"
)

// CommandMessage is our sqs message containing data parsed from VCS event
// to run either autoplan or comment commands in the worker.
type CommandMessage struct {
	BaseRepo  models.Repo           `json:"base_repo"`
	HeadRepo  models.Repo           `json:"head_repo"`
	Pull      models.PullRequest    `json:"pull"`
	User      models.User           `json:"user"`
	PullNum   int                   `json:"pull_num"`
	Timestamp int64                 `json:"timestamp"`
	Cmd       events.CommentCommand `json:"cmd"`
	Trigger   events.CommandTrigger `json:"trigger"`
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_sqs_message_handler.go MessageProcessor
type MessageProcessor interface {
	ProcessMessage(types.Message) error
}

type MessageHandler struct {
	CommandRunner events.CommandRunner
	Scope         tally.Scope
	TestingMode   bool
}

func (m *MessageHandler) ProcessMessage(msg types.Message) error {
	successCount := m.Scope.Counter(Success)
	errorCount := m.Scope.Counter(Error)

	var cm *CommandMessage
	err := json.Unmarshal([]byte(*msg.Body), &cm)
	if err != nil {
		errorCount.Inc(1)
		return fmt.Errorf("unmarshalling json to CommandMessage: %w", err)
	}

	if !m.TestingMode {
		go m.runCommand(cm)
	} else {
		m.runCommand(cm)
	}
	successCount.Inc(1)
	// TODO: send a processing message back to VCS
	return nil
}

func (m *MessageHandler) runCommand(cm *CommandMessage) {
	if cm.Trigger == events.Auto {
		m.CommandRunner.RunAutoplanCommand(cm.BaseRepo, cm.HeadRepo, cm.Pull, cm.User, time.Unix(cm.Timestamp, 0))
	} else {
		m.CommandRunner.RunCommentCommand(cm.BaseRepo, &cm.HeadRepo, &cm.Pull, cm.User, cm.PullNum, &cm.Cmd, time.Unix(cm.Timestamp, 0))
	}
}
