package sns

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	awsSns "github.com/aws/aws-sdk-go/service/sns"
	snsApi "github.com/aws/aws-sdk-go/service/sns/snsiface"
)

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_writer.go Writer

type Writer interface {
	// Write a message to an SNS topic with the specified string payload
	Write([]byte) error
}

// NewWriter returns a new instance of Writer that will connect to the specifed
// sns topic using the specified session
func NewWriter(session *session.Session, topicArn string) Writer {
	return &writer{
		client:   awsSns.New(session),
		topicArn: aws.String(topicArn),
	}
}

type writer struct {
	client   snsApi.SNSAPI
	topicArn *string
}

func (w *writer) Write(payload []byte) error {
	_, err := w.client.Publish(&awsSns.PublishInput{
		Message:  aws.String(string(payload)),
		TopicArn: w.topicArn,
	})
	return err
}
