package sqs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/logging"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/uber-go/tally"
)

const (
	ProcessMessageMetricName = "process"
	ReceiveMessageMetricName = "receive"
	DeleteMessageMetricName  = "delete"

	Latency = "latency"
	Success = "success"
	Error   = "error"
)

type Worker struct {
	Queue            Queue
	QueueURL         string
	MessageProcessor MessageProcessor
	Logger           logging.SimpleLogging
	Context          context.Context
}

func NewGatewaySQSWorker(scope tally.Scope, logger logging.SimpleLogging, queueURL string, postHandler VCSPostHandler, ctx context.Context) (*Worker, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error loading aws config for sqs worker")
	}
	scope = scope.SubScope("aws.sqs.msg")
	sqsQueueWrapper := &QueueWithStats{
		Queue:    sqs.NewFromConfig(cfg),
		Scope:    scope,
		QueueURL: queueURL,
	}

	handler := &VCSEventMessageProcessorStats{
		VCSEventMessageProcessor: VCSEventMessageProcessor{
			PostHandler: postHandler,
		},
		Scope: scope.SubScope(ProcessMessageMetricName),
	}

	return &Worker{
		Queue:            sqsQueueWrapper,
		QueueURL:         queueURL,
		MessageProcessor: handler,
		Logger:           logger,
		Context:          ctx,
	}, nil
}

func (w *Worker) Work() {
	messages := make(chan types.Message)
	// Used to synchronize stopping message retrieval and processing
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.Logger.Info("start processing sqs messages routine")
		w.processMessage(messages)
	}()
	request := &sqs.ReceiveMessageInput{
		QueueUrl:            &w.QueueURL,
		MaxNumberOfMessages: 10, //max number of batch-able messages
		WaitTimeSeconds:     20, //max duration long polling
	}
	w.Logger.Info("start receiving sqs messages")
	w.receiveMessages(messages, request)
	wg.Wait()
}

func (w *Worker) receiveMessages(messages chan types.Message, request *sqs.ReceiveMessageInput) {
	for {
		select {
		case <-w.Context.Done():
			close(messages)
			w.Logger.Info("closed sqs messages channel")
			return
		default:
			response, err := w.Queue.ReceiveMessage(w.Context, request)
			if err != nil {
				w.Logger.With("err", err).Err("unable to receive sqs message")
				continue
			}
			for _, message := range response.Messages {
				w.Logger.Info("sending received sqs message through messages channel")
				messages <- message
			}
		}
	}
}

func (w *Worker) processMessage(messages chan types.Message) {
	// VisibilityTimeout is 30s, ideally enough time to "processMessage" < 10 messages (i.e. spin up goroutine for each)
	for message := range messages {
		err := w.MessageProcessor.ProcessMessage(message)
		if err != nil {
			w.Logger.With("err", err).Err("unable to process sqs message")
			continue
		}

		// Since we've successfully processed the message, let's go ahead and delete it from the queue
		_, err = w.Queue.DeleteMessage(w.Context, &sqs.DeleteMessageInput{
			QueueUrl:      &w.QueueURL,
			ReceiptHandle: message.ReceiptHandle,
		})
		if err != nil {
			// keep it as a warning since this is not a big deal unless this is occurring frequently since
			// we'll have already processed the message.
			w.Logger.With("err", err).Err("unable to delete processed sqs message")
		}
	}
}
