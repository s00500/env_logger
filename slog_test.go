package env_logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	stdlog "log"
	"log/slog"
	"sync"
	"testing"
	"time"

	logrus "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func slogTestLogger(buffer *bytes.Buffer) *logrus.Logger {
	logger := logrus.New()
	logger.Out = buffer
	logger.Formatter = new(logrus.JSONFormatter)
	return logger
}

// slogLines decodes what a slogTestLogger has written, one map per line
func slogLines(t *testing.T, buffer *bytes.Buffer) []map[string]interface{} {
	t.Helper()
	var lines []map[string]interface{}
	for _, line := range bytes.Split(bytes.TrimSpace(buffer.Bytes()), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var fields map[string]interface{}
		require.NoError(t, json.Unmarshal(line, &fields))
		lines = append(lines, fields)
	}
	return lines
}

type slogValuer struct{}

func (slogValuer) LogValue() slog.Value { return slog.StringValue("resolved") }

func TestSlogLevelMapping(t *testing.T) {
	for level, want := range map[slog.Level]logrus.Level{
		slog.LevelDebug - 8: logrus.TraceLevel,
		slog.LevelDebug - 1: logrus.TraceLevel,
		slog.LevelDebug:     logrus.DebugLevel,
		slog.LevelDebug + 1: logrus.DebugLevel,
		slog.LevelInfo:      logrus.InfoLevel,
		slog.LevelWarn:      logrus.WarnLevel,
		slog.LevelError:     logrus.ErrorLevel,
		slog.LevelError + 8: logrus.ErrorLevel,
	} {
		assert.Equal(t, want, slogLevel(level), "slog level %v", level)
	}

	defer SetGlobalDebugConfig("")
	var buffer bytes.Buffer
	ConfigureAllLoggers(slogTestLogger(&buffer), "trace")
	logger := Slog("lib")
	ctx := context.Background()
	logger.Log(ctx, slog.LevelDebug-4, "t")
	logger.Debug("d")
	logger.Info("i")
	logger.Warn("w")
	logger.Error("e")
	logger.Log(ctx, slog.LevelError+4, "e2")

	var got []string
	for _, line := range slogLines(t, &buffer) {
		got = append(got, line["msg"].(string)+"="+line["level"].(string))
		assert.Equal(t, "lib", line["module"])
	}
	assert.Equal(t, []string{"t=trace", "d=debug", "i=info", "w=warning", "e=error", "e2=error"}, got)
}

// The name of a slog logger is its module in the debug config
func TestSlogPerPackageLevels(t *testing.T) {
	defer SetGlobalDebugConfig("")
	var buffer bytes.Buffer
	ConfigureAllLoggers(slogTestLogger(&buffer), "info,lbsauth=debug,quiet=warn")
	ctx := context.Background()

	auth, other, quiet := Slog("lbsauth"), Slog("other"), Slog("quiet")
	assert.True(t, auth.Enabled(ctx, slog.LevelDebug))
	assert.False(t, auth.Enabled(ctx, slog.LevelDebug-4))
	assert.False(t, other.Enabled(ctx, slog.LevelDebug))
	assert.True(t, other.Enabled(ctx, slog.LevelInfo))
	assert.False(t, quiet.Enabled(ctx, slog.LevelInfo))

	auth.Debug("auth-debug")
	other.Debug("hidden")
	other.Info("other-info")
	quiet.Info("hidden")
	quiet.Warn("quiet-warn")

	lines := slogLines(t, &buffer)
	require.Len(t, lines, 3)
	assert.Equal(t, "auth-debug", lines[0]["msg"])
	assert.Equal(t, "lbsauth", lines[0]["module"])
	assert.Equal(t, "other-info", lines[1]["msg"])
	assert.Equal(t, "quiet-warn", lines[2]["msg"])

	// loggers made before a reconfigure follow it
	buffer.Reset()
	reconfigureActiveLogger("warn,other=debug")
	assert.False(t, auth.Enabled(ctx, slog.LevelInfo))
	auth.Info("hidden")
	other.Debug("other-debug")
	lines = slogLines(t, &buffer)
	require.Len(t, lines, 1)
	assert.Equal(t, "other-debug", lines[0]["msg"])

	// and so does SetLevel
	buffer.Reset()
	SetLevel(logrus.DebugLevel)
	auth.Debug("auth-debug2")
	lines = slogLines(t, &buffer)
	require.Len(t, lines, 1)
	assert.Equal(t, "auth-debug2", lines[0]["msg"])
}

func TestSlogAttrsAndGroups(t *testing.T) {
	defer SetGlobalDebugConfig("")
	var buffer bytes.Buffer
	ConfigureAllLoggers(slogTestLogger(&buffer), "info")

	logger := Slog("lib").With("a", 1, "valuer", slogValuer{}).WithGroup("g").With("b", "two").WithGroup("").WithGroup("h")
	logger.Info("msg",
		"c", 3.5,
		slog.Group("sub", "d", true, slog.Group("deep", "e", time.Second)),
		slog.Group("", "inline", "yes"),
		slog.Group("empty"),
		slog.Attr{},
		"lv", slogValuer{},
	)

	lines := slogLines(t, &buffer)
	require.Len(t, lines, 1)
	line := lines[0]
	assert.Equal(t, "msg", line["msg"])
	assert.Equal(t, "lib", line["module"])
	assert.Equal(t, float64(1), line["a"])
	assert.Equal(t, "resolved", line["valuer"])
	assert.Equal(t, "two", line["g.b"])
	assert.Equal(t, 3.5, line["g.h.c"])
	assert.Equal(t, true, line["g.h.sub.d"])
	assert.Equal(t, float64(time.Second), line["g.h.sub.deep.e"])
	assert.Equal(t, "yes", line["g.h.inline"])
	assert.Equal(t, "resolved", line["g.h.lv"])
	assert.NotContains(t, line, "g.h.empty")
	assert.NotContains(t, line, "")
	// time, level, msg, module and the eight above
	assert.Len(t, line, 12)
}

// Each With makes its own handler, siblings do not see each other's attrs
func TestSlogWithAttrsIsolated(t *testing.T) {
	defer SetGlobalDebugConfig("")
	var c lineCollector
	var buffer bytes.Buffer
	ConfigureAllLoggers(slogTestLogger(&buffer), "info")
	remove := AddSink(logrus.InfoLevel, c.add)
	defer remove()

	base := Slog("lib").With("base", 1)
	first := base.With("first", 1)
	second := base.With("second", 2, "base", 3)
	first.Info("first")
	second.Info("second")
	base.Info("base")

	require.Len(t, c.lines, 3)
	assert.Equal(t, logrus.Fields{"module": "lib", "base": int64(1), "first": int64(1)}, c.lines[0].Fields)
	assert.Equal(t, logrus.Fields{"module": "lib", "base": int64(3), "second": int64(2)}, c.lines[1].Fields)
	assert.Equal(t, logrus.Fields{"module": "lib", "base": int64(1)}, c.lines[2].Fields)
}

func TestSlogErrorFields(t *testing.T) {
	defer SetGlobalDebugConfig("")
	var buffer bytes.Buffer
	ConfigureAllLoggers(slogTestLogger(&buffer), "info")
	var c lineCollector
	remove := AddSink(logrus.InfoLevel, c.add)
	defer remove()

	boom := errors.New("boom")
	Slog("lib").Error("failed", "err", boom, slog.Any(logrus.ErrorKey, Wrap(boom, "wrapped")))

	require.Len(t, c.lines, 1)
	err, ok := c.lines[0].Fields["err"].(error)
	require.True(t, ok, "error values stay errors")
	assert.ErrorIs(t, err, boom)
	err, ok = c.lines[0].Fields[logrus.ErrorKey].(error)
	require.True(t, ok)
	assert.ErrorIs(t, err, boom)

	lines := slogLines(t, &buffer)
	require.Len(t, lines, 1)
	assert.Equal(t, "boom", lines[0]["err"])
	assert.Equal(t, "wrapped: boom", lines[0][logrus.ErrorKey])
}

// Time and context of the record are kept, ln adds the file of the caller
func TestSlogTimeContextAndFile(t *testing.T) {
	defer SetGlobalDebugConfig("")
	var buffer bytes.Buffer
	logger := slogTestLogger(&buffer)
	var gotCtx context.Context
	logger.AddHook(ctxHook(func(e *logrus.Entry) { gotCtx = e.Context }))
	ConfigureAllLoggers(logger, "info,ln")

	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "v")
	when := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	record := slog.NewRecord(when, slog.LevelInfo, "at", 0)
	require.NoError(t, SlogHandler("lib").Handle(ctx, record))
	assert.Equal(t, "v", gotCtx.Value(key{}))
	require.NoError(t, SlogHandler("").Handle(ctx, record))

	Slog("lib").InfoContext(ctx, "with-file")

	lines := slogLines(t, &buffer)
	require.Len(t, lines, 3)
	assert.Equal(t, slogUnknownModule, lines[1]["module"])
	lines = append(lines[:1], lines[2])
	logged, err := time.Parse(time.RFC3339, lines[0]["time"].(string))
	require.NoError(t, err)
	assert.True(t, when.Equal(logged), "logged at %v", logged)
	assert.Equal(t, "lib", lines[0]["module"])
	assert.NotContains(t, lines[0], "file", "a record without caller has no file")
	assert.Contains(t, lines[1]["file"], "slog_test.go:")
	assert.Equal(t, "lib", lines[1]["module"])
}

type ctxHook func(*logrus.Entry)

func (ctxHook) Levels() []logrus.Level { return logrus.AllLevels }

func (h ctxHook) Fire(e *logrus.Entry) error {
	h(e)
	return nil
}

// Sinks get slog statements like any other, including the ones that only
// the sink wants
func TestSlogSinks(t *testing.T) {
	defer SetGlobalDebugConfig("")
	var buffer bytes.Buffer
	ConfigureAllLoggers(slogTestLogger(&buffer), "info")

	logger := Slog("lib")
	assert.False(t, logger.Enabled(context.Background(), slog.LevelDebug))

	var c lineCollector
	remove := AddSink(logrus.DebugLevel, c.add)
	assert.True(t, logger.Enabled(context.Background(), slog.LevelDebug))

	logger.Debug("sink-only", "k", "v")
	logger.Info("both")
	logger.Log(context.Background(), slog.LevelDebug-4, "nobody")

	assert.Equal(t, []string{"sink-only", "both"}, c.messages())
	assert.Equal(t, "lib", c.lines[0].Module)
	assert.Equal(t, "v", c.lines[0].Fields["k"])
	assert.Equal(t, logrus.DebugLevel, c.lines[0].Level)
	lines := slogLines(t, &buffer)
	require.Len(t, lines, 1)
	assert.Equal(t, "both", lines[0]["msg"])

	remove()
	assert.False(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

// The default handler logs as the package of the caller, and takes over the
// standard library's log package as well
func TestSlogDefault(t *testing.T) {
	defer SetGlobalDebugConfig("")
	previous := slog.Default()
	defer slog.SetDefault(previous)

	var buffer bytes.Buffer
	ConfigureAllLoggers(slogTestLogger(&buffer), "info")
	SetSlogDefault()

	Info("plain")
	slog.Info("via-slog", "k", 1)
	stdlog.Print("via-log")
	slog.Debug("hidden")

	lines := slogLines(t, &buffer)
	require.Len(t, lines, 3)
	module := lines[0]["module"].(string)
	require.NotEmpty(t, module)
	assert.Equal(t, "via-slog", lines[1]["msg"])
	assert.Equal(t, module, lines[1]["module"])
	assert.Equal(t, float64(1), lines[1]["k"])
	assert.Equal(t, "via-log", lines[2]["msg"])
	assert.Equal(t, "info", lines[2]["level"])
	assert.Equal(t, module, lines[2]["module"])

	// the package of the caller is what the config filters on
	buffer.Reset()
	reconfigureActiveLogger("info,unrelated=trace")
	slog.Debug("still-hidden")
	assert.Empty(t, buffer.String())
	reconfigureActiveLogger("info," + module + "=debug")
	slog.Debug("shown")
	lines = slogLines(t, &buffer)
	require.Len(t, lines, 1)
	assert.Equal(t, "shown", lines[0]["msg"])
	assert.Equal(t, "debug", lines[0]["level"])

	// and a sink still gets what only it wants
	buffer.Reset()
	reconfigureActiveLogger("info")
	var c lineCollector
	remove := AddSink(logrus.DebugLevel, c.add)
	slog.Debug("sink-only")
	remove()
	assert.Empty(t, buffer.String())
	assert.Equal(t, []string{"sink-only"}, c.messages())
	assert.Equal(t, module, c.lines[0].Module)
}

func TestSlogConcurrent(t *testing.T) {
	defer SetGlobalDebugConfig("")
	var buffer bytes.Buffer
	var mu sync.Mutex
	logger := slogTestLogger(&buffer)
	logger.Out = writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return buffer.Write(p)
	})
	ConfigureAllLoggers(logger, "info")

	base := Slog("lib").With("shared", 1)
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			own := base.WithGroup("g").With("i", i)
			for {
				select {
				case <-stop:
					return
				default:
					own.Debug("d", "k", "v")
					own.Info("i", "err", errors.New("x"))
					slog.New(SlogHandler("")).Info("auto")
				}
			}
		}(i)
	}
	for i := 0; i < 20; i++ {
		var c lineCollector
		remove := AddSink(logrus.DebugLevel, c.add)
		reconfigureActiveLogger("info,lib=debug,ln")
		remove()
		reconfigureActiveLogger("info")
	}
	close(stop)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	lines := slogLines(t, &buffer)
	assert.NotEmpty(t, lines)
	for _, line := range lines {
		assert.NotEmpty(t, line["module"])
		assert.Contains(t, []string{"d", "i", "auto"}, line["msg"])
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }
