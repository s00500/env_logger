package env_logger

import logrus "github.com/sirupsen/logrus"

func (e *Entry) WithField(key string, value interface{}) *logrus.Entry {
	return getLogger(nil).WithField(key, value)
}

func (e *Entry) WithFields(fields logrus.Fields) *logrus.Entry {
	return getLogger(nil).WithFields(fields)
}

func (e *Entry) WithError(err error) *logrus.Entry {
	return getLogger(nil).WithError(err)
}

// Warn prints a warning...
func (e *Entry) Warn(args ...interface{}) {
	getLogger(e).Warn(args...)
}

func (e *Entry) Warnln(args ...interface{}) {
	getLogger(e).Warnln(args...)
}

func (e *Entry) Warnf(format string, args ...interface{}) {
	getLogger(e).Warnf(format, args...)
}

func (e *Entry) Info(args ...interface{}) {
	getLogger(e).Info(args...)
}

func (e *Entry) Infoln(args ...interface{}) {
	getLogger(e).Infoln(args...)
}

func (e *Entry) Infof(format string, args ...interface{}) {
	getLogger(e).Infof(format, args...)
}

func (e *Entry) Trace(args ...interface{}) {
	getLogger(e).Trace(args...)
}

func (e *Entry) Traceln(args ...interface{}) {
	getLogger(e).Traceln(args...)
}

func (e *Entry) Tracef(format string, args ...interface{}) {
	getLogger(e).Tracef(format, args...)
}

func (e *Entry) Debug(args ...interface{}) {
	getLogger(e).Debug(args...)
}

func (e *Entry) Debugln(args ...interface{}) {
	getLogger(e).Debugln(args...)
}

func (e *Entry) Debugf(format string, args ...interface{}) {
	getLogger(e).Debugf(format, args...)
}

func (e *Entry) Print(args ...interface{}) {
	getLogger(e).Print(args...)
}

func (e *Entry) Println(args ...interface{}) {
	getLogger(e).Println(args...)
}

func (e *Entry) Printf(format string, args ...interface{}) {
	getLogger(e).Printf(format, args...)
}

func (e *Entry) Error(args ...interface{}) {
	getLogger(e).Error(args...)
}

func (e *Entry) Errorf(format string, args ...interface{}) {
	getLogger(e).Errorf(format, args...)
}

func (e *Entry) Errorln(args ...interface{}) {
	getLogger(e).Errorln(args...)
}

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
	getLogger(e).Log(level, args...)
}

func (e *Entry) Logf(level logrus.Level, format string, args ...interface{}) {
	getLogger(e).Logf(level, format, args...)
}

func (e *Entry) Logln(level logrus.Level, args ...interface{}) {
	getLogger(e).Logln(level, args...)
}
