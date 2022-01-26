package handlers

import (
	"sync"

	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
)

// AsyncProjectCommandOutputHandler is a handler to transport terraform client
// outputs to the front end.
type AsyncProjectCommandOutputHandler struct {
	projectCmdOutput chan *models.ProjectCmdOutputLine

	projectOutputBuffers     map[string][]string
	projectOutputBuffersLock sync.RWMutex

	receiverBuffers     map[string]map[chan string]bool
	receiverBuffersLock sync.RWMutex

	projectStatusUpdater   ProjectStatusUpdater
	projectJobURLGenerator ProjectJobURLGenerator

	logger logging.SimpleLogging

	// Tracks all the jobs for a pull request
	// Used for clean up after a pull request is closed.
	pullToJobMapping map[models.PullContext]map[string]bool
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_job_id_generator.go JobIDGenerator
type JobIDGenerator interface {
	GenerateJobID() string
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_project_job_url_generator.go ProjectJobURLGenerator

// ProjectJobURLGenerator generates urls to view project's progress.
type ProjectJobURLGenerator interface {
	GenerateProjectJobURL(p models.ProjectCommandContext) (string, error)
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_project_status_updater.go ProjectStatusUpdater

type ProjectStatusUpdater interface {
	// UpdateProject sets the commit status for the project represented by
	// ctx.
	UpdateProject(ctx models.ProjectCommandContext, cmdName models.CommandName, status models.CommitStatus, url string) error
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_project_command_output_handler.go ProjectCommandOutputHandler

type ProjectCommandOutputHandler interface {
	// Clear clears the buffer from previous terraform output lines
	Clear(ctx models.ProjectCommandContext)

	// Send will enqueue the msg and wait for Handle() to receive the message.
	Send(ctx models.ProjectCommandContext, msg string)

	// Register registers a channel and blocks until it is caught up. Callers should call this asynchronously when attempting
	// to read the channel in the same goroutine
	Register(jobID string, receiver chan string)

	// Deregister removes a channel from successive updates and closes it.
	Deregister(jobID string, receiver chan string)

	// Listens for msg from channel
	Handle()

	// SetJobURLWithStatus sets the commit status for the project represented by
	// ctx and updates the status with and url to a job.
	SetJobURLWithStatus(ctx models.ProjectCommandContext, cmdName models.CommandName, status models.CommitStatus) error

	ResourceCleaner
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_resource_cleaner.go ResourceCleaner

type ResourceCleaner interface {
	CleanUp(jobContext models.PullContext)
}

func NewAsyncProjectCommandOutputHandler(
	projectCmdOutput chan *models.ProjectCmdOutputLine,
	projectStatusUpdater ProjectStatusUpdater,
	projectJobURLGenerator ProjectJobURLGenerator,
	logger logging.SimpleLogging,
) ProjectCommandOutputHandler {
	return &AsyncProjectCommandOutputHandler{
		projectCmdOutput:       projectCmdOutput,
		logger:                 logger,
		receiverBuffers:        map[string]map[chan string]bool{},
		projectStatusUpdater:   projectStatusUpdater,
		projectJobURLGenerator: projectJobURLGenerator,
		projectOutputBuffers:   map[string][]string{},
		pullToJobMapping:       map[models.PullContext]map[string]bool{},
	}
}

func (p *AsyncProjectCommandOutputHandler) Send(ctx models.ProjectCommandContext, msg string) {
	p.projectCmdOutput <- &models.ProjectCmdOutputLine{
		JobID: ctx.JobID,
		JobContext: models.JobContext{
			HeadCommit: ctx.Pull.HeadCommit,
			PullContext: models.PullContext{
				PullNum:     ctx.Pull.Num,
				Repo:        ctx.BaseRepo.Name,
				ProjectName: ctx.ProjectName,
				Workspace:   ctx.Workspace,
			},
		},
		Line: msg,
	}
}

func (p *AsyncProjectCommandOutputHandler) Register(jobID string, receiver chan string) {
	p.addChan(receiver, jobID)
}

func (p *AsyncProjectCommandOutputHandler) Handle() {
	for msg := range p.projectCmdOutput {
		if msg.ClearBuffBefore {
			p.clearLogLines(msg.JobID)
		} else {
			if p.pullToJobMapping[msg.JobContext.PullContext] == nil {
				p.pullToJobMapping[msg.JobContext.PullContext] = map[string]bool{}
			}
			p.pullToJobMapping[msg.JobContext.PullContext][msg.JobID] = true
		}
		p.writeLogLine(msg.JobID, msg.Line)
	}
}

func (p *AsyncProjectCommandOutputHandler) Clear(ctx models.ProjectCommandContext) {
	p.projectCmdOutput <- &models.ProjectCmdOutputLine{
		JobID:           ctx.JobID,
		Line:            models.LogStreamingClearMsg,
		ClearBuffBefore: true,
	}
}

func (p *AsyncProjectCommandOutputHandler) SetJobURLWithStatus(ctx models.ProjectCommandContext, cmdName models.CommandName, status models.CommitStatus) error {
	url, err := p.projectJobURLGenerator.GenerateProjectJobURL(ctx)

	if err != nil {
		return err
	}
	return p.projectStatusUpdater.UpdateProject(ctx, cmdName, status, url)
}

func (p *AsyncProjectCommandOutputHandler) clearLogLines(jobID string) {
	p.projectOutputBuffersLock.Lock()
	delete(p.projectOutputBuffers, jobID)
	p.projectOutputBuffersLock.Unlock()
}

func (p *AsyncProjectCommandOutputHandler) addChan(ch chan string, jobID string) {
	p.projectOutputBuffersLock.RLock()
	buffer := p.projectOutputBuffers[jobID]
	p.projectOutputBuffersLock.RUnlock()

	for _, line := range buffer {
		ch <- line
	}

	// add the channel to our registry after we backfill the contents of the buffer,
	// to prevent new messages coming in interleaving with this backfill.
	p.receiverBuffersLock.Lock()
	if p.receiverBuffers[jobID] == nil {
		p.receiverBuffers[jobID] = map[chan string]bool{}
	}
	p.receiverBuffers[jobID][ch] = true
	p.receiverBuffersLock.Unlock()
}

//Add log line to buffer and send to all current channels
func (p *AsyncProjectCommandOutputHandler) writeLogLine(jobID string, line string) {
	p.receiverBuffersLock.Lock()
	for ch := range p.receiverBuffers[jobID] {
		select {
		case ch <- line:
		default:
			// Client ws conn could be closed in two ways:
			// 1. Client closes the conn gracefully -> the closeHandler() is executed which
			//  	closes the channel and cleans up resources.
			// 2. Client does not close the conn and the closeHandler() is not executed -> the
			// 		receiverChan will be blocking for N number of messages (equal to buffer size)
			// 		before we delete the channel and clean up the resources.
			delete(p.receiverBuffers[jobID], ch)
		}
	}
	p.receiverBuffersLock.Unlock()

	// No need to write to projectOutputBuffers if clear msg.
	if line == models.LogStreamingClearMsg {
		return
	}

	p.projectOutputBuffersLock.Lock()
	if p.projectOutputBuffers[jobID] == nil {
		p.projectOutputBuffers[jobID] = []string{}
	}
	p.projectOutputBuffers[jobID] = append(p.projectOutputBuffers[jobID], line)
	p.projectOutputBuffersLock.Unlock()
}

//Remove channel, so client no longer receives Terraform output
func (p *AsyncProjectCommandOutputHandler) Deregister(jobID string, ch chan string) {
	p.logger.Debug("Removing channel for %s", jobID)
	p.receiverBuffersLock.Lock()
	delete(p.receiverBuffers[jobID], ch)
	p.receiverBuffersLock.Unlock()
}

func (p *AsyncProjectCommandOutputHandler) GetReceiverBufferForPull(jobID string) map[chan string]bool {
	return p.receiverBuffers[jobID]
}

func (p *AsyncProjectCommandOutputHandler) GetProjectOutputBuffer(jobID string) []string {
	return p.projectOutputBuffers[jobID]
}

func (p *AsyncProjectCommandOutputHandler) GetJobIdMapForPullContext(pullContext models.PullContext) map[string]bool {
	return p.pullToJobMapping[pullContext]
}

func (p *AsyncProjectCommandOutputHandler) CleanUp(pullContext models.PullContext) {
	if jobIdMap, ok := p.pullToJobMapping[pullContext]; ok {
		for jobID := range jobIdMap {
			p.projectOutputBuffersLock.Lock()
			delete(p.projectOutputBuffers, jobID)
			p.projectOutputBuffersLock.Unlock()

			// Only delete the pull record from receiver buffers.
			// WS channel will be closed when the user closes the browser tab
			// in closeHanlder().
			p.receiverBuffersLock.Lock()
			delete(p.receiverBuffers, jobID)
			p.receiverBuffersLock.Unlock()
		}

		delete(p.pullToJobMapping, pullContext)
	}
}

// NoopProjectOutputHandler is a mock that doesn't do anything
type NoopProjectOutputHandler struct{}

func (p *NoopProjectOutputHandler) Send(ctx models.ProjectCommandContext, msg string) {
}

func (p *NoopProjectOutputHandler) Register(jobID string, receiver chan string)   {}
func (p *NoopProjectOutputHandler) Deregister(jobID string, receiver chan string) {}

func (p *NoopProjectOutputHandler) Handle() {
}

func (p *NoopProjectOutputHandler) Clear(ctx models.ProjectCommandContext) {
}

func (p *NoopProjectOutputHandler) SetJobURLWithStatus(ctx models.ProjectCommandContext, cmdName models.CommandName, status models.CommitStatus) error {
	return nil
}

func (p *NoopProjectOutputHandler) CleanUp(pullContext models.PullContext) {
}

func (p *NoopProjectOutputHandler) GenerateJobID(pull models.PullRequest, projectName string, workspace string) string {
	return ""
}
