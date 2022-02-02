package handlers

import (
	"sync"

	"github.com/runatlantis/atlantis/server/events/models"
	jobmodels "github.com/runatlantis/atlantis/server/jobs/models"
	"github.com/runatlantis/atlantis/server/logging"
)

// AsyncProjectCommandOutputHandler is a handler to transport terraform client
// outputs to the front end.
type AsyncProjectCommandOutputHandler struct {
	projectCmdOutput chan *jobmodels.ProjectCmdOutputLine

	projectOutputBuffers     map[string]jobmodels.OutputBuffer
	projectOutputBuffersLock sync.RWMutex

	receiverBuffers     map[string]map[chan string]bool
	receiverBuffersLock sync.RWMutex

	logger logging.SimpleLogging

	// Tracks all the jobs for a pull request which is used for clean up after a pull request is closed.
	pullToJobMapping sync.Map
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o ../mocks/mock_project_command_output_handler.go ProjectCommandOutputHandler

type ProjectCommandOutputHandler interface {
	// Send will enqueue the msg and wait for Handle() to receive the message.
	Send(ctx models.ProjectCommandContext, msg string, operationComplete bool)

	// Register registers a channel and blocks until it is caught up. Callers should call this asynchronously when attempting
	// to read the channel in the same goroutine
	Register(jobID string, receiver chan string)

	// Deregister removes a channel from successive updates and closes it.
	Deregister(jobID string, receiver chan string)

	// Listens for msg from channel
	Handle()

	ResourceCleaner
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o ../mocks/mock_resource_cleaner.go ResourceCleaner

type ResourceCleaner interface {
	CleanUp(pullContext jobmodels.PullContext)
}

func NewAsyncProjectCommandOutputHandler(
	projectCmdOutput chan *jobmodels.ProjectCmdOutputLine,
	logger logging.SimpleLogging,
) ProjectCommandOutputHandler {
	return &AsyncProjectCommandOutputHandler{
		projectCmdOutput:     projectCmdOutput,
		logger:               logger,
		receiverBuffers:      map[string]map[chan string]bool{},
		projectOutputBuffers: map[string]jobmodels.OutputBuffer{},
		pullToJobMapping:     sync.Map{},
	}
}

func (p *AsyncProjectCommandOutputHandler) Send(ctx models.ProjectCommandContext, msg string, operationComplete bool) {
	p.projectCmdOutput <- &jobmodels.ProjectCmdOutputLine{
		JobID: ctx.JobID,
		JobContext: jobmodels.JobContext{
			HeadCommit: ctx.Pull.HeadCommit,
			PullContext: jobmodels.PullContext{
				PullNum:     ctx.Pull.Num,
				Repo:        ctx.BaseRepo.Name,
				ProjectName: ctx.ProjectName,
				Workspace:   ctx.Workspace,
			},
		},
		Line:              msg,
		OperationComplete: operationComplete,
	}
}

func (p *AsyncProjectCommandOutputHandler) Register(jobID string, receiver chan string) {
	p.addChan(receiver, jobID)
}

func (p *AsyncProjectCommandOutputHandler) Handle() {
	for msg := range p.projectCmdOutput {
		if msg.OperationComplete {
			p.completeJob(msg.JobID)
			continue
		}

		// Add job to pullToJob mapping
		if _, ok := p.pullToJobMapping.Load(msg.JobContext.PullContext); !ok {
			p.pullToJobMapping.Store(msg.JobContext.PullContext, map[string]bool{})
		}
		value, _ := p.pullToJobMapping.Load(msg.JobContext.PullContext)
		jobMapping := value.(map[string]bool)
		jobMapping[msg.JobID] = true

		// Forward new message to all receiver channels and output buffer
		p.writeLogLine(msg.JobID, msg.Line)
	}
}

func (p *AsyncProjectCommandOutputHandler) completeJob(jobID string) {
	p.projectOutputBuffersLock.Lock()
	p.receiverBuffersLock.Lock()
	defer func() {
		p.projectOutputBuffersLock.Unlock()
		p.receiverBuffersLock.Unlock()
	}()

	// Update operation status to complete
	if outputBuffer, ok := p.projectOutputBuffers[jobID]; ok {
		outputBuffer.OperationComplete = true
		p.projectOutputBuffers[jobID] = outputBuffer
	}

	// Close active receiver channels
	if openChannels, ok := p.receiverBuffers[jobID]; ok {
		for ch := range openChannels {
			close(ch)
		}
	}

}

func (p *AsyncProjectCommandOutputHandler) addChan(ch chan string, jobID string) {
	p.projectOutputBuffersLock.RLock()
	outputBuffer := p.projectOutputBuffers[jobID]
	p.projectOutputBuffersLock.RUnlock()

	for _, line := range outputBuffer.Buffer {
		ch <- line
	}

	// No need register receiver since all the logs have been streamed
	if outputBuffer.OperationComplete {
		close(ch)
		return
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

	p.projectOutputBuffersLock.Lock()
	if _, ok := p.projectOutputBuffers[jobID]; !ok {
		p.projectOutputBuffers[jobID] = jobmodels.OutputBuffer{
			Buffer: []string{},
		}
	}
	outputBuffer := p.projectOutputBuffers[jobID]
	outputBuffer.Buffer = append(outputBuffer.Buffer, line)
	p.projectOutputBuffers[jobID] = outputBuffer

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

func (p *AsyncProjectCommandOutputHandler) GetProjectOutputBuffer(jobID string) jobmodels.OutputBuffer {
	return p.projectOutputBuffers[jobID]
}

func (p *AsyncProjectCommandOutputHandler) GetJobIdMapForPullContext(pullContext jobmodels.PullContext) map[string]bool {
	if value, ok := p.pullToJobMapping.Load(pullContext); ok {
		return value.(map[string]bool)
	}
	return nil
}

func (p *AsyncProjectCommandOutputHandler) CleanUp(pullContext jobmodels.PullContext) {
	if value, ok := p.pullToJobMapping.Load(pullContext); ok {
		jobMapping := value.(map[string]bool)
		for jobID := range jobMapping {
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

		// Remove job mapping
		p.pullToJobMapping.Delete(pullContext)
	}
}

// NoopProjectOutputHandler is a mock that doesn't do anything
type NoopProjectOutputHandler struct{}

func (p *NoopProjectOutputHandler) Send(ctx models.ProjectCommandContext, msg string, isOperationComplete bool) {
}

func (p *NoopProjectOutputHandler) Register(jobID string, receiver chan string)   {}
func (p *NoopProjectOutputHandler) Deregister(jobID string, receiver chan string) {}

func (p *NoopProjectOutputHandler) Handle() {
}

func (p *NoopProjectOutputHandler) CleanUp(pullContext jobmodels.PullContext) {
}
