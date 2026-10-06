package env_logger

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	logrus "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// A logstring that can not be understood must not take the program down
func TestLogstringEndpoint(t *testing.T) {
	defer SetGlobalDebugConfig("")

	var buffer bytes.Buffer
	hook := &countingHook{}
	logger := logrus.New()
	logger.Out = &buffer
	logger.Formatter = new(logrus.JSONFormatter)
	logger.AddHook(hook)
	logger.SetLevel(logrus.WarnLevel)
	ConfigureAllLoggers(logger, "")
	held := GetLoggerForPrefix("held")

	mux := newProfileMux()

	for _, config := range []string{"a=b=c", "nonsense", "pkg=debug"} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/logstring", strings.NewReader(config)))
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "New log config: "+config, recorder.Body.String())
	}
	assert.Equal(t, logrus.DebugLevel, activeSet.Load().levels["pkg"])

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/logstring", strings.NewReader(strings.Repeat("x", maxLogstringSize+1))))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, logrus.DebugLevel, activeSet.Load().levels["pkg"])

	// The logger the application configured is kept: its output, formatter
	// and hooks, and its own level wherever the logstring does not set one
	assert.Same(t, logger, activeSet.Load().logger)
	buffer.Reset()
	hook.count = 0

	Info("hidden")
	GetLoggerForPrefix("pkg").Debug("pkg-debug")
	assert.Equal(t, 1, hook.count)
	assert.Equal(t, 1, bytes.Count(buffer.Bytes(), []byte("\n")))
	assert.Contains(t, buffer.String(), `"msg":"pkg-debug"`)

	post := func(config string) {
		mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/logstring", strings.NewReader(config)))
	}

	buffer.Reset()
	post("trace")
	Trace("default-trace")
	held.Trace("held-trace")
	assert.Contains(t, buffer.String(), `"msg":"default-trace"`)
	assert.Contains(t, buffer.String(), `"msg":"held-trace"`)

	buffer.Reset()
	post("")
	Info("hidden")
	held.Info("hidden")
	assert.Empty(t, buffer.String())
	Warn("default-warn")
	assert.Contains(t, buffer.String(), `"msg":"default-warn"`)
}

type countingHook struct {
	count int
}

func (h *countingHook) Levels() []logrus.Level { return logrus.AllLevels }

func (h *countingHook) Fire(*logrus.Entry) error {
	h.count++
	return nil
}

// Prefixes made up at runtime must not grow the entry cache forever
func TestModuleEntryCacheIsBounded(t *testing.T) {
	logger := logrus.New()
	logger.Out = &bytes.Buffer{}
	ConfigureAllLoggers(logger, "")

	for i := 0; i < 2*maxModuleEntries; i++ {
		GetLoggerForPrefix(fmt.Sprintf("conn-%d", i))
	}

	cached := 0
	activeSet.Load().moduleEntries.entries.Range(func(_, _ interface{}) bool {
		cached++
		return true
	})
	assert.LessOrEqual(t, cached, maxModuleEntries)
}
