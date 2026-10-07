package markdown_test

import (
	"net/url"
	"testing"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github/markdown"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform/failurehints"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"github.com/stretchr/testify/assert"
)

func failedJob(failure terraform.Failure) *state.Job {
	return &state.Job{
		Status: state.FailedJobStatus,
		Output: &state.JobOutput{
			URL:     &url.URL{Scheme: "https", Host: "atlantis.example.com", Path: "/jobs/1234"},
			Failure: failure,
		},
	}
}

func TestRenderWorkflowStateTmpl_planFailure(t *testing.T) {
	out := markdown.RenderWorkflowStateTmpl(&state.Workflow{
		Plan: failedJob(terraform.Failure{
			Errors: []string{`Error: Unsupported argument` + "\n\n" + `An argument named "bucket_policy" is not expected here.`},
			Hint: &failurehints.Hint{
				Rule:        "unsupported-argument",
				Explanation: "The configuration passes an argument that the resource or module doesn't accept.",
				Fix:         "Rename or remove the argument.",
			},
		}),
	})

	assert.Contains(t, out, "## Plan Failed :x:")
	assert.Contains(t, out, "**Why it failed:** The configuration passes an argument that the resource or module doesn&#39;t accept.")
	assert.Contains(t, out, "**How to fix it:** Rename or remove the argument.")
	assert.Contains(t, out, "<details><summary>Terraform error</summary>")
	// Error text is HTML-escaped so it can't inject markup into the check run.
	assert.Contains(t, out, "<pre>Error: Unsupported argument\n\nAn argument named &#34;bucket_policy&#34; is not expected here.</pre>")
	assert.NotContains(t, out, "## Apply Failed")
}

func TestRenderWorkflowStateTmpl_applyFailureWithoutHint(t *testing.T) {
	out := markdown.RenderWorkflowStateTmpl(&state.Workflow{
		Plan: &state.Job{
			Status: state.SuccessJobStatus,
			Output: &state.JobOutput{URL: &url.URL{Scheme: "https", Host: "atlantis.example.com"}},
		},
		Apply: failedJob(terraform.Failure{Errors: []string{"Error: <script>alert(1)</script>"}}),
	})

	assert.Contains(t, out, "## Apply Failed :x:")
	assert.Contains(t, out, "<pre>Error: &lt;script&gt;alert(1)&lt;/script&gt;</pre>")
	assert.NotContains(t, out, "<script>")
	assert.NotContains(t, out, "Why it failed")
	assert.NotContains(t, out, "## Plan Failed")
}

func TestRenderWorkflowStateTmpl_failedJobWithoutDetails(t *testing.T) {
	out := markdown.RenderWorkflowStateTmpl(&state.Workflow{
		Plan: failedJob(terraform.Failure{}),
	})

	assert.NotContains(t, out, "## Plan Failed")
	assert.NotContains(t, out, "Terraform error")
}
