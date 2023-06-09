# Atlantis

This was forked from [runatlantis/atlantis](https://github.com/runatlantis/atlantis) at [v0.17.3](https://github.com/runatlantis/atlantis/releases/tag/v0.17.3)

Since then this version has diverged significantly from upstream and was therefore detached.

:warning: This repo is still still contains a lot of code from upstream which is slowly being phased out as we test our implementation in production.  It is not ready for general consumption.


## What's different?

**Functional Differences**
* Applies automatically occur AFTER merging a PR (In PR applies can be enabled for certain repos but this shouldn't be used primarily)
* Applies are queued for a given root as commits are merged in
* No PR locks; concurrent plans can be run against the same root

**Non Functional Differences**
* 2 microservices: Gateway event proxy + Worker
* Improved reliability and availability since you can have n number of proxies or workers or both.
* [Temporal](https://docs.temporal.io/) is used heavily to maintain execution state, provide host-level routing and provide fault tolerance to our workflows.

## Architecture

Most of the new code can be found in `server/neptune`. Neptune is the codename for our rebuild of Atlantis.  Outside of that package is mostly old code which we are removing as we deprecate upstream/legacy workers. 

Currently, Atlantis can operate in 3 modes based on the configuration passed in:
* Gateway
* Legacy Worker (This is currently being deprecated as we cutover fully to the Temporal worker within Lyft)
* Temporal Worker

In order for Neptune to work correctly, all of these must exist.  

## Gateway
Receives webhook events from github and acts on them accordingly.  Gateway is stateless however each request does spin up a go routine that clones a repository to disk.  This is the primary bottleneck here.

### Events

#### Push
* Clone repository on disk and fetch root configuration.
* Get the latest diff and determine if any roots have changed.
* Start/signal the deploy temporal workflow for each root that requires processing.

#### Pull
* Clone repository on disk and fetch root configuration.
* Get the latest diff of the entire PR and determine if any roots have changed.
* Proxy event to legacy worker if any roots require processing.

#### Check Run
* Determine which action is being triggered.  We have custom buttons that are thrown up on the check run depending on the situation.  As of now there are 2 types of events we are looking for:  `unlocked` and `plan_review`.  When we receive both these events, we signal our deploy workflow and terraform workflow respectively.
* Listens for the GH provided `Re-run failed checkruns` button selection and signals our deploy workflow that we are attempting to add the previously failing revision back into the deploy queue for a rerun attempt. This is similar to the check suite event described below, but the key difference here is this request only reruns attempts where a checkrun has failed. 

#### Check Suite
* Listens for GH provided `Re-run all checkruns` button selection events and signals our deploy workflows that we are attempting add all off the modified roots within a revision back into their respective deploy queues for a rerun attempt, regardless of success or failure status. Note that we only support rerun attempts if a revision meets the following criteria:
  - check run request comes from a revision on the default branch of a repo (force applies are not allowed)
  - revision is identical to the latest persisted deployment attempt for a specific root
## Legacy Worker
Responsible for speculative planning and policy checking within the PR.  This code is relatively untouched from upstream atlantis and should eventually be nuked in favor of Temporal workflows. 

## Temporal Worker
Responsible for running 3 primary processes:
* HTTP Server for viewing realtime logs
* Terraform Task Queue Worker for running Terraform Workflow tasks
* Deploy Task Queue Worker for running Deploy Workflow tasks

## Temporal Workflows

### Deploy

Deploy workflows are run on the granularity of a single repository root.  It follows the ID pattern below:
```
<OWNER/REPO>||<ROOT_NAME>
```

The following is a high level diagram of how this workflow is structured:

```
                                           ┌─────────────┐
                                           │             │
                                           │  deployment │
                                           │    store    │
┌─────────────────┐                        │             │
│     select      │                        └────▲───┬────┘
│                 │                             │   │
│                 │                             │   │
│                 │                             │   │
├┬───────────────┬┤                        ┌────┴───▼──────┐
││revision signal││    ┌──────────────┐    │               │
││   channel     │┼────►priority queue├────► queue worker  │
│┼───────────────┼│    └──────────────┘    │               │
│┼───────────────┼│                        └──────┬────────┘
││ timeout timer ││                               │
│┼───────────────┼│                               │
└─────────────────┘                       ┌───────▼─────────┐
                                          │     select      │
                                          │                 │
                                          │                 │
                                          │                 │
                                          ├┬───────────────┬┤
                                          ││ queue::CanPop │┼───────────────┐
                                          ││               ││               │
                                          │┼───────────────┼│      ┌────────▼─────────┐
                                          │┼───────────────┼│      │      select      │
                                          ││ unlock signal ││      │                  │         ┌─────────┐
                                          │┼───────────────┼│      │                  │         │ Github  │
                                          └─────────────────┘      │                  │         └▲────────┘
                                                                   ├┬────────────────┬┤          │
                                                                   ││ state change   ││          │
                                                                   ││ signal channel │┼──────────┴───┐
                                                                   │┼────────────────┼│              │
                                                                   │┼────────────────┼│              │
                                                                   ││ child workflow ││           ┌──▼──┐
                                                                   │┼────────────────┼│           │ SNS │
                                                                   └──────────────────┘           └─────┘
```

The deploy workflow is responsible for a few things:
* Receiving revisions to deploy from gateway.
* Queueing up successive terraform workflows.
* Updating github check run state based on the in-progress terraform workflow

In order to receive revisions our main workflow thread listens to a dedicated channel.  If we haven't received a new revision in 60 minutes and our queue is empty, we instigate a shutdown of the workflow in its entirety.

#### Queue

The queue is modeled as a priority queue where manually triggered deployments always happen before merge triggered deployments.  This queue can be in a locked state if the last deployment that happened was triggered manually.  The queue lock applies only to deployments that have been triggered via merging and can be unlocked through the corresponding check run of an item that is blocked.  

Items can only be popped of the queue if the queue is unlocked OR if the queue is locked but contains a manually triggered deployment.  

By default, a new workflow starts up in an unlocked state.

#### Queue Worker
Upon workflow startup, we start a queue worker go routine which is responsible for popping off items from our queue when it can, and listening for unlock signals from gateway.

The worker also maintains local state on the latest deployment that has happened. This is used for validating that new revisions intended for deploy are ahead of the latest deployed revisions.  Once each deployment is complete, the worker persists this information in the configured blob store.  The worker only fetches from this blob store on workflow startup and maintains the information locally for lifetime of its execution.

A deploy consists of executing a terraform workflow.  The worker blocks on execution of this "child" workflow and listens for state changes via a dedicated signal channel.  Once the child is complete, we stop listening to this signal channel and move on.

#### State Signal

State changes are reflected in the github checks UI (ie. plan in progress, plan failed, apply in progress etc.).  A single check run is used to represent the deployment state.  The check run state is indicative of the completion state of the deployment and the details of the deployment itself are rendered in the check run details section.

State changes for apply jobs specifically are sent to SNS for internal auditing purposes.

### Terraform

The Terraform workflow runs on the granularity of a single deployment.  It's identifier is the deployment's identifier which is randomly generated in the Deploy Workflow.  Note: this means a single revision can be tied to multiple deployments.

The terraform workflow is stateful due to the fact that it keeps data on disk and references it throughout the workflow.  Cleanup of that data only happens when that workflow is complete.  In order to ensure this statefulness, the terraform workflow is aware of the worker it's running on and fetches this information as part of the first activity.  Each successive activity task takes place on the same task queue.  

Following this:
* The workflow clones a repository and stores it on disk
* Generates a Job ID for the first set of Terraform Operations
* Runs Terraform Init followed by Terraform Plan

Before and after this job, the workflow signals it's parent execution with a state object.  At this point, the workflow either blocks on a dedicated plan review channel, or proceeds to the apply under some criteria. Atm this is only if there are no changes in the plan.

Plan review signals are received directly by this workflow from gateway which pulls the workflow ID from the check run's `External ID` field.  

If the plan is approved, the workflow proceeds with the apply, all the while updating the parent execution with the status, before exiting the workflow.
If the plan is rejected or times out (1 week timeout on plan reviews), the parent is notified and the workflow exits.

#### Retries

The workflow itself has no retries configured.  All activities use the default retry policy except for Terraform Activities.  Terraform Activities throw up a `TerraformClientError` if there is an error from the binary itself.  This error is configured to be non-retryable since most of the time this is a user error.  

For Terraform Applies, timeouts are not retried. Timeouts can happen from exceeding the ScheduleToClose threshold or from lack of heartbeat for over a minute.  Instead of retrying the apply, which can have unpredictable results, we signal our parent that there has been a timeout and this is surfaced to the user.

#### Heartbeats

Since Terraform activities can run long, we send hearbeats at 5 second intervals. If 1 minute goes by without receiving a hearbeat, temporal will assume the worker node is down and the configured retry policy will be run.

#### Job Logs

Terraform operation logs are streamed to the local server process using go channels.  Once the operation is complete, the channel is closed and the receiving process persists the logs to the configured job store.  

## Developing

### Running Atlantis Locally
* Clone the repo from https://github.com/runatlantis/atlantis/
* Compile Atlantis:
    ```
    go install
    ```
* Run Atlantis:
    ```
    atlantis server --gh-user <your username> --gh-token <your token> --repo-allowlist <your repo> --gh-webhook-secret <your webhook secret> --log-level debug
    ```
    If you get an error like `command not found: atlantis`, ensure that `$GOPATH/bin` is in your `$PATH`.

### Running Atlantis With Local Changes

The atlantis worker can't technically be run yet locally given it's dependency on sqs.
However, Docker compose is set up to run a gateway, a temporal worker,
temporalite and ngrok all in the same network.
Ngrok allows us to expose localhost to the public internet in order to test github app integrations.

There is some setup that is required in order to have your containers running and receiving webhooks.

1. Setup your own personal github organization and test github app.
2. Install this app to a test repo within your organization with the following configuration.
    * Repository permissions
      * Checks - Read and Write
      * Commit statuses - Read and Write
      * Contents - Read and Write
      * Issues - Read and Write
      * Pull requests - Read and Write
    * Organization permissions
      * Members - Read-only
    * Subscribe to events
      * Create
      * Issue comment
      * Pull request
      * Pull request review
      * Push
    * Webhook - This will be setup later when you start ngrok and get the webhook URL, until then fill out any value to get past the app create validation.
3. Download the key file and save it to `~/.ssh` directory. Note: `~/.ssh` is mounted to allow for referencing any local ssh keys.

4. Create the following files:
`~/atlantis-gateway.env`
```sh
ATLANTIS_PORT=4143
ATLANTIS_GH_APP_ID=<FILL THIS IN>
ATLANTIS_GH_APP_KEY_FILE=/.ssh/your-key-file.pem
ATLANTIS_GH_WEBHOOK_SECRET=<FILL THIS IN>
ATLANTIS_GH_APP_SLUG=<FILL THIS IN>

# The github organization the feature flag repo resides
ATLANTIS_FF_OWNER=<FILL THIS IN>
# Name of the feature flag repo
ATLANTIS_FF_REPO=<FILL THIS IN>
# The path to the flags.yaml file
ATLANTIS_FF_PATH=<FILL THIS IN>

ATLANTIS_DATA_DIR=/tmp/
ATLANTIS_LYFT_MODE=gateway
ATLANTIS_REPO_CONFIG=/generated/repo-config.yaml
ATLANTIS_WRITE_GIT_CREDS=true
ATLANTIS_ENABLE_POLICY_CHECKS=true
ATLANTIS_ENABLE_DIFF_MARKDOWN_FORMAT=true

ATLANTIS_REPO_ALLOWLIST=<FILL THIS IN>
ALLOWED_REPOS=<FILL THIS IN>
```

`~/atlantis-temporalworker.env`
```sh
ATLANTIS_PORT=4142
ATLANTIS_GH_APP_ID=<FILL THIS IN>
ATLANTIS_GH_APP_KEY_FILE=/.ssh/your-key-file.pem
ATLANTIS_GH_WEBHOOK_SECRET=<FILL THIS IN>
ATLANTIS_GH_APP_SLUG=<FILL THIS IN>
ATLANTIS_FF_OWNER=<FILL THIS IN>
ATLANTIS_FF_REPO=<FILL THIS IN>
ATLANTIS_FF_PATH=<FILL THIS IN>
ATLANTIS_DATA_DIR=/tmp/
ATLANTIS_LYFT_MODE=temporalworker
ATLANTIS_REPO_CONFIG=/generated/repo-config.yaml
ATLANTIS_WRITE_GIT_CREDS=true
ATLANTIS_ENABLE_POLICY_CHECKS=true
ATLANTIS_ENABLE_DIFF_MARKDOWN_FORMAT=true
ATLANTIS_REPO_ALLOWLIST=<FILL THIS IN>
ALLOWED_REPOS=<FILL THIS IN>
ATLANTIS_DEFAULT_TF_VERSION=1.2.5
```

Once these steps are complete, everything should startup as normal. You just need to run:

```
make build-service
docker-compose build
docker-compose up
```

In order to have events routed to gateway, you'll need to visit `http://localhost:4040/` and copy the https url into your GitHub app.  

In order to see the temporal ui visit `http://localhost:8233/`.

### Rebuilding

If the ngrok container is restarted, the url will change which is a hassle. Fortunately, when we make a code change, we can rebuild and restart the atlantis container easily without disrupting ngrok.

e.g.

```
make build-service
docker-compose up --detach --build
```

## Running Tests Locally:

`make test`. If you want to run the integration tests that actually run real `terraform` commands, run `make test-all`.

## Running Tests In Docker:
```
docker run --rm -v $(pwd):/go/src/github.com/runatlantis/atlantis -w /go/src/github.com/runatlantis/atlantis ghcr.io/runatlantis/testing-env:latest make test
```

Or to run the integration tests
```
docker run --rm -v $(pwd):/go/src/github.com/runatlantis/atlantis -w /go/src/github.com/runatlantis/atlantis ghcr.io/runatlantis/testing-env:latest make test-all
```

## Calling Your Local Atlantis From GitHub
- Create a Github organization by following the steps in this [tutorial](https://docs.github.com/en/organizations/collaborating-with-groups-in-organizations/creating-a-new-organization-from-scratch).
- In the homepage of your organization, navigate to the settings, developer settings and finally to Github Apps. 
- Click on new Github App. Set the homepage URL to https://www.atlantis.com and the setup the webhook URL as described in the steps below. 
- Under Repository Permissions, set it to following: 
```
	- Metadata: Read-Only 
	- Pull Requests: Read and Write
	- Commit Statuses: Read and Write
```
Similarly, subscribe for the following events: 
```
	- Pull Request
	- Issue Comment
```
- Create a test terraform repository in your GitHub.
- Create a personal access token for Atlantis. See [Create a GitHub token](https://github.com/runatlantis/atlantis/tree/master/runatlantis.io/docs/access-credentials.md#generating-an-access-token).
- Start Atlantis in server mode using that token:
```
atlantis server --gh-user <your username> --gh-token <your token> --repo-allowlist <your repo> --gh-webhook-secret <your webhook secret> --log-level debug
```
- Download ngrok from https://ngrok.com/download. This will enable you to expose Atlantis running on your laptop to the internet so GitHub can call it.
- When you've downloaded and extracted ngrok, run it on port `4141`:
```
ngrok http 4141
```
- Create a Webhook in your repo and use the `https` url that `ngrok` printed out after running `ngrok http 4141`. Be sure to append `/events` so your webhook url looks something like `https://efce3bcd.ngrok.io/events`. See [Add GitHub Webhook](https://github.com/runatlantis/atlantis/blob/master/runatlantis.io/docs/configuring-webhooks.md#configuring-webhooks).
- Create a pull request and type `atlantis help`. You should see the request in the `ngrok` and Atlantis logs and you should also see Atlantis comment back.

## Code Style
### Logging
- `ctx.Log` should be available in most methods. If not, pass it down.
- levels:
    - debug is for developers of atlantis
    - info is for users (expected that people run on info level)
    - warn is for something that might be a problem but we're not sure
    - error is for something that's definitely a problem
- **ALWAYS** logs should be all lowercase (when printed, the first letter of each line will be automatically capitalized)
- **ALWAYS** quote any string variables using %q in the fmt string, ex. `ctx.Log.Infof("cleaning clone dir %q", dir)` => `Cleaning clone directory "/tmp/atlantis/lkysow/atlantis-terraform-test/3"`
- **NEVER** use colons "`:`" in a log since that's used to separate error descriptions and causes
  - if you need to have a break in your log, either use `-` or `,` ex. `failed to clean directory, continuing regardless`

### Errors
- **ALWAYS** use lowercase unless the word requires it
- **ALWAYS** use `errors.Wrap(err, "additional context...")"` instead of `fmt.Errorf("additional context: %s", err)`
because it is less likely to result in mistakes and gives us the ability to trace call stacks
- **NEVER** use the words "error occurred when...", or "failed to..." or "unable to...", etc. Instead, describe what was occurring at
time of the error, ex. "cloning repository", "creating AWS session". This will prevent errors from looking like
```
Error setting up workspace: failed to run git clone: could find git
```

and will instead look like
```
Error: setting up workspace: running git clone: no executable "git"
```
This is easier to read and more consistent

### Testing
- place tests under `{package under test}_test` to enforce testing the external interfaces
- if you need to test internally i.e. access non-exported stuff, call the file `{file under test}_internal_test.go`
- use our testing utility for easier-to-read assertions: `import . "github.com/runatlantis/atlantis/testing"` and then use `Assert()`, `Equals()` and `Ok()`

### Mocks
We write our own mocks to reduce dependencies.  Most of the old code uses [pegomock](https://github.com/petergtz/pegomock) which is unmaintained.  We shouldn't use this for any new changes going forward.
