package metrics

const (
	ExecutionTimeMetric    = "execution_time"
	ExecutionSuccessMetric = "execution_success"
	ExecutionErrorMetric   = "execution_error"
	ExecutionFailureMetric = "execution_failure"

	FilterPresentMetric = "present"
	FilterAbsentMetric  = "absent"
	FilterErrorMetric   = "error"

	RootTag     = "root"
	RepoTag     = "repo"
	RevisionTag = "revision"

	ActivityExecutionSuccess = "activity_execution_success"
	ActivityExecutionFailure = "activity_execution_failure"

	// Note: This is specifically calculated when the activity starts (not scheduled)
	ActivityExecutionLatency = "activity_execution_latency"

	SignalNameTag = "signal_name"
	PollNameTag   = "poll_name"

	// Signal handling metrics before it is added to a buffered channel
	SignalHandleSuccess = "signal_handle_success"
	SignalHandleFailure = "signal_handle_failure"
	SignalHandleLatency = "signal_handle_latency"

	// Signal receive is when we receive it off the channel
	SignalReceive = "signal_receive"
	PollTick      = "poll_tick"
	ContextCancel = "context_canceled"

	// Forced shutdown timeout when a workflow is suspected to be abandoned
	ShutdownTimeout = "shutdown_timeout"

	// Metrics are scoped to workflow namespaces anyways so let's
	// keep these metrics simple.
	WorkflowSuccess = "success"
	WorkflowFailure = "failure"
	WorkflowLatency = "latency"

	ManualOverride          = "manual_override"
	ManualOverrideReasonTag = "reason"

	// Terraform workflow execution metrics
	TerraformWorkflowExecution = "terraform_workflow_execution"
	TerraformWorkflowDuration  = "terraform_workflow_duration"
	WorkflowType               = "workflow_type" // PR, Deploy, Adhoc
	WorkflowStatus             = "status"        // success, failure
	WorkflowRepo               = "repository"
	WorkflowPRNum              = "pr_number"
	WorkflowRoot               = "root_name"
)
