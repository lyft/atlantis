package temporal

import (
	"github.com/runatlantis/atlantis/server/neptune/context"
	"go.temporal.io/sdk/log"
)

// Logger creates an instance of a logger with default kvs
// that are merged dynamically.
type Logger struct {
	kvs      []interface{}
	delegate log.Logger
}

func newLogger(ctx context.KVStore, delegate log.Logger) *Logger {
	return &Logger{
		kvs:      context.ExtractFieldsAsList(ctx),
		delegate: delegate,
	}
}

func (l *Logger) Debug(msg string, keyvals ...interface{}) {
	l.delegate.Debug(msg, append(l.kvs, keyvals)...)
}

func (l *Logger) Info(msg string, keyvals ...interface{}) {
	l.delegate.Info(msg, append(l.kvs, keyvals)...)
}

func (l *Logger) Warn(msg string, keyvals ...interface{}) {
	l.delegate.Warn(msg, append(l.kvs, keyvals)...)
}

func (l *Logger) Error(msg string, keyvals ...interface{}) {
	l.delegate.Error(msg, append(l.kvs, keyvals)...)
}
