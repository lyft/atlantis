package sqs

import (
	"bufio"
	"bytes"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/pkg/errors"
	events_controller "github.com/runatlantis/atlantis/server/controllers/events"
	"github.com/uber-go/tally"
	"net/http"
)

// VCSMessage is our sqs message containing data parsed from VCS event
// to run either autoplan or comment commands in the worker.
type VCSMessage struct {
	Writer http.ResponseWriter
	Req    *http.Request
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_sqs_message_handler.go MessageProcessor
type MessageProcessor interface {
	ProcessMessage(types.Message) error
}

type VCSEventMessageProcessor struct {
	PostHandler events_controller.VCSPostHandler
}

func (p *VCSEventMessageProcessor) ProcessMessage(msg types.Message) error {
	if msg.Body == nil {
		return errors.New("message received from sqs has no body")
	}

	buffer := bytes.NewBufferString(*msg.Body)
	buf := bufio.NewReader(buffer)
	req, err := http.ReadRequest(buf)
	if err != nil {
		return errors.Wrap(err, "reading bytes from sqs into http request")
	}

	// TODO: send a processing message back to VCS (can't stay nil), might need to implement a decorator around Post
	p.PostHandler.Post(nil, req)
	return nil
}

type VCSEventMessageProcessorStats struct {
	Scope tally.Scope
	VCSEventMessageProcessor
}

func (s *VCSEventMessageProcessorStats) ProcessMessage(msg types.Message) error {
	successCount := s.Scope.Counter(Success)
	errorCount := s.Scope.Counter(Error)

	if err := s.VCSEventMessageProcessor.ProcessMessage(msg); err != nil {
		errorCount.Inc(1)
		return err
	}
	successCount.Inc(1)
	return nil
}
