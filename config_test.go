package env_logger_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"

	env_logger "github.com/s00500/env_logger"
	logrus "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func jsonLogger(buffer *bytes.Buffer) *logrus.Logger {
	logger := logrus.New()
	logger.Out = buffer
	logger.Formatter = new(logrus.JSONFormatter)
	return logger
}

// logLines decodes what a jsonLogger has written, one Fields per line
func logLines(t *testing.T, buffer *bytes.Buffer) []logrus.Fields {
	var lines []logrus.Fields
	for _, line := range bytes.Split(bytes.TrimSpace(buffer.Bytes()), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var fields logrus.Fields
		require.NoError(t, json.Unmarshal(line, &fields))
		lines = append(lines, fields)
	}
	return lines
}

// A config that can not be understood must neither end the program nor take
// the understandable parts down with it
func TestMalformedConfigIsIgnored(t *testing.T) {
	var buffer bytes.Buffer
	logger := jsonLogger(&buffer)
	logger.ExitFunc = func(int) { t.Error("config must not exit the program") }

	env_logger.ConfigureAllLoggers(logger, "a=b=c,pkg=dbg,=debug,verbose,mut=x,ppport=70000,good=debug")

	lines := logLines(t, &buffer)
	require.Len(t, lines, 6)
	for _, line := range lines {
		assert.Equal(t, "warning", line["level"])
		assert.Equal(t, "env_logger", line["module"])
		assert.Contains(t, line["msg"], "ignoring log config")
	}

	buffer.Reset()
	env_logger.GetLoggerForPrefix("good").Debug("good-debug")
	env_logger.GetLoggerForPrefix("pkg").Debug("pkg-debug")
	env_logger.GetLoggerForPrefix("pkg").Info("pkg-info")
	lines = logLines(t, &buffer)
	require.Len(t, lines, 2)
	assert.Equal(t, "good-debug", lines[0]["msg"])
	assert.Equal(t, "pkg-info", lines[1]["msg"])
}

func TestConfigWhitespaceAndLevelNames(t *testing.T) {
	var buffer bytes.Buffer
	env_logger.ConfigureAllLoggers(jsonLogger(&buffer), " warning , a = debug,, b=TRACE ,")
	assert.Empty(t, buffer.String(), "valid config must not be reported")

	env_logger.GetLoggerForPrefix("a").Debug("a-debug")
	env_logger.GetLoggerForPrefix("a").Trace("a-trace")
	env_logger.GetLoggerForPrefix("b").Trace("b-trace")
	env_logger.GetLoggerForPrefix("c").Info("c-info")
	env_logger.GetLoggerForPrefix("c").Warn("c-warn")

	lines := logLines(t, &buffer)
	require.Len(t, lines, 3)
	assert.Equal(t, "a-debug", lines[0]["msg"])
	assert.Equal(t, "b-trace", lines[1]["msg"])
	assert.Equal(t, "c-warn", lines[2]["msg"])
}

// Entries that are held across a reconfigure follow the new config
func TestHeldEntryFollowsReconfigure(t *testing.T) {
	var before, after bytes.Buffer
	env_logger.ConfigureAllLoggers(jsonLogger(&before), "info")
	held := env_logger.GetLoggerForPrefix("held")
	derived := held.WithField("k", "v")

	env_logger.ConfigureAllLoggers(jsonLogger(&after), "debug")
	held.Debug("held-debug")
	derived.Debug("derived-debug")
	held.WithField("k2", "v2").Info("rederived-info")

	assert.Empty(t, before.String())
	lines := logLines(t, &after)
	require.Len(t, lines, 3)
	assert.Equal(t, "held-debug", lines[0]["msg"])
	assert.Equal(t, "held", lines[0]["module"])
	assert.Equal(t, "derived-debug", lines[1]["msg"])
	assert.Equal(t, "v", lines[1]["k"])
	assert.Equal(t, "rederived-info", lines[2]["msg"])

	after.Reset()
	env_logger.ConfigureAllLoggers(jsonLogger(&before), "held=error")
	held.Warn("held-warn")
	derived.Warn("derived-warn")
	assert.Empty(t, before.String())
	assert.Empty(t, after.String())
}

func TestLineNumbersPerCallSite(t *testing.T) {
	var buffer bytes.Buffer
	env_logger.ConfigureAllLoggers(jsonLogger(&buffer), "ln")
	entry := env_logger.GetLoggerForPrefix("pkg")

	_, _, line, _ := runtime.Caller(0)
	for i := 0; i < 2; i++ {
		env_logger.Info("first")
		env_logger.Info("second")
		entry.Info("entry")
		entry.WithField("k", "v").Info("derived")
	}

	lines := logLines(t, &buffer)
	require.Len(t, lines, 8)
	for i, fields := range lines {
		assert.Contains(t, fields["file"], fmt.Sprintf("config_test.go:%d'", line+2+i%4), fields["msg"])
	}

	// and gone again once the config no longer asks for it
	buffer.Reset()
	env_logger.ConfigureAllLoggers(jsonLogger(&buffer), "")
	env_logger.Info("plain")
	entry.Info("entry")
	env_logger.EnableLineNumbers()
	env_logger.Info("enabled")

	lines = logLines(t, &buffer)
	require.Len(t, lines, 3)
	assert.NotContains(t, lines[0], "file")
	assert.NotContains(t, lines[1], "file")
	assert.Contains(t, lines[2]["file"], "config_test.go:")
}

func TestGetLoggerForPackage(t *testing.T) {
	var buffer bytes.Buffer
	env_logger.ConfigureAllLoggers(jsonLogger(&buffer), "")

	env_logger.Info("function")
	env_logger.GetLoggerForPackage().Info("entry")

	lines := logLines(t, &buffer)
	require.Len(t, lines, 2)
	assert.NotEmpty(t, lines[0]["module"])
	assert.Equal(t, lines[0]["module"], lines[1]["module"])

	assert.Contains(t, env_logger.ListModules(), lines[0]["module"])
}

func TestListModules(t *testing.T) {
	var buffer bytes.Buffer
	env_logger.ConfigureAllLoggers(jsonLogger(&buffer), "")
	env_logger.GetLoggerForPrefix("listed").Info("hi")

	modules := env_logger.ListModules()
	assert.Contains(t, modules, "listed")
	assert.IsIncreasing(t, modules)
}

// The panic is reported for the code that panicked, not for PanicHandler
func TestPanicHandlerReportsPanicSite(t *testing.T) {
	var buffer bytes.Buffer
	env_logger.ConfigureAllLoggers(jsonLogger(&buffer), "ln")

	env_logger.Info("reference")
	for _, cause := range []func(){
		func() { panic("boom") },
		func() { var m map[string]int; m["boom"] = 1 },
	} {
		func() {
			defer func() { assert.NotNil(t, recover(), "panic has to be passed on") }()
			defer env_logger.PanicHandler()
			cause()
		}()
	}
	func() {
		defer func() { assert.NotNil(t, recover(), "panic has to be passed on") }()
		defer env_logger.GetLoggerForPrefix("pkg").PanicHandler()
		panic("boom")
	}()

	lines := logLines(t, &buffer)
	require.Len(t, lines, 4)
	for _, fields := range lines[1:] {
		assert.Equal(t, "panic", fields["level"])
		assert.Contains(t, fields["file"], "config_test.go:")
	}
	assert.Equal(t, lines[0]["module"], lines[1]["module"])
	assert.Equal(t, lines[0]["module"], lines[2]["module"])
	assert.Equal(t, "pkg", lines[3]["module"])
}

// mut= is undone once the config no longer contains it
func TestMutexProfileFractionIsReset(t *testing.T) {
	var buffer bytes.Buffer
	current := func() int { return runtime.SetMutexProfileFraction(-1) }
	before := current()

	env_logger.ConfigureAllLoggers(jsonLogger(&buffer), "mut=7")
	assert.Equal(t, 7, current())
	env_logger.ConfigureAllLoggers(jsonLogger(&buffer), "mut=9")
	assert.Equal(t, 9, current())
	env_logger.ConfigureAllLoggers(jsonLogger(&buffer), "")
	assert.Equal(t, before, current())
}

func TestManyPrefixes(t *testing.T) {
	var buffer bytes.Buffer
	env_logger.ConfigureAllLoggers(jsonLogger(&buffer), "")

	const prefixes = 3000
	for i := 0; i < prefixes; i++ {
		env_logger.GetLoggerForPrefix(fmt.Sprintf("conn-%d", i)).Info("hi")
	}

	lines := logLines(t, &buffer)
	require.Len(t, lines, prefixes)
	assert.Equal(t, "conn-0", lines[0]["module"])
	assert.Equal(t, fmt.Sprintf("conn-%d", prefixes-1), lines[prefixes-1]["module"])
}
