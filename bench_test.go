package env_logger_test

import (
	"io"
	"testing"

	env_logger "github.com/s00500/env_logger"
	logrus "github.com/sirupsen/logrus"
)

func silentLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return l
}

// BenchmarkDebugFiltered: Debug() while level=Info. The level early-out
// should turn this into ~one atomic load + level comparison + return.
func BenchmarkDebugFiltered(b *testing.B) {
	env_logger.ConfigureAllLoggers(silentLogger(), "info")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env_logger.Debug("hello world")
	}
}

// BenchmarkInfoEmitted: Info() while level=Info. Pays the full caller
// resolution (cached) + WithFields + format + write-to-Discard cost.
func BenchmarkInfoEmitted(b *testing.B) {
	env_logger.ConfigureAllLoggers(silentLogger(), "info")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env_logger.Info("hello world")
	}
}

// BenchmarkDebugEmitted: Debug() while level=Debug. All paths fire.
func BenchmarkDebugEmitted(b *testing.B) {
	env_logger.ConfigureAllLoggers(silentLogger(), "debug")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env_logger.Debug("hello world")
	}
}

// BenchmarkEntryInfoEmitted: pre-built entry + emit. The e != nil branch
// skips getPackage entirely when filelines/printGoRoutines are off.
func BenchmarkEntryInfoEmitted(b *testing.B) {
	env_logger.ConfigureAllLoggers(silentLogger(), "info")
	entry := env_logger.WithField("k", "v")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry.Info("hello world")
	}
}

// BenchmarkEntryDebugFiltered: pre-built entry, filtered level. Should
// short-circuit on the entry's logger.IsLevelEnabled check alone.
func BenchmarkEntryDebugFiltered(b *testing.B) {
	env_logger.ConfigureAllLoggers(silentLogger(), "info")
	entry := env_logger.WithField("k", "v")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry.Debug("hello world")
	}
}

// BenchmarkDebugFilteredOtherPkgDebug: Debug() from a package at Info while
// another package is at Trace. The global gate passes, so the caller frame
// has to be resolved before the per-package gate can drop the call.
func BenchmarkDebugFilteredOtherPkgDebug(b *testing.B) {
	env_logger.ConfigureAllLoggers(silentLogger(), "info,other=trace")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env_logger.Debug("hello world")
	}
}

// BenchmarkEntryDebugFilteredOtherPkgDebug: same, but through a pre-built
// entry, which is gated on its module field instead of the caller frame.
func BenchmarkEntryDebugFilteredOtherPkgDebug(b *testing.B) {
	env_logger.ConfigureAllLoggers(silentLogger(), "info,other=trace")
	entry := env_logger.GetLoggerForPrefix("pkg")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry.Debug("hello world")
	}
}

// BenchmarkEntryWithFieldDebugFiltered: adding a field to a pre-built entry
// should not resolve the caller frame when filelines/printGoRoutines are off.
func BenchmarkEntryWithFieldDebugFiltered(b *testing.B) {
	env_logger.ConfigureAllLoggers(silentLogger(), "info")
	entry := env_logger.GetLoggerForPrefix("pkg")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry.WithField("k", i).Debug("hello world")
	}
}

// BenchmarkInfoEmittedParallel: Info() from many goroutines at once.
func BenchmarkInfoEmittedParallel(b *testing.B) {
	env_logger.ConfigureAllLoggers(silentLogger(), "info")
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			env_logger.Info("hello world")
		}
	})
}

// BenchmarkInfoEmittedLineNumbers: Info() with ln on. The file field is part
// of the entry cached for the call site, so it should cost about the same as
// BenchmarkInfoEmitted.
func BenchmarkInfoEmittedLineNumbers(b *testing.B) {
	env_logger.ConfigureAllLoggers(silentLogger(), "info,ln")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env_logger.Info("hello world")
	}
}

// BenchmarkPackageEntryDebugFilteredOtherPkgDebug: like
// BenchmarkDebugFilteredOtherPkgDebug, but through GetLoggerForPackage,
// which does not have to resolve the caller frame.
func BenchmarkPackageEntryDebugFilteredOtherPkgDebug(b *testing.B) {
	env_logger.ConfigureAllLoggers(silentLogger(), "info,other=trace")
	entry := env_logger.GetLoggerForPackage()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry.Debug("hello world")
	}
}

// BenchmarkDebugFilteredAfterSink: like BenchmarkDebugFiltered, after a sink
// was added and removed again. Must cost the same as without ever having one.
func BenchmarkDebugFilteredAfterSink(b *testing.B) {
	env_logger.ConfigureAllLoggers(silentLogger(), "info")
	env_logger.AddSink(logrus.DebugLevel, func(env_logger.LogLine) {})()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env_logger.Debug("hello world")
	}
}

// BenchmarkInfoEmittedAfterSink: like BenchmarkInfoEmitted, after a sink was
// added and removed again, which must leave no hook behind.
func BenchmarkInfoEmittedAfterSink(b *testing.B) {
	env_logger.ConfigureAllLoggers(silentLogger(), "info")
	env_logger.AddSink(logrus.DebugLevel, func(env_logger.LogLine) {})()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env_logger.Info("hello world")
	}
}

// BenchmarkDebugFilteredSinkAtInfo: a sink at info does not want debug
// either, so Debug() stays as cheap as in BenchmarkDebugFiltered.
func BenchmarkDebugFilteredSinkAtInfo(b *testing.B) {
	env_logger.ConfigureAllLoggers(silentLogger(), "info")
	defer env_logger.AddSink(logrus.InfoLevel, func(env_logger.LogLine) {})()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env_logger.Debug("hello world")
	}
}

// BenchmarkInfoEmittedSinkActive: what a statement costs on top of
// BenchmarkInfoEmitted while a sink listens.
func BenchmarkInfoEmittedSinkActive(b *testing.B) {
	env_logger.ConfigureAllLoggers(silentLogger(), "info")
	defer env_logger.AddSink(logrus.InfoLevel, func(env_logger.LogLine) {})()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env_logger.Info("hello world")
	}
}

// BenchmarkDebugSinkOnly: Debug() under LOG=info while a sink wants debug.
// The statement is produced for the sink only, nothing is formatted.
func BenchmarkDebugSinkOnly(b *testing.B) {
	env_logger.ConfigureAllLoggers(silentLogger(), "info")
	defer env_logger.AddSink(logrus.DebugLevel, func(env_logger.LogLine) {})()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env_logger.Debug("hello world")
	}
}
