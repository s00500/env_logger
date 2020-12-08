package env_logger

import (
	"os"
	"runtime"
	"strings"

	logrus "github.com/sirupsen/logrus"
)

var (
	internalLogger = logrus.New()
	defaultLogger  *logrus.Logger
	loggers        = make(map[string]*logrus.Logger)
)

const (
	TraceV = iota
	DebugV = iota
	InfoV  = iota
	WarnV  = iota
	ErrV   = iota
)

func toEnum(s string) int {
	switch strings.ToLower(s) {
	case "trace":
		return TraceV
	case "warn":
		return WarnV
	case "debug":
		return DebugV
	case "info":
		return InfoV
	case "error":
		return ErrV
	default:
		return InfoV
	}
}

func configurePackageLogger(log *logrus.Logger, value int) *logrus.Logger {
	switch value {
	case ErrV:
		log.SetLevel(logrus.ErrorLevel)
	case WarnV:
		log.SetLevel(logrus.WarnLevel)
	case InfoV:
		log.SetLevel(logrus.InfoLevel)
	case DebugV:
		log.SetLevel(logrus.DebugLevel)
	case TraceV:
		log.SetLevel(logrus.TraceLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
	}
	return log
}

// ConfiguredefaultLogger instantiates a default logger instance
func ConfigureInternalLogger(newInternalLogger *logrus.Logger) {
	internalLogger = newInternalLogger
}

func init() {
	defaultLogger = logrus.New()
	ConfigureLogger(defaultLogger)
}

// GetLoggerForPrefix gets the logger for a certain prefix if it has been configured
func GetLoggerForPrefix(prefix string) logrus.FieldLogger {
	if logger, ok := loggers[prefix]; ok {
		return logger.WithFields(logrus.Fields{"module": prefix})
	}
	return defaultLogger.WithFields(logrus.Fields{"module": prefix})
}

// SetLevel sets the default loggers level
func SetLevel(level logrus.Level) {
	defaultLogger.SetLevel(level)
}

// ConfigureLogger takes in a prefix and a logger object and configures the logger depending on environment variables.
// Configured based on the GOLANG_DEBUG environment variable
func ConfigureLogger(newdefaultLogger *logrus.Logger) {
	levels := make(map[string]int)

	if debugRaw, ok := os.LookupEnv("GOLANG_LOG"); ok {
		packages := strings.Split(debugRaw, ",")

		for _, pkg := range packages {
			// check if a package name has been specified, if not default to main
			tmp := strings.Split(pkg, "=")
			if len(tmp) == 1 {
				levels["main"] = toEnum(tmp[0])
			} else if len(tmp) == 2 {
				levels[tmp[0]] = toEnum(tmp[1])
			} else {
				newdefaultLogger.Fatal("line: '", pkg, "' is formatted incorrectly, please refer to the documentation for correct usage")
			}
		}
	}

	for key, value := range levels {
		loggers[key] = configurePackageLogger(logrus.New(), value)
	}

	// configure main logger
	if value, ok := loggers["main"]; ok {
		defaultLogger = value
	} else {
		defaultLogger = newdefaultLogger
	}
}

// Props to https://stackoverflow.com/a/35213181 for the code
func getPackage() string {

	// we get the callers as uintptrs - but we just need 1
	fpcs := make([]uintptr, 1)

	// skip 4 levels to get to the caller of whoever called getPackage()
	n := runtime.Callers(4, fpcs)
	if n == 0 {
		return "" // proper error her would be better
	}

	// get the info of the actual function that's in the pointer
	fun := runtime.FuncForPC(fpcs[0] - 1)
	if fun == nil {
		return ""
	}

	name := fun.Name()
	lastSlash := strings.LastIndex(name, "/") + 1
	firstPoint := strings.Index(name[lastSlash:], ".")
	// return its name
	return name[0 : lastSlash+firstPoint]
}

func getLogger() *logrus.Entry {
	pkg := getPackage()
	internalLogger.Debug("pkg: ", pkg)
	if log, ok := loggers[pkg]; ok {
		return log.WithFields(logrus.Fields{"module": pkg})
	}
	return defaultLogger.WithFields(logrus.Fields{"module": pkg})
}

func WithField(key string, value interface{}) *logrus.Entry {
	return getLogger().WithField(key, value)
}

func WithFields(fields logrus.Fields) *logrus.Entry {
	return getLogger().WithFields(fields)
}

func WithError(err error) *logrus.Entry {
	return getLogger().WithError(err)
}

// Warn prints a warning...
func Warn(args ...interface{}) {
	getLogger().Warn(args...)
}

func Warnln(args ...interface{}) {
	getLogger().Warnln(args...)
}

func Warnf(format string, args ...interface{}) {
	getLogger().Warnf(format, args...)
}

func Info(args ...interface{}) {
	getLogger().Info(args...)
}

func Infoln(args ...interface{}) {
	getLogger().Infoln(args...)
}

func Infof(format string, args ...interface{}) {
	getLogger().Infof(format, args...)
}

func Trace(args ...interface{}) {
	getLogger().Trace(args...)
}

func Traceln(args ...interface{}) {
	getLogger().Traceln(args...)
}

func Tracef(format string, args ...interface{}) {
	getLogger().Tracef(format, args...)
}

func Debug(args ...interface{}) {
	getLogger().Debug(args...)
}

func Debugln(args ...interface{}) {
	getLogger().Debugln(args...)
}

func Debugf(format string, args ...interface{}) {
	getLogger().Debugf(format, args...)
}

func Print(args ...interface{}) {
	getLogger().Print(args...)
}

func Println(args ...interface{}) {
	getLogger().Println(args...)
}

func Printf(format string, args ...interface{}) {
	getLogger().Printf(format, args...)
}

func Error(args ...interface{}) {
	getLogger().Error(args...)
}

func Errorf(format string, args ...interface{}) {
	getLogger().Errorf(format, args...)
}

func Errorln(args ...interface{}) {
	getLogger().Errorln(args...)
}

func Fatal(args ...interface{}) {
	getLogger().Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	getLogger().Fatalf(format, args...)
}

func Fatalln(args ...interface{}) {
	getLogger().Fatalln(args...)
}

func Panic(args ...interface{}) {
	getLogger().Fatal(args...)
}

func Panicf(format string, args ...interface{}) {
	getLogger().Panicf(format, args...)
}

func Panicln(args ...interface{}) {
	getLogger().Panicln(args...)
}
