package testutil

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/go-logr/logr"
)

// Logger implements logr.Logger and logs to testing.T to preserve the order of log lines in tests.
type Logger struct {
	t      *testing.T
	names  []string
	values map[string]interface{}
}

var _ logr.LogSink = (*Logger)(nil)

// NewLogger returns a new Logger flushing to testing.T.
func NewLogger(t *testing.T) logr.Logger {
	t.Helper()
	l := &Logger{
		t:      t,
		values: map[string]interface{}{},
	}
	return logr.New(l)
}

// Info implements logr.LogSink.Info.
func (l *Logger) Info(level int, msg string, kvs ...interface{}) {
	// marks this function as a helper method, so it will be excluded in the log stacktrace
	l.t.Helper()

	values := addValues(l.values, kvs...)

	j, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}
	l.t.Logf("%-15s %-20s %s", strings.Join(l.names, "."), msg, string(j))
}

// Error implements logr.LogSink.Error.
func (l *Logger) Error(err error, msg string, kvs ...interface{}) {
	// marks this function as a helper method, so it will be excluded in the log stacktrace
	l.t.Helper()
	l.Info(1, msg, append(kvs, "error", err.Error())...)
}

// Enabled implements logr.LogSink.Enabled.
func (l *Logger) Enabled(level int) bool {
	return true
}

// Init implements logr.LogSink.Init.
func (l *Logger) Init(info logr.RuntimeInfo) {

}

// WithValues implements logr.LogSink.WithValues.
func (l *Logger) WithValues(kvs ...interface{}) logr.LogSink {
	return &Logger{
		t:      l.t,
		names:  l.names,
		values: addValues(l.values, kvs...),
	}
}

// WithName implements logr.LogSink.WithName.
func (l *Logger) WithName(name string) logr.LogSink {
	return &Logger{
		t:      l.t,
		names:  append(l.names, name),
		values: l.values,
	}
}

func addValues(base map[string]interface{}, kvs ...interface{}) map[string]interface{} {
	values := map[string]interface{}{}
	// add existing k/v pairs
	for k := range base {
		values[k] = base[k]
	}
	// add new k/v pairs
	for i := 0; i < len(kvs); i += 2 {
		values[fmt.Sprint(kvs[i])] = kvs[i+1]
	}
	return values
}
