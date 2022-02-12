package sqs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/uber-go/tally"
	"sync"
)

const (
	ProcessMessageMetricName = "process_msg"
	ReceiveMessageMetricName = "receive_msg"
	DeleteMessageMetricName  = "delete_msg"

	Latency = "latency"
	Success = "success"
	Error   = "error"
)

type Worker struct {
	Queue    Queue
	QueueURL string
	Handler  MessageProcessor
	Scope    tally.Scope
}

// TODO: initialize SQS worker in server.go upon creation of worker/hybrid modes

func NewGatewaySQSWorker(scope tally.Scope, queueURL string, commandRunner events.CommandRunner) (*Worker, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}
	sqsQueueWrapper := &QueueWithStats{
		Queue:    sqs.NewFromConfig(cfg),
		Scope:    scope,
		QueueURL: queueURL,
	}

	handler := &MessageHandler{
		CommandRunner: commandRunner,
		Scope:         scope.SubScope(ProcessMessageMetricName),
		TestingMode:   false,
	}

	return &Worker{
		Queue:    sqsQueueWrapper,
		QueueURL: queueURL,
		Handler:  handler,
		Scope:    scope,
	}, nil
}

func (w *Worker) Work(ctx context.Context) {
	messages := make(chan types.Message)
	// Used to synchronize stopping message retrivial and processing
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.processMessage(ctx, messages)
	}()
	request := &sqs.ReceiveMessageInput{
		QueueUrl:            &w.QueueURL,
		MaxNumberOfMessages: 10, //max number of batch-able messages
		WaitTimeSeconds:     20, //max duration long polling
	}
	w.receiveMessages(ctx, messages, request)
	wg.Wait()
}

func (w *Worker) receiveMessages(ctx context.Context, messages chan types.Message, request *sqs.ReceiveMessageInput) {
	for {
		select {
		case <-ctx.Done():
			close(messages)
			return
		default:
			response, err := w.Queue.ReceiveMessage(ctx, request)
			if err != nil {
				continue
			}
			for _, message := range response.Messages {
				messages <- message
			}
		}
	}
}

func (w *Worker) processMessage(ctx context.Context, messages chan types.Message) {
	// VisibilityTimeout is 30s, ideally enough time to "processMessage" < 10 messages (i.e. spin up goroutine for each)
	for message := range messages {
		err := w.Handler.ProcessMessage(message)
		if err != nil {
			continue
		}

		// Since we've successfully processed the message, let's go ahead and delete it from the queue
		_, err = w.Queue.DeleteMessage(ctx, &sqs.DeleteMessageInput{
			QueueUrl:      &w.QueueURL,
			ReceiptHandle: message.ReceiptHandle,
		})
	}
}
