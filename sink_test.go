package env_logger

import (
	"bytes"
	"sync"
	"testing"

	logrus "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type lineCollector struct {
	mu    sync.Mutex
	lines []LogLine
}

func (c *lineCollector) add(line LogLine) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, line)
}

func (c *lineCollector) messages() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var messages []string
	for _, line := range c.lines {
		messages = append(messages, line.Message)
	}
	return messages
}

func sinkTestLogger(buffer *bytes.Buffer) *logrus.Logger {
	logger := logrus.New()
	logger.Out = buffer
	logger.Formatter = &logrus.TextFormatter{DisableColors: true}
	return logger
}

func hookCount(logger *logrus.Logger) int {
	hooks := logger.ReplaceHooks(make(logrus.LevelHooks))
	logger.ReplaceHooks(hooks)
	count := 0
	for _, list := range hooks {
		count += len(list)
	}
	return count
}

// A sink gets what it asks for, the output only what the config asks for
func TestSinkGetsDebugOutputStaysInfo(t *testing.T) {
	defer SetGlobalDebugConfig("")
	var buffer bytes.Buffer
	ConfigureAllLoggers(sinkTestLogger(&buffer), "info")
	held := GetLoggerForPrefix("held") // built before the sink exists

	var c lineCollector
	remove := AddSink(logrus.DebugLevel, c.add)

	Debug("pkg-debug")
	Info("pkg-info")
	held.Debug("held-debug")
	held.WithField("k", "v").Debug("held-field-debug")
	Trace("trace")
	held.Trace("held-trace")

	assert.Equal(t, []string{"pkg-debug", "pkg-info", "held-debug", "held-field-debug"}, c.messages())
	assert.Equal(t, "held", c.lines[2].Module)
	assert.Equal(t, "v", c.lines[3].Fields["k"])
	assert.Equal(t, logrus.DebugLevel, c.lines[0].Level)
	assert.Equal(t, 1, bytes.Count(buffer.Bytes(), []byte("\n")))
	assert.Contains(t, buffer.String(), "msg=pkg-info")

	remove()
	remove() // a second call does nothing

	Debug("after")
	Info("after-info")
	assert.Len(t, c.lines, 4)
}

// Removing the last sink restores the snapshot and the hooks exactly
func TestSinkRemoveRestores(t *testing.T) {
	defer SetGlobalDebugConfig("")
	var buffer bytes.Buffer
	logger := sinkTestLogger(&buffer)
	owner := &countingHook{}
	logger.AddHook(owner)
	ConfigureAllLoggers(logger, "info,pkg=warn")
	before := *activeSet.Load()

	var a, b lineCollector
	removeA := AddSink(logrus.DebugLevel, a.add)
	removeB := AddSink(logrus.TraceLevel, b.add)
	set := activeSet.Load()
	assert.Equal(t, logrus.TraceLevel, set.sinkLevel)
	assert.Equal(t, logrus.TraceLevel, set.maxLevel)
	assert.Equal(t, logrus.InfoLevel, set.outLevel)
	assert.Equal(t, logrus.InfoLevel, logger.GetLevel())
	assert.Equal(t, 2*len(logrus.AllLevels), hookCount(logger))

	Trace("trace")
	Debug("debug")
	assert.Equal(t, []string{"debug"}, a.messages())
	assert.Equal(t, []string{"trace", "debug"}, b.messages())
	assert.Empty(t, buffer.String())
	assert.Equal(t, 0, owner.count)

	removeB()
	assert.Equal(t, logrus.DebugLevel, activeSet.Load().sinkLevel)
	removeA()

	after := activeSet.Load()
	assert.Equal(t, before.maxLevel, after.maxLevel)
	assert.Equal(t, before.outLevel, after.outLevel)
	assert.Equal(t, logrus.PanicLevel, after.sinkLevel)
	assert.Equal(t, before.levels, after.levels)
	assert.Equal(t, len(logrus.AllLevels), hookCount(logger))
	assert.Nil(t, hookedLogger)

	Info("info")
	assert.Equal(t, 1, owner.count)
}

// A new logger takes the sink hook over from the old one
func TestSinkSurvivesReconfigure(t *testing.T) {
	defer SetGlobalDebugConfig("")
	var first, second bytes.Buffer
	firstLogger := sinkTestLogger(&first)
	ConfigureAllLoggers(firstLogger, "info")

	var c lineCollector
	remove := AddSink(logrus.DebugLevel, c.add)
	defer remove()

	secondLogger := sinkTestLogger(&second)
	ConfigureAllLoggers(secondLogger, "warn")
	assert.Equal(t, 0, hookCount(firstLogger))
	assert.Equal(t, len(logrus.AllLevels), hookCount(secondLogger))

	Info("info")
	Debug("debug")
	assert.Equal(t, []string{"info", "debug"}, c.messages())
	assert.Empty(t, second.String())

	// reconfiguring the active logger keeps the hook as well
	reconfigureActiveLogger("debug")
	Debug("debug2")
	assert.Equal(t, []string{"info", "debug", "debug2"}, c.messages())
	assert.Contains(t, second.String(), "msg=debug2")
	assert.Equal(t, len(logrus.AllLevels), hookCount(secondLogger))
}

// Lines that pass the output gate reach the sink once, not twice
func TestSinkNoDuplicates(t *testing.T) {
	defer SetGlobalDebugConfig("")
	var buffer bytes.Buffer
	ConfigureAllLoggers(sinkTestLogger(&buffer), "debug")

	var c lineCollector
	remove := AddSink(logrus.DebugLevel, c.add)
	defer remove()

	Debug("debug")
	GetLoggerForPrefix("p").Debug("prefixed")
	assert.Equal(t, []string{"debug", "prefixed"}, c.messages())
	assert.Equal(t, 2, bytes.Count(buffer.Bytes(), []byte("\n")))
}

func TestSinkConcurrent(t *testing.T) {
	defer SetGlobalDebugConfig("")
	var buffer bytes.Buffer
	ConfigureAllLoggers(sinkTestLogger(&buffer), "info")

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					Debug("x")
					GetLoggerForPrefix("p").Debug("y")
				}
			}
		}()
	}
	for i := 0; i < 50; i++ {
		var c lineCollector
		AddSink(logrus.DebugLevel, c.add)()
	}
	close(stop)
	wg.Wait()
	assert.Nil(t, sinks.Load())
}
