package env_logger

import (
	"io"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	logrus "github.com/sirupsen/logrus"
)

// LogLine is a log statement as handed to a sink.
type LogLine struct {
	Time    time.Time
	Level   logrus.Level
	Module  string
	Message string
	// Fields are the fields of the statement, including module and, if
	// enabled, file and routines. The map belongs to this one statement and
	// must not be modified.
	Fields logrus.Fields
}

type sink struct {
	level logrus.Level
	fn    func(LogLine)
}

// sinks is the current list of sinks. It is replaced, never modified, under
// configMu, so the hook can read it without a lock.
var sinks atomic.Pointer[[]*sink]

// AddSink makes every log statement up to level, from every package, also be
// passed to fn, until the returned remove function is called.
//
// The level of a sink is independent of the debug config: a sink at
// DebugLevel gets debug statements even under LOG=info, without them being
// written to the output. To not cost anything while nobody listens, the
// statements that are only wanted by a sink are only produced while that sink
// exists.
//
// fn is called synchronously by the goroutine that logs, so it must be quick,
// must not block and must not log itself.
func AddSink(level logrus.Level, fn func(LogLine)) (remove func()) {
	s := &sink{level: level, fn: fn}

	configMu.Lock()
	var list []*sink
	if current := sinks.Load(); current != nil {
		list = slices.Clone(*current)
	}
	list = append(list, s)
	sinks.Store(&list)
	applySinksLocked()
	configMu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			configMu.Lock()
			defer configMu.Unlock()

			list := slices.DeleteFunc(slices.Clone(*sinks.Load()), func(other *sink) bool { return other == s })
			if len(list) == 0 {
				sinks.Store(nil)
			} else {
				sinks.Store(&list)
			}
			applySinksLocked()
		})
	}
}

// sinkLevelLocked returns the most permissive level any sink wants, or
// PanicLevel if there are no sinks. Caller must hold configMu.
func sinkLevelLocked() logrus.Level {
	level := logrus.PanicLevel
	if list := sinks.Load(); list != nil {
		for _, s := range *list {
			level = max(level, s.level)
		}
	}
	return level
}

// applySinksLocked republishes the active snapshot for the current sinks.
// Caller must hold configMu.
func applySinksLocked() {
	next := *activeSet.Load()
	publishSet(&next)
	hookSinksLocked(next.logger)
}

// hookedLogger is the logger sinkHook is added to, if any. Guarded by
// configMu.
var hookedLogger *logrus.Logger

// hookSinksLocked makes sure sinkHook is on logger exactly while there are
// sinks. A logrus hook costs an allocation per statement even when it does
// nothing, so it is removed again once the last sink is gone.
// Caller must hold configMu.
func hookSinksLocked(logger *logrus.Logger) {
	want := sinks.Load() != nil
	if hookedLogger != nil && (hookedLogger != logger || !want) {
		unhook(hookedLogger)
		hookedLogger = nil
	}
	if want && hookedLogger == nil {
		logger.AddHook(sinkHook{})
		hookedLogger = logger
	}
}

// unhook removes sinkHook from logger, keeping its other hooks. logrus has no
// way to remove a single hook, so the hooks are swapped out and back in;
// hooks of the owner fire on neither of the two swaps.
func unhook(logger *logrus.Logger) {
	hooks := logger.ReplaceHooks(make(logrus.LevelHooks))
	kept := make(logrus.LevelHooks, len(hooks))
	for level, list := range hooks {
		for _, hook := range list {
			if _, ours := hook.(sinkHook); !ours {
				kept[level] = append(kept[level], hook)
			}
		}
	}
	logger.ReplaceHooks(kept)
}

// sinkHook passes the statements of the logger it is on to the sinks.
type sinkHook struct{}

func (sinkHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (sinkHook) Fire(entry *logrus.Entry) error {
	list := sinks.Load()
	if list == nil {
		return nil
	}
	module, _ := entry.Data["module"].(string)
	line := LogLine{
		Time:    entry.Time,
		Level:   entry.Level,
		Module:  module,
		Message: entry.Message,
		Fields:  entry.Data,
	}
	for _, s := range *list {
		if line.Level <= s.level {
			s.fn(line)
		}
	}
	return nil
}

// sinkLogger takes the statements that only sinks want. It writes nothing,
// the statements reach the sinks through its hook.
var sinkLogger = newSinkLogger()

func newSinkLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	logger.SetFormatter(discardFormatter{})
	logger.SetLevel(logrus.TraceLevel)
	logger.SetNoLock()
	logger.AddHook(sinkHook{})
	return logger
}

// toSink returns a copy of e that logs to sinkLogger.
func toSink(e *logrus.Entry) *logrus.Entry {
	bound := *e
	bound.Logger = sinkLogger
	return &bound
}

type discardFormatter struct{}

func (discardFormatter) Format(*logrus.Entry) ([]byte, error) {
	return nil, nil
}
