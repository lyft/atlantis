package locksweeper

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
)

func TestRootFromS3Key(t *testing.T) {
	const prefix = "lyft/dsterraform/"
	cases := []struct {
		key  string
		root string
		ok   bool
	}{
		{"lyft/dsterraform/dsterraform-spots-production/terraform.tfstate.tflock", "dsterraform-spots-production", true},
		{"lyft/dsterraform/dsterraform-spots-production/terraform.tfstate", "", false},
		{"lyft/dsterraform/a/b/terraform.tfstate.tflock", "", false},
		{"lyft/dsterraform/../terraform.tfstate.tflock", "", false},
		{"other/prefix/x/terraform.tfstate.tflock", "", false},
		{"lyft/dsterraform/terraform.tfstate.tflock", "", false},
	}
	for _, tc := range cases {
		root, ok := rootFromS3Key(prefix, tc.key)
		assert.Equal(t, tc.ok, ok, tc.key)
		assert.Equal(t, tc.root, root, tc.key)
	}
}

func TestRootFromDynamoLockID(t *testing.T) {
	const prefix = "bucket/lyft/dsterraform/"
	root, ok := rootFromDynamoLockID(prefix, "bucket/lyft/dsterraform/dsterraform-spots-production/terraform.tfstate")
	assert.True(t, ok)
	assert.Equal(t, "dsterraform-spots-production", root)

	// the digest item Terraform keeps next to the lock must never be treated as a lock
	_, ok = rootFromDynamoLockID(prefix, "bucket/lyft/dsterraform/dsterraform-spots-production/terraform.tfstate-md5")
	assert.False(t, ok)
	_, ok = rootFromDynamoLockID(prefix, "bucket/lyft/dsterraform/x/y/terraform.tfstate")
	assert.False(t, ok)
}

type fakeS3 struct {
	s3API
	etag    string
	headErr error
	deleted []string
}

func (f *fakeS3) HeadObjectWithContext(_ aws.Context, _ *s3.HeadObjectInput, _ ...request.Option) (*s3.HeadObjectOutput, error) {
	if f.headErr != nil {
		return nil, f.headErr
	}
	return &s3.HeadObjectOutput{ETag: aws.String(f.etag)}, nil
}

func (f *fakeS3) DeleteObjectWithContext(_ aws.Context, in *s3.DeleteObjectInput, _ ...request.Option) (*s3.DeleteObjectOutput, error) {
	f.deleted = append(f.deleted, aws.StringValue(in.Key))
	return &s3.DeleteObjectOutput{}, nil
}

func TestDeleteS3Lock(t *testing.T) {
	const key = "lyft/dsterraform/r/terraform.tfstate.tflock"
	req := DeleteLockRequest{Bucket: "b", Lock: Lock{Store: StoreS3, Key: key, Fingerprint: `"abc"`}}

	t.Run("deletes when the ETag still matches", func(t *testing.T) {
		f := &fakeS3{etag: `"abc"`}
		resp, err := (&Activities{S3: f}).DeleteLock(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, resp.Deleted)
		assert.Equal(t, []string{key}, f.deleted)
	})

	t.Run("does not delete a lock that was replaced", func(t *testing.T) {
		f := &fakeS3{etag: `"different"`}
		resp, err := (&Activities{S3: f}).DeleteLock(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Deleted)
		assert.Empty(t, f.deleted)
	})

	t.Run("treats an already released lock as not deleted", func(t *testing.T) {
		f := &fakeS3{headErr: awserr.New("NotFound", "gone", nil)}
		resp, err := (&Activities{S3: f}).DeleteLock(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Deleted)
		assert.Empty(t, f.deleted)
	})

	t.Run("refuses a key that is not a lock object", func(t *testing.T) {
		f := &fakeS3{etag: `"abc"`}
		bad := req
		bad.Lock.Key = "lyft/dsterraform/r/terraform.tfstate"
		_, err := (&Activities{S3: f}).DeleteLock(context.Background(), bad)
		require.Error(t, err)
		assert.Empty(t, f.deleted)
	})
}

type fakeDynamo struct {
	dynamoAPI
	err error
	in  *dynamodb.DeleteItemInput
}

func (f *fakeDynamo) DeleteItemWithContext(_ aws.Context, in *dynamodb.DeleteItemInput, _ ...request.Option) (*dynamodb.DeleteItemOutput, error) {
	f.in = in
	return &dynamodb.DeleteItemOutput{}, f.err
}

func TestDeleteDynamoLock(t *testing.T) {
	const id = "b/lyft/dsterraform/r/terraform.tfstate"
	req := DeleteLockRequest{Table: "t", Lock: Lock{Store: StoreDynamoDB, Key: id, Fingerprint: `{"ID":"1"}`}}

	t.Run("deletes conditionally on the Info attribute", func(t *testing.T) {
		f := &fakeDynamo{}
		resp, err := (&Activities{Dynamo: f}).DeleteLock(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, resp.Deleted)
		assert.Equal(t, "Info = :info", aws.StringValue(f.in.ConditionExpression))
		assert.Equal(t, `{"ID":"1"}`, aws.StringValue(f.in.ExpressionAttributeValues[":info"].S))
		assert.Equal(t, id, aws.StringValue(f.in.Key["LockID"].S))
	})

	t.Run("a failed condition means the lock changed, not an error", func(t *testing.T) {
		f := &fakeDynamo{err: awserr.New("ConditionalCheckFailedException", "changed", nil)}
		resp, err := (&Activities{Dynamo: f}).DeleteLock(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Deleted)
	})

	t.Run("refuses the md5 digest item", func(t *testing.T) {
		f := &fakeDynamo{}
		bad := req
		bad.Lock.Key = id + "-md5"
		_, err := (&Activities{Dynamo: f}).DeleteLock(context.Background(), bad)
		require.Error(t, err)
		assert.Nil(t, f.in)
	})
}

type fakeTemporal struct {
	executions int
	err        error
	query      string
}

func (f *fakeTemporal) ListWorkflow(_ context.Context, req *workflowservice.ListWorkflowExecutionsRequest) (*workflowservice.ListWorkflowExecutionsResponse, error) {
	f.query = req.Query
	resp := &workflowservice.ListWorkflowExecutionsResponse{}
	for i := 0; i < f.executions; i++ {
		resp.Executions = append(resp.Executions, &workflow.WorkflowExecutionInfo{})
	}
	return resp, f.err
}

func TestRootBusy(t *testing.T) {
	t.Run("busy when a Terraform workflow is running, scoped by type, repo and root", func(t *testing.T) {
		f := &fakeTemporal{executions: 1}
		resp, err := (&Activities{Temporal: f, Namespace: "atlantis"}).RootBusy(context.Background(), RootBusyRequest{Repository: "lyft/dsterraform", Root: "r"})
		require.NoError(t, err)
		assert.True(t, resp.Busy)
		assert.Equal(t, `WorkflowType = "Terraform" AND atlantis_repository = "lyft/dsterraform" AND atlantis_root = "r" AND ExecutionStatus = "Running"`, f.query)
	})

	t.Run("idle when none are running", func(t *testing.T) {
		resp, err := (&Activities{Temporal: &fakeTemporal{}}).RootBusy(context.Background(), RootBusyRequest{Repository: "lyft/dsterraform", Root: "r"})
		require.NoError(t, err)
		assert.False(t, resp.Busy)
	})

	t.Run("a Temporal error is returned so the lock is left alone", func(t *testing.T) {
		_, err := (&Activities{Temporal: &fakeTemporal{err: assert.AnError}}).RootBusy(context.Background(), RootBusyRequest{Repository: "lyft/dsterraform", Root: "r"})
		require.Error(t, err)
	})

	t.Run("refuses a root that could inject into the query", func(t *testing.T) {
		f := &fakeTemporal{}
		_, err := (&Activities{Temporal: f}).RootBusy(context.Background(), RootBusyRequest{Repository: "lyft/dsterraform", Root: `x" OR 1=1`})
		require.Error(t, err)
		assert.Empty(t, f.query)
	})
}
