// Package locksweeper clears Terraform state locks that were orphaned when an Atlantis worker was
// killed mid-run. It only ever touches the exact lock objects Terraform itself creates (the S3
// "<root>/terraform.tfstate.tflock" object and the DynamoDB lock item), never the state.
package locksweeper

import (
	"regexp"
	"time"
)

type Store string

const (
	StoreS3       Store = "s3"
	StoreDynamoDB Store = "dynamodb"

	// PlanOperation is the only operation whose orphaned lock is deleted automatically. Terraform
	// writes "OperationTypePlan" for a plan and "OperationTypeApply" for an apply.
	PlanOperation = "OperationTypePlan"

	// TerraformWorkflowType is the type of the per-revision Terraform child workflow (and the PR
	// plan workflow). The long-lived Deploy workflow is deliberately not counted as "busy".
	TerraformWorkflowType = "Terraform"
)

// rootPattern restricts roots to a single directory name so a malformed key can never widen the
// set of objects touched.
var rootPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type Config struct {
	Bucket string
	// KeyPrefix is the state key prefix, e.g. "lyft/dsterraform".
	KeyPrefix string
	// Repository is the "owner/repo" whose roots live under KeyPrefix. It scopes the Temporal check.
	Repository string
	// Table is the DynamoDB lock table.
	Table string
	// OwnerPrefix is the prefix of the lock's Who field for locks Atlantis took, e.g.
	// "svc-user@worker-host". Only those are ever deleted; a lock held by anyone else
	// (a person running Terraform locally, for example) is alerted on instead. Empty disables the check.
	OwnerPrefix string
	// MinAge is how old a lock must be before it is considered orphaned. It must exceed the longest
	// a Terraform activity can run.
	MinAge time.Duration
	// DryRun reports what would happen without deleting anything.
	DryRun bool
}

// Lock is one lock found in one store.
type Lock struct {
	Store Store
	Root  string
	// Key is the S3 object key or the DynamoDB LockID.
	Key string
	// Fingerprint identifies this exact lock instance (S3 ETag, or the DynamoDB Info attribute) so a
	// delete never removes a lock that was replaced after it was listed.
	Fingerprint string
	ID          string
	Operation   string
	Who         string
	Created     time.Time
}

// lockInfo is the JSON Terraform writes as the lock body.
type lockInfo struct {
	ID        string
	Operation string
	Who       string
	Created   time.Time
}
