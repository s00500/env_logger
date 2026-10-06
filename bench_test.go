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
