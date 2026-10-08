package temporalworker

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gogo/protobuf/jsonpb"
	lyftWorkflows "github.com/runatlantis/atlantis/server/neptune/lyft/workflows"
	"github.com/runatlantis/atlantis/server/neptune/temporal"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	enumspb "go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// replayHistoryDirEnv names a directory of workflow histories exported as JSON,
// e.g. with `temporal workflow show --output json`. Histories hold production
// workflow inputs, so they are read from a local directory rather than checked in.
const replayHistoryDirEnv = "ATLANTIS_REPLAY_HISTORY_DIR"

// TestReplayHistories replays exported workflow histories against the current
// workflow code, registered the way the worker registers it. A failure means a
// code or SDK change would break workflows that are already running.
func TestReplayHistories(t *testing.T) {
	dir := os.Getenv(replayHistoryDirEnv)
	if dir == "" {
		t.Skipf("set %s to a directory of exported workflow history JSON files", replayHistoryDirEnv)
	}

	// Reads go through os.Root, so a history name can't reach outside dir.
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	defer root.Close()

	files, err := fs.Glob(root.FS(), "*.json")
	require.NoError(t, err)
	require.NotEmpty(t, files, "no *.json histories in %s", dir)

	replayer, err := worker.NewWorkflowReplayerWithOptions(worker.WorkflowReplayerOptions{
		Interceptors: []interceptor.WorkerInterceptor{temporal.NewWorkerInterceptor()},
	})
	require.NoError(t, err)

	replayer.RegisterWorkflowWithOptions(deployWorkflow(), workflow.RegisterOptions{Name: workflows.Deploy})
	replayer.RegisterWorkflow(workflows.PR)
	replayer.RegisterWorkflow(workflows.Terraform)
	replayer.RegisterWorkflow(lyftWorkflows.PRRevision)

	for _, name := range files {
		name := name
		t.Run(name, func(t *testing.T) {
			history, err := readHistory(root, name)
			require.NoError(t, err)
			require.NoError(t, replayer.ReplayWorkflowHistory(nil, history))
		})
	}
}

// readHistory parses an exported history. Newer servers send fields this SDK's
// API version doesn't define; the worker ignores them when it receives history
// as protobuf, so they're ignored here too.
func readHistory(root *os.Root, name string) (*historypb.History, error) {
	raw, err := root.ReadFile(name)
	if err != nil {
		return nil, err
	}
	converted, err := legacyEnumNames(raw)
	if err != nil {
		return nil, err
	}
	var history historypb.History
	u := jsonpb.Unmarshaler{AllowUnknownFields: true}
	if err := u.Unmarshal(bytes.NewReader(converted), &history); err != nil {
		return nil, err
	}
	return &history, nil
}

// historyEnums maps each enum's JSON prefix in current Temporal CLI exports
// (e.g. EVENT_TYPE_) to the value names the SDK's JSON parser accepts.
var historyEnums = map[string]map[string]int32{
	"ARCHIVAL_STATE_": enumspb.ArchivalState_value,
	"CANCEL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_": enumspb.CancelExternalWorkflowExecutionFailedCause_value,
	"COMMAND_TYPE_":              enumspb.CommandType_value,
	"CONTINUE_AS_NEW_INITIATOR_": enumspb.ContinueAsNewInitiator_value,
	"ENCODING_TYPE_":             enumspb.EncodingType_value,
	"EVENT_TYPE_":                enumspb.EventType_value,
	"INDEXED_VALUE_TYPE_":        enumspb.IndexedValueType_value,
	"PARENT_CLOSE_POLICY_":       enumspb.ParentClosePolicy_value,
	"RETRY_STATE_":               enumspb.RetryState_value,
	"SIGNAL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_": enumspb.SignalExternalWorkflowExecutionFailedCause_value,
	"START_CHILD_WORKFLOW_EXECUTION_FAILED_CAUSE_":     enumspb.StartChildWorkflowExecutionFailedCause_value,
	"TASK_QUEUE_KIND_":                       enumspb.TaskQueueKind_value,
	"TIMEOUT_TYPE_":                          enumspb.TimeoutType_value,
	"WORKFLOW_EXECUTION_STATUS_":             enumspb.WorkflowExecutionStatus_value,
	"WORKFLOW_ID_REUSE_POLICY_":              enumspb.WorkflowIdReusePolicy_value,
	"WORKFLOW_TASK_FAILED_CAUSE_":            enumspb.WorkflowTaskFailedCause_value,
	"RESET_REAPPLY_TYPE_":                    enumspb.ResetReapplyType_value,
	"SEVERITY_":                              enumspb.Severity_value,
	"QUERY_REJECT_CONDITION_":                enumspb.QueryRejectCondition_value,
	"WORKFLOW_UPDATE_DURABILITY_PREFERENCE_": enumspb.WorkflowUpdateDurabilityPreference_value,
	"WORKFLOW_UPDATE_RESULT_ACCESS_STYLE_":   enumspb.WorkflowUpdateResultAccessStyle_value,
	"SCHEDULE_OVERLAP_POLICY_":               enumspb.ScheduleOverlapPolicy_value,
	"PENDING_ACTIVITY_STATE_":                enumspb.PendingActivityState_value,
	"PENDING_WORKFLOW_TASK_STATE_":           enumspb.PendingWorkflowTaskState_value,
	"RESOURCE_EXHAUSTED_CAUSE_":              enumspb.ResourceExhaustedCause_value,
}

var screamingSnake = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// legacyEnumNames rewrites enum values in a history exported by a current
// Temporal CLI (EVENT_TYPE_WORKFLOW_EXECUTION_STARTED) to the names this SDK
// version's JSON parser accepts (WorkflowExecutionStarted). Other values are
// left alone.
func legacyEnumNames(raw []byte) ([]byte, error) {
	var doc interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return json.Marshal(rewriteEnums(doc))
}

func rewriteEnums(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, child := range t {
			t[k] = rewriteEnums(child)
		}
		return t
	case []interface{}:
		for i, child := range t {
			t[i] = rewriteEnums(child)
		}
		return t
	case string:
		return legacyEnumName(t)
	default:
		return v
	}
}

func legacyEnumName(s string) string {
	if !screamingSnake.MatchString(s) {
		return s
	}
	for prefix, values := range historyEnums {
		if !strings.HasPrefix(s, prefix) {
			continue
		}
		name := camelCase(strings.TrimPrefix(s, prefix))
		if _, ok := values[name]; ok {
			return name
		}
	}
	return s
}

func camelCase(s string) string {
	var b strings.Builder
	for _, part := range strings.Split(strings.ToLower(s), "_") {
		if part != "" {
			b.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
	}
	return b.String()
}

func TestLegacyEnumNames(t *testing.T) {
	in := []byte(`{"events":[{"eventId":"1","eventType":"EVENT_TYPE_WORKFLOW_EXECUTION_STARTED",` +
		`"workflowExecutionStartedEventAttributes":{"taskQueue":{"name":"deploy","kind":"TASK_QUEUE_KIND_NORMAL"},` +
		`"parentClosePolicy":"PARENT_CLOSE_POLICY_REQUEST_CANCEL","identity":"KEEP_ME"}}]}`)

	out, err := legacyEnumNames(in)
	require.NoError(t, err)

	s := string(out)
	assert.Contains(t, s, `"eventType":"WorkflowExecutionStarted"`)
	assert.Contains(t, s, `"kind":"Normal"`)
	assert.Contains(t, s, `"parentClosePolicy":"RequestCancel"`)
	assert.Contains(t, s, `"identity":"KEEP_ME"`)
}
