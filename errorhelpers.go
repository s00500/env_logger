package env_logger

// Must Checks if an error occured, otherwise panic
func Must(err error) {
	if err != nil {
		getLogger(nil).Panicf("Error on must: %v", err)
	}
}

// MustFatal Checks if an error occured, otherwise stop the program
func MustFatal(err error) {
	if err != nil {
		getLogger(nil).Fatalf("Fatal Error: %v", err)
	}
}

// Should Checks if an error occured, otherwise prints it as error, returns true if error is not nil
func Should(err error) bool {
	if err != nil {
		getLogger(nil).Error(err)
		return true
	}
	return false
}

// Should Checks if an error occured, otherwise prints it as error, returns true if error is not nil
func ShouldWrap(err error, msg string, args ...interface{}) bool {
	if err != nil {
		getLogger(nil).Error(Wrap(err, msg, args...))
		return true
	}
	return false
}

// ShouldWarn Checks if an error occured, otherwise prints it as warning, returns true if error is not nil
func ShouldWarn(err error) bool {
	if err != nil {
		getLogger(nil).Warn(err)
		return true
	}
	return false
}

// Must Checks if an error occured, otherwise panic
func (e *Entry) Must(err error) {
	if err != nil {
		getLogger(e).Panicf("Error on must: %v", err)
	}
}

// MustFatal Checks if an error occured, otherwise stop the program
func (e *Entry) MustFatal(err error) {
	if err != nil {
		getLogger(e).Fatalf("Fatal Error: %v", err)
	}
}

// ShouldWrap Checks if an error occured, otherwise prints it as error, returns true if error is not nil
func (e *Entry) ShouldWrap(err error, msg string, args ...interface{}) bool {
	if err != nil {
		getLogger(e).Error(Wrap(err, msg, args...))
		return true
	}
	return false
}

// Should Checks if an error occured, otherwise prints it as error, returns true if error is not nil
func (e *Entry) Should(err error) bool {
	if err != nil {
		// Should get the linenumbers and goroutines!
		getLogger(e).Error(err)
		return true
	}
	return false
}

// ShouldWarn Checks if an error occured, otherwise prints it as warning, returns true if error is not nil
func (e *Entry) ShouldWarn(err error) bool {
	if err != nil {
		getLogger(e).Warn(err)
		return true
	}
	return false
}
