package env_logger

import logrus "github.com/sirupsen/logrus"

// Must Checks if an error occured, otherwise panic. Un-gated: panic must fire.
func Must(err error) {
	if err != nil {
		getLogger(nil).Panicf("Error on must: %v", err)
	}
}

// MustFatal Checks if an error occured, otherwise stop the program. Un-gated: os.Exit must fire.
func MustFatal(err error) {
	if err != nil {
		getLogger(nil).Fatalf("Fatal Error: %v", err)
	}
}

// Should Checks if an error occured, otherwise prints it as error, returns true if error is not nil
func Should(err error) bool {
	if err == nil {
		return false
	}
	if l := getLoggerIfLevel(nil, logrus.ErrorLevel); l != nil {
		l.Error(err)
	}
	return true
}

// ShouldWrap Checks if an error occured, otherwise prints it as error, returns true if error is not nil
func ShouldWrap(err error, msg string, args ...interface{}) bool {
	if err == nil {
		return false
	}
	if l := getLoggerIfLevel(nil, logrus.ErrorLevel); l != nil {
		l.Error(Wrap(err, msg, args...))
	}
	return true
}

// ShouldWarn Checks if an error occured, otherwise prints it as warning, returns true if error is not nil
func ShouldWarn(err error) bool {
	if err == nil {
		return false
	}
	if l := getLoggerIfLevel(nil, logrus.WarnLevel); l != nil {
		l.Warn(err)
	}
	return true
}

// Must Checks if an error occured, otherwise panic. Un-gated: panic must fire.
func (e *Entry) Must(err error) {
	if err != nil {
		getLogger(e).Panicf("Error on must: %v", err)
	}
}

// MustFatal Checks if an error occured, otherwise stop the program. Un-gated: os.Exit must fire.
func (e *Entry) MustFatal(err error) {
	if err != nil {
		getLogger(e).Fatalf("Fatal Error: %v", err)
	}
}

// ShouldWrap Checks if an error occured, otherwise prints it as error, returns true if error is not nil
func (e *Entry) ShouldWrap(err error, msg string, args ...interface{}) bool {
	if err == nil {
		return false
	}
	if l := getLoggerIfLevel(e, logrus.ErrorLevel); l != nil {
		l.Error(Wrap(err, msg, args...))
	}
	return true
}

// Should Checks if an error occured, otherwise prints it as error, returns true if error is not nil
func (e *Entry) Should(err error) bool {
	if err == nil {
		return false
	}
	if l := getLoggerIfLevel(e, logrus.ErrorLevel); l != nil {
		l.Error(err)
	}
	return true
}

// ShouldWarn Checks if an error occured, otherwise prints it as warning, returns true if error is not nil
func (e *Entry) ShouldWarn(err error) bool {
	if err == nil {
		return false
	}
	if l := getLoggerIfLevel(e, logrus.WarnLevel); l != nil {
		l.Warn(err)
	}
	return true
}
