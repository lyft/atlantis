package websocket_test

import (
	"fmt"
	"net/http"
	"testing"

	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/controllers/websocket"
	"github.com/runatlantis/atlantis/server/controllers/websocket/mocks"
	"github.com/runatlantis/atlantis/server/controllers/websocket/mocks/matchers"
	"github.com/runatlantis/atlantis/server/logging"
	. "github.com/runatlantis/atlantis/testing"
	"github.com/stretchr/testify/assert"
)

func setupTestArtifacts(t *testing.T) (*mocks.MockPartitionRegistry, *mocks.MockStorageBackendReader, *websocket.Multiplexor, *mocks.MockPartitionKeyGenerator, *mocks.MockWriter) {
	partitionRegistry := mocks.NewMockPartitionRegistry()
	storageBackendReader := mocks.NewMockStorageBackendReader()
	keyGenerator := mocks.NewMockPartitionKeyGenerator()
	writer := mocks.NewMockWriter()
	mux := websocket.NewMultiplexor(logging.NewNoopLogger(t), keyGenerator, partitionRegistry, storageBackendReader, writer)

	return partitionRegistry, storageBackendReader, mux, keyGenerator, writer
}

func TestHandle(t *testing.T) {
	RegisterMockTestingT(t)
	t.Run("No job id", func(t *testing.T) {
		_, _, mux, keyGenerator, _ := setupTestArtifacts(t)
		expectedErrString := "generating partition key: internal error: no job-id in route"

		req, err := http.NewRequest("GET", "/jobs", nil)
		Ok(t, err)

		When(keyGenerator.Generate(req)).ThenReturn("", fmt.Errorf("internal error: no job-id in route"))
		err = mux.Handle(nil, req)

		assert.EqualError(t, err, expectedErrString)
	})

	t.Run("Key not in storage backend and partition registry", func(t *testing.T) {
		partitionRegistry, storageBackendReader, mux, keyGenerator, _ := setupTestArtifacts(t)
		key := "1234"

		req, err := http.NewRequest("GET", fmt.Sprintf("jobs/%s", key), nil)
		Ok(t, err)

		When(storageBackendReader.IsKeyExists(key)).ThenReturn(false)
		When(partitionRegistry.IsKeyExists(key)).ThenReturn(false)
		When(keyGenerator.Generate(req)).ThenReturn(key, nil)

		expectedErrorString := fmt.Sprintf("invalid key: %s", key)

		err = mux.Handle(nil, req)
		assert.EqualError(t, err, expectedErrorString)

	})

	t.Run("Key in storage backend", func(t *testing.T) {
		_, storageBackendReader, mux, keyGenerator, writer := setupTestArtifacts(t)
		key := "1234"

		req, err := http.NewRequest("GET", fmt.Sprintf("/jobs/%s", key), nil)
		Ok(t, err)

		When(keyGenerator.Generate(req)).ThenReturn(key, nil)
		When(storageBackendReader.IsKeyExists(key)).ThenReturn(true)
		When(storageBackendReader.Read(key)).ThenReturn(mocks.NewMockReadCloser())

		When(writer.WriteFromReader(matchers.AnyHttpResponseWriter(), matchers.AnyPtrToHttpRequest(), matchers.AnyIoReadCloser())).ThenReturn(nil)

		err = mux.Handle(mocks.NewMockResponseWriter(), req)
		Ok(t, err)

		storageBackendReader.VerifyWasCalledOnce().Read(key)
		writer.VerifyWasCalledOnce().WriteFromReader(matchers.AnyHttpResponseWriter(), matchers.AnyPtrToHttpRequest(), matchers.AnyIoReadCloser())
	})

	t.Run("Key in parition registry", func(t *testing.T) {
		registry, storageBackendReader, mux, keyGenerator, writer := setupTestArtifacts(t)
		key := "1234"

		req, err := http.NewRequest("GET", fmt.Sprintf("/jobs/%s", key), nil)
		Ok(t, err)

		When(keyGenerator.Generate(req)).ThenReturn(key, nil)
		When(storageBackendReader.IsKeyExists(key)).ThenReturn(false)
		When(registry.IsKeyExists(key)).ThenReturn(true)

		When(writer.WriteFromChan(matchers.AnyHttpResponseWriter(), matchers.AnyPtrToHttpRequest(), matchers.AnyChanOfString())).ThenReturn(nil)

		err = mux.Handle(mocks.NewMockResponseWriter(), req)
		Ok(t, err)

		writer.VerifyWasCalledOnce().WriteFromChan(matchers.AnyHttpResponseWriter(), matchers.AnyPtrToHttpRequest(), matchers.AnyChanOfString())
	})
}
