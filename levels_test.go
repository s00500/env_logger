package env_logger_test

import (
	"bytes"
	"sync"
	"testing"

	env_logger "github.com/s00500/env_logger"
	logrus "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func bufferLogger(buffer *bytes.Buffer) *logrus.Logger {
	logger := logrus.New()
	logger.Out = buffer
	logger.Formatter = &logrus.TextFormatter{DisableColors: true}
	return logger
}

func TestSetLevelRaisesVerbosity(t *testing.T) {
	var buffer bytes.Buffer
	env_logger.ConfigureAllLoggers(bufferLogger(&buffer), "")

	env_logger.Debug("before")
	assert.Empty(t, buffer.String())

	env_logger.SetLevel(logrus.DebugLevel)
	env_logger.Debug("after")
	assert.Contains(t, buffer.String(), "msg=after")

	buffer.Reset()
	env_logger.SetLevel(logrus.WarnLevel)
	env_logger.Info("hidden")
	assert.Empty(t, buffer.String())
}

// SetLevel on the logger that was passed in has to keep working as well
func TestSetLevelOnPassedLogger(t *testing.T) {
	var buffer bytes.Buffer
	logger := bufferLogger(&buffer)
	env_logger.ConfigureAllLoggers(logger, "pkg=warn")

	logger.SetLevel(logrus.DebugLevel)
	env_logger.Debug("after")
	assert.Contains(t, buffer.String(), "msg=after")

	buffer.Reset()
	env_logger.GetLoggerForPrefix("pkg").Info("hidden")
	assert.Empty(t, buffer.String())

	logger.SetLevel(logrus.ErrorLevel)
	env_logger.Warn("hidden")
	assert.Empty(t, buffer.String())
}

func TestPerPackageLevels(t *testing.T) {
	var buffer bytes.Buffer
	env_logger.ConfigureAllLoggers(bufferLogger(&buffer), "verbose=debug,quiet=warn")

	verbose := env_logger.GetLoggerForPrefix("verbose")
	quiet := env_logger.GetLoggerForPrefix("quiet")
	other := env_logger.GetLoggerForPrefix("other")

	verbose.Debug("verbose-debug")
	verbose.Trace("verbose-trace")
	verbose.WithField("k", "v").Debug("verbose-field-debug")
	quiet.Info("quiet-info")
	quiet.WithField("k", "v").Info("quiet-field-info")
	quiet.Warn("quiet-warn")
	other.Debug("other-debug")
	other.Info("other-info")
	env_logger.Debug("caller-debug")

	out := buffer.String()
	assert.Contains(t, out, "msg=verbose-debug")
	assert.Contains(t, out, "msg=verbose-field-debug")
	assert.Contains(t, out, "msg=quiet-warn")
	assert.Contains(t, out, "msg=other-info")
	assert.NotContains(t, out, "verbose-trace")
	assert.NotContains(t, out, "quiet-info")
	assert.NotContains(t, out, "quiet-field-info")
	assert.NotContains(t, out, "other-debug")
	assert.NotContains(t, out, "caller-debug")
}

// The level a logger was created with is the default again once the config
// no longer names one, also when the same logger is configured repeatedly
func TestReconfigureSameLoggerKeepsOwnLevel(t *testing.T) {
	var buffer bytes.Buffer
	logger := bufferLogger(&buffer)
	logger.SetLevel(logrus.WarnLevel)

	env_logger.ConfigureAllLoggers(logger, "trace,pkg=debug")
	env_logger.Trace("shown")
	assert.Contains(t, buffer.String(), "msg=shown")

	buffer.Reset()
	env_logger.ConfigureAllLoggers(logger, "")
	env_logger.Info("hidden")
	assert.Empty(t, buffer.String())
	env_logger.Warn("warned")
	assert.Contains(t, buffer.String(), "msg=warned")
}

type countHook struct {
	mu    sync.Mutex
	count int
}

func (h *countHook) Levels() []logrus.Level { return logrus.AllLevels }

func (h *countHook) Fire(*logrus.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.count++
	return nil
}

// Hooks of the logger passed in fire no matter what the config sets
func TestHooksSurviveLevelConfig(t *testing.T) {
	var buffer bytes.Buffer
	for _, cfg := range []string{"", "debug", "pkg=debug", "info,pkg=trace"} {
		logger := bufferLogger(&buffer)
		hook := &countHook{}
		logger.AddHook(hook)
		env_logger.ConfigureAllLoggers(logger, cfg)

		env_logger.Info("default")
		env_logger.GetLoggerForPrefix("pkg").Info("pkg")
		assert.Equal(t, 2, hook.count, "config %q", cfg)
	}
}

// unsyncedWriter is a writer that is only safe when writes are serialized
type unsyncedWriter struct {
	data []byte
}

func (w *unsyncedWriter) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

// TestPerPackageSharedOutputRace checks that packages with their own level
// still serialize their writes to the shared output. Must be run with -race
// to be meaningful.
func TestPerPackageSharedOutputRace(t *testing.T) {
	const lines = 500

	writer := &unsyncedWriter{}
	logger := logrus.New()
	logger.Out = writer
	env_logger.ConfigureAllLoggers(logger, "a=debug,b=trace")

	var wg sync.WaitGroup
	for _, pkg := range []string{"a", "b", "c"} {
		wg.Add(1)
		go func(pkg string) {
			defer wg.Done()
			entry := env_logger.GetLoggerForPrefix(pkg)
			for i := 0; i < lines; i++ {
				entry.Info("hello")
			}
		}(pkg)
	}
	wg.Wait()

	assert.Equal(t, 3*lines, bytes.Count(writer.data, []byte("\n")))
}
