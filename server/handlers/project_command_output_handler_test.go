package handlers_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/handlers"
	"github.com/runatlantis/atlantis/server/logging"
	. "github.com/runatlantis/atlantis/testing"
	"github.com/stretchr/testify/assert"
)

func createProjectCommandOutputHandler(t *testing.T) handlers.ProjectCommandOutputHandler {
	logger := logging.NewNoopLogger(t)
	prjCmdOutputChan := make(chan *models.ProjectCmdOutputLine)
	return handlers.NewProjectCommandOutputHandler(prjCmdOutputChan, logger)
}

func TestProjectCommandOutputHandler(t *testing.T) {

	REPO_NAME := "test-repo"
	REPO_OWNER := "test-org"
	REPO_BASE_BRANCH := "master"
	USER := "test-user"
	WORKSPACE := "myworkspace"
	REPO_DIR := "test-dir"
	PROJECT_NAME := "test-project"
	MSG := "Test Terraform Output"

	logger := logging.NewNoopLogger(t)
	ctx := models.ProjectCommandContext{
		BaseRepo: models.Repo{
			Name:  REPO_NAME,
			Owner: REPO_OWNER,
		},
		HeadRepo: models.Repo{
			Name:  REPO_NAME,
			Owner: REPO_OWNER,
		},
		Pull: models.PullRequest{
			Num:        1,
			HeadBranch: REPO_BASE_BRANCH,
			BaseBranch: REPO_BASE_BRANCH,
			Author:     USER,
		},
		User: models.User{
			Username: USER,
		},
		Log:         logger,
		Workspace:   WORKSPACE,
		RepoRelDir:  REPO_DIR,
		ProjectName: PROJECT_NAME,
	}

	t.Run("Should Receive Message Sent in the ProjectCmdOutput channel", func(t *testing.T) {
		var wg sync.WaitGroup
		var expectedMsg string

		projectOutputHandler := createProjectCommandOutputHandler(t)
		go func() {
			projectOutputHandler.Handle()
		}()

		wg.Add(1)
		ch := make(chan string)
		go func() {
			err := projectOutputHandler.Receive(ctx.PullInfo(), ch, func(msg string) error {
				expectedMsg = msg
				wg.Done()
				return nil
			})
			Ok(t, err)
		}()

		projectOutputHandler.Send(ctx, MSG)

		// Wait for the msg to be read.
		wg.Wait()
		Equals(t, expectedMsg, MSG)
	})

	t.Run("Should Clear ProjectOutputBuffer when new Plan", func(t *testing.T) {
		var wg sync.WaitGroup

		projectOutputHandler := createProjectCommandOutputHandler(t)
		go func() {
			projectOutputHandler.Handle()
		}()

		wg.Add(1)
		ch := make(chan string)
		go func() {
			err := projectOutputHandler.Receive(ctx.PullInfo(), ch, func(msg string) error {
				wg.Done()
				return nil
			})
			Ok(t, err)
		}()

		projectOutputHandler.Send(ctx, MSG)

		// Wait for the msg to be read.
		wg.Wait()

		// Send a clear msg
		projectOutputHandler.Clear(ctx)

		// Wait for the clear msg to be received by handle()
		time.Sleep(1 * time.Second)
		assert.Empty(t, projectOutputHandler.GetProjectOutputBuffer(ctx.PullInfo()))
	})

	t.Run("Should Cleanup receiverBuffers receiving WS channel closed", func(t *testing.T) {
		var wg sync.WaitGroup

		projectOutputHandler := createProjectCommandOutputHandler(t)
		go func() {
			projectOutputHandler.Handle()
		}()

		wg.Add(1)
		ch := make(chan string)
		go func() {
			err := projectOutputHandler.Receive(ctx.PullInfo(), ch, func(msg string) error {
				wg.Done()
				return nil
			})
			Ok(t, err)
		}()

		projectOutputHandler.Send(ctx, MSG)

		// Wait for the msg to be read.
		wg.Wait()

		// Close chan to execute cleanup.
		close(ch)
		time.Sleep(1 * time.Second)

		x := projectOutputHandler.GetReceiverBufferForPull(ctx.PullInfo())
		assert.Empty(t, x)
	})

	t.Run("Should copy over existing log messages to new WS channels", func(t *testing.T) {
		var wg sync.WaitGroup

		projectOutputHandler := createProjectCommandOutputHandler(t)
		go func() {
			projectOutputHandler.Handle()
		}()

		wg.Add(1)
		ch := make(chan string)
		go func() {
			err := projectOutputHandler.Receive(ctx.PullInfo(), ch, func(msg string) error {
				fmt.Println(msg)
				wg.Done()
				return nil
			})
			Ok(t, err)
		}()

		projectOutputHandler.Send(ctx, MSG)

		// Wait for the msg to be read.
		wg.Wait()

		// Close channel to close prev connection.
		// This should close the first go routine with receive call.
		close(ch)

		ch = make(chan string)

		// Expecting two calls to callback.
		wg.Add(2)

		expectedMsg := []string{}
		go func() {
			err := projectOutputHandler.Receive(ctx.PullInfo(), ch, func(msg string) error {
				expectedMsg = append(expectedMsg, msg)
				wg.Done()
				return nil
			})
			Ok(t, err)
		}()

		// Make sure addChan gets the buffer lock and adds ch to the map.
		time.Sleep(1 * time.Second)

		projectOutputHandler.Send(ctx, MSG)

		// Wait for the message to be read.
		wg.Wait()
		close(ch)
		assert.Equal(t, []string{MSG, MSG}, expectedMsg)
	})
}
