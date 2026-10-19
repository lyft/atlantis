package locksweeper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"
	"go.temporal.io/api/workflowservice/v1"
)

const (
	s3LockSuffix       = "terraform.tfstate.tflock"
	dynamoLockSuffix   = "terraform.tfstate"
	dynamoInfoAttr     = "Info"
	dynamoLockIDAttr   = "LockID"
	conditionalFailed  = "ConditionalCheckFailedException"
	s3NotFoundCode     = "NotFound"
	s3NoSuchKeyCode    = "NoSuchKey"
	runningStatusQuery = `ExecutionStatus = "Running"`
)

type s3API interface {
	ListObjectsV2PagesWithContext(ctx aws.Context, input *s3.ListObjectsV2Input, fn func(*s3.ListObjectsV2Output, bool) bool, opts ...request.Option) error
	GetObjectWithContext(ctx aws.Context, input *s3.GetObjectInput, opts ...request.Option) (*s3.GetObjectOutput, error)
	HeadObjectWithContext(ctx aws.Context, input *s3.HeadObjectInput, opts ...request.Option) (*s3.HeadObjectOutput, error)
	DeleteObjectWithContext(ctx aws.Context, input *s3.DeleteObjectInput, opts ...request.Option) (*s3.DeleteObjectOutput, error)
}

type dynamoAPI interface {
	ScanPagesWithContext(ctx aws.Context, input *dynamodb.ScanInput, fn func(*dynamodb.ScanOutput, bool) bool, opts ...request.Option) error
	DeleteItemWithContext(ctx aws.Context, input *dynamodb.DeleteItemInput, opts ...request.Option) (*dynamodb.DeleteItemOutput, error)
}

type temporalLister interface {
	ListWorkflow(ctx context.Context, request *workflowservice.ListWorkflowExecutionsRequest) (*workflowservice.ListWorkflowExecutionsResponse, error)
}

// Activities does the I/O for the sweeper workflow.
type Activities struct {
	// Config is what the sweeper runs with. The workflow reads it through GetConfig on every run so a
	// restarted worker with new config takes effect on the next sweep.
	Config    Config
	S3        s3API
	Dynamo    dynamoAPI
	Temporal  temporalLister
	Namespace string
}

// GetConfig returns the worker's current sweeper config.
func (a *Activities) GetConfig(_ context.Context) (Config, error) {
	return a.Config, nil
}

type ListLocksRequest struct {
	Bucket    string
	KeyPrefix string
	Table     string
}

type ListLocksResponse struct {
	Locks []Lock
}

// ListLocks returns every lock in both stores under the configured prefix.
func (a *Activities) ListLocks(ctx context.Context, req ListLocksRequest) (ListLocksResponse, error) {
	s3Locks, err := a.listS3Locks(ctx, req)
	if err != nil {
		return ListLocksResponse{}, errors.Wrap(err, "listing S3 locks")
	}
	dynamoLocks, err := a.listDynamoLocks(ctx, req)
	if err != nil {
		return ListLocksResponse{}, errors.Wrap(err, "listing DynamoDB locks")
	}
	return ListLocksResponse{Locks: append(s3Locks, dynamoLocks...)}, nil
}

func (a *Activities) listS3Locks(ctx context.Context, req ListLocksRequest) ([]Lock, error) {
	prefix := strings.TrimSuffix(req.KeyPrefix, "/") + "/"
	var locks []Lock
	var getErr error

	err := a.S3.ListObjectsV2PagesWithContext(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(req.Bucket),
		Prefix: aws.String(prefix),
	}, func(page *s3.ListObjectsV2Output, _ bool) bool {
		for _, obj := range page.Contents {
			root, ok := rootFromS3Key(prefix, aws.StringValue(obj.Key))
			if !ok {
				continue
			}
			lock := Lock{
				Store:       StoreS3,
				Root:        root,
				Key:         aws.StringValue(obj.Key),
				Fingerprint: aws.StringValue(obj.ETag),
			}
			info, err := a.readS3LockInfo(ctx, req.Bucket, lock.Key)
			if err != nil {
				getErr = err
				return false
			}
			applyInfo(&lock, info)
			locks = append(locks, lock)
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	if getErr != nil {
		return nil, getErr
	}
	return locks, nil
}

// rootFromS3Key returns the root for exactly "<prefix><root>/terraform.tfstate.tflock".
func rootFromS3Key(prefix, key string) (string, bool) {
	rest := strings.TrimPrefix(key, prefix)
	if rest == key {
		return "", false
	}
	root, name, found := strings.Cut(rest, "/")
	if !found || name != s3LockSuffix || !rootPattern.MatchString(root) || strings.Contains(root, "..") {
		return "", false
	}
	return root, true
}

func (a *Activities) readS3LockInfo(ctx context.Context, bucket, key string) (lockInfo, error) {
	out, err := a.S3.GetObjectWithContext(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		if isNotFound(err) {
			// released between listing and reading
			return lockInfo{}, nil
		}
		return lockInfo{}, errors.Wrapf(err, "reading %s", key)
	}
	defer out.Body.Close()
	body, err := io.ReadAll(out.Body)
	if err != nil {
		return lockInfo{}, errors.Wrapf(err, "reading body of %s", key)
	}
	return parseLockInfo(string(body)), nil
}

func (a *Activities) listDynamoLocks(ctx context.Context, req ListLocksRequest) ([]Lock, error) {
	idPrefix := req.Bucket + "/" + strings.TrimSuffix(req.KeyPrefix, "/") + "/"
	var locks []Lock

	err := a.Dynamo.ScanPagesWithContext(ctx, &dynamodb.ScanInput{
		TableName:            aws.String(req.Table),
		FilterExpression:     aws.String("attribute_exists(" + dynamoInfoAttr + ")"),
		ProjectionExpression: aws.String(dynamoLockIDAttr + ", " + dynamoInfoAttr),
	}, func(page *dynamodb.ScanOutput, _ bool) bool {
		for _, item := range page.Items {
			lockID := aws.StringValue(item[dynamoLockIDAttr].S)
			root, ok := rootFromDynamoLockID(idPrefix, lockID)
			if !ok {
				continue
			}
			infoStr := aws.StringValue(item[dynamoInfoAttr].S)
			lock := Lock{Store: StoreDynamoDB, Root: root, Key: lockID, Fingerprint: infoStr}
			applyInfo(&lock, parseLockInfo(infoStr))
			locks = append(locks, lock)
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return locks, nil
}

// rootFromDynamoLockID returns the root for exactly "<idPrefix><root>/terraform.tfstate".
func rootFromDynamoLockID(idPrefix, lockID string) (string, bool) {
	rest := strings.TrimPrefix(lockID, idPrefix)
	if rest == lockID {
		return "", false
	}
	root, name, found := strings.Cut(rest, "/")
	if !found || name != dynamoLockSuffix || !rootPattern.MatchString(root) || strings.Contains(root, "..") {
		return "", false
	}
	return root, true
}

func parseLockInfo(body string) lockInfo {
	var info lockInfo
	// An unreadable body leaves Created zero, which Decide treats as "age unknown" and skips.
	_ = json.Unmarshal([]byte(body), &info)
	return info
}

func applyInfo(l *Lock, info lockInfo) {
	l.ID = info.ID
	l.Operation = info.Operation
	l.Who = info.Who
	l.Created = info.Created
}

type RootBusyRequest struct {
	Repository string
	Root       string
}

type RootBusyResponse struct {
	Busy bool
}

// RootBusy reports whether Temporal has an open Terraform workflow for the root. Any error is
// returned so the caller fails safe and skips the lock rather than assuming the root is idle.
func (a *Activities) RootBusy(ctx context.Context, req RootBusyRequest) (RootBusyResponse, error) {
	if !rootPattern.MatchString(req.Root) || strings.ContainsAny(req.Repository, `"\`) {
		return RootBusyResponse{}, fmt.Errorf("refusing to query for root %q in repository %q", req.Root, req.Repository)
	}
	query := fmt.Sprintf(`WorkflowType = "%s" AND atlantis_repository = "%s" AND atlantis_root = "%s" AND %s`,
		TerraformWorkflowType, req.Repository, req.Root, runningStatusQuery)

	resp, err := a.Temporal.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
		Namespace: a.Namespace,
		PageSize:  1,
		Query:     query,
	})
	if err != nil {
		return RootBusyResponse{}, errors.Wrap(err, "listing open Terraform workflows")
	}
	return RootBusyResponse{Busy: len(resp.Executions) > 0}, nil
}

type DeleteLockRequest struct {
	Bucket string
	Table  string
	Lock   Lock
}

type DeleteLockResponse struct {
	// Deleted is false when the lock changed or was already released since it was listed.
	Deleted bool
}

// DeleteLock removes exactly the lock that was listed, and nothing if it has since changed.
func (a *Activities) DeleteLock(ctx context.Context, req DeleteLockRequest) (DeleteLockResponse, error) {
	switch req.Lock.Store {
	case StoreS3:
		return a.deleteS3Lock(ctx, req)
	case StoreDynamoDB:
		return a.deleteDynamoLock(ctx, req)
	default:
		return DeleteLockResponse{}, fmt.Errorf("unknown lock store %q", req.Lock.Store)
	}
}

func (a *Activities) deleteS3Lock(ctx context.Context, req DeleteLockRequest) (DeleteLockResponse, error) {
	if !strings.HasSuffix(req.Lock.Key, "/"+s3LockSuffix) {
		return DeleteLockResponse{}, fmt.Errorf("refusing to delete %q: not a lock object", req.Lock.Key)
	}
	// The v1 SDK has no conditional delete, so re-check the ETag immediately before deleting. A lock
	// replaced inside that window would still be removed; the window is a single request.
	head, err := a.S3.HeadObjectWithContext(ctx, &s3.HeadObjectInput{Bucket: aws.String(req.Bucket), Key: aws.String(req.Lock.Key)})
	if err != nil {
		if isNotFound(err) {
			return DeleteLockResponse{}, nil
		}
		return DeleteLockResponse{}, errors.Wrapf(err, "checking %s", req.Lock.Key)
	}
	if aws.StringValue(head.ETag) != req.Lock.Fingerprint {
		return DeleteLockResponse{}, nil
	}
	if _, err := a.S3.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{Bucket: aws.String(req.Bucket), Key: aws.String(req.Lock.Key)}); err != nil {
		return DeleteLockResponse{}, errors.Wrapf(err, "deleting %s", req.Lock.Key)
	}
	return DeleteLockResponse{Deleted: true}, nil
}

func (a *Activities) deleteDynamoLock(ctx context.Context, req DeleteLockRequest) (DeleteLockResponse, error) {
	if !strings.HasSuffix(req.Lock.Key, "/"+dynamoLockSuffix) {
		return DeleteLockResponse{}, fmt.Errorf("refusing to delete %q: not a lock item", req.Lock.Key)
	}
	_, err := a.Dynamo.DeleteItemWithContext(ctx, &dynamodb.DeleteItemInput{
		TableName:           aws.String(req.Table),
		Key:                 map[string]*dynamodb.AttributeValue{dynamoLockIDAttr: {S: aws.String(req.Lock.Key)}},
		ConditionExpression: aws.String(dynamoInfoAttr + " = :info"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":info": {S: aws.String(req.Lock.Fingerprint)},
		},
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == conditionalFailed {
			return DeleteLockResponse{}, nil
		}
		return DeleteLockResponse{}, errors.Wrapf(err, "deleting %s", req.Lock.Key)
	}
	return DeleteLockResponse{Deleted: true}, nil
}

func isNotFound(err error) bool {
	aerr, ok := err.(awserr.Error)
	if !ok {
		return false
	}
	return aerr.Code() == s3NotFoundCode || aerr.Code() == s3NoSuchKeyCode
}
