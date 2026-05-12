package env_logger

import logrus "github.com/sirupsen/logrus"

func (e *Entry) WithField(key string, value interface{}) *Entry {
	return (*Entry)(getLogger(e).WithField(key, value))
}

func (e *Entry) WithFields(fields logrus.Fields) *Entry {
	return (*Entry)(getLogger(e).WithFields(fields))
}

func (e *Entry) WithError(err error) *Entry {
	return (*Entry)(getLogger(e).WithError(err))
}

// Warn prints a warning...
func (e *Entry) Warn(args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.WarnLevel); l != nil {
		l.Warn(args...)
	}
}

func (e *Entry) Warnln(args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.WarnLevel); l != nil {
		l.Warnln(args...)
	}
}

func (e *Entry) Warnf(format string, args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.WarnLevel); l != nil {
		l.Warnf(format, args...)
	}
}

func (e *Entry) Info(args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.InfoLevel); l != nil {
		l.Info(args...)
	}
}

func (e *Entry) Infoln(args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.InfoLevel); l != nil {
		l.Infoln(args...)
	}
}

func (e *Entry) Infof(format string, args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.InfoLevel); l != nil {
		l.Infof(format, args...)
	}
}

func (e *Entry) Trace(args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.TraceLevel); l != nil {
		l.Trace(args...)
	}
}

func (e *Entry) Traceln(args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.TraceLevel); l != nil {
		l.Traceln(args...)
	}
}

func (e *Entry) Tracef(format string, args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.TraceLevel); l != nil {
		l.Tracef(format, args...)
	}
}

func (e *Entry) Debug(args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.DebugLevel); l != nil {
		l.Debug(args...)
	}
}

func (e *Entry) Debugln(args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.DebugLevel); l != nil {
		l.Debugln(args...)
	}
}

func (e *Entry) Debugf(format string, args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.DebugLevel); l != nil {
		l.Debugf(format, args...)
	}
}

func (e *Entry) Print(args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.InfoLevel); l != nil {
		l.Print(args...)
	}
}

func (e *Entry) Println(args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.InfoLevel); l != nil {
		l.Println(args...)
	}
}

func (e *Entry) Printf(format string, args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.InfoLevel); l != nil {
		l.Printf(format, args...)
	}
}

func (e *Entry) Error(args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.ErrorLevel); l != nil {
		l.Error(args...)
	}
}

func (e *Entry) Errorf(format string, args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.ErrorLevel); l != nil {
		l.Errorf(format, args...)
	}
}

func (e *Entry) Errorln(args ...interface{}) {
	if l := getLoggerIfLevel(e, logrus.ErrorLevel); l != nil {
		l.Errorln(args...)
	}
}

// Fatal/Panic stay un-gated — see env_logger.go for the same reason.
func (e *Entry) Fatal(args ...interface{}) {
	getLogger(e).Fatal(args...)
}

func (e *Entry) Fatalf(format string, args ...interface{}) {
	getLogger(e).Fatalf(format, args...)
}

func (e *Entry) Fatalln(args ...interface{}) {
	getLogger(e).Fatalln(args...)
}

func (e *Entry) Panic(args ...interface{}) {
	getLogger(e).Panic(args...)
}

func (e *Entry) Panicf(format string, args ...interface{}) {
	getLogger(e).Panicf(format, args...)
}

func (e *Entry) Panicln(args ...interface{}) {
	getLogger(e).Panicln(args...)
}

func (e *Entry) Log(level logrus.Level, args ...interface{}) {
	if level == logrus.FatalLevel || level == logrus.PanicLevel {
		getLogger(e).Log(level, args...)
		return
	}
	if l := getLoggerIfLevel(e, level); l != nil {
		l.Log(level, args...)
	}
}

func (e *Entry) Logf(level logrus.Level, format string, args ...interface{}) {
	if level == logrus.FatalLevel || level == logrus.PanicLevel {
		getLogger(e).Logf(level, format, args...)
		return
	}
	if l := getLoggerIfLevel(e, level); l != nil {
		l.Logf(level, format, args...)
	}
}

func (e *Entry) Logln(level logrus.Level, args ...interface{}) {
	if level == logrus.FatalLevel || level == logrus.PanicLevel {
		getLogger(e).Logln(level, args...)
		return
	}
	if l := getLoggerIfLevel(e, level); l != nil {
		l.Logln(level, args...)
	}
}
