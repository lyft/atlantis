package activities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
)

type DeploymentInfo struct {
	ID         string
	CheckRunID int64
	Revision   string
	Root       root.Root
}

// Downloader is implemented by manager.Downloader
type s3Client interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

type dbActivities struct {
	S3Client   s3Client
	BucketName string
}

type FetchLatestDeploymentRequest struct {
	RepositoryName string
	RootName       string
}

type FetchLatestDeploymentResponse struct {
	DeploymentInfo DeploymentInfo
}

func (a *dbActivities) FetchLatestDeployment(ctx context.Context, request FetchLatestDeploymentRequest) (FetchLatestDeploymentResponse, error) {
	logger.Info(ctx, "fetching latest deployment")

	return FetchLatestDeploymentResponse{
		DeploymentInfo: DeploymentInfo{},
	}, nil
}

type StoreLatestDeploymentRequest struct {
	DeploymentInfo DeploymentInfo
	RepoName       string
}

func (a *dbActivities) StoreLatestDeployment(ctx context.Context, request StoreLatestDeploymentRequest) error {
	logger.Info(ctx, "storing latest deployment")
	object, err := json.Marshal(request.DeploymentInfo)
	if err != nil {
		return errors.Wrap(err, "marshalling deployment object")
	}

	key := fmt.Sprintf("deployments/%s/%s/deployment.json", request.RepoName, request.DeploymentInfo.Root.Name)
	uploadInput := &s3.PutObjectInput{
		Body:        bytes.NewReader(object),
		Bucket:      &a.BucketName,
		Key:         aws.String(key),
		ContentType: aws.String("application/json"),
	}
	_, err = a.S3Client.PutObject(ctx, uploadInput)
	if err != nil {
		return errors.Wrapf(err, "uploading deployment info for %s", request.DeploymentInfo.ID)
	}

	return nil
}
