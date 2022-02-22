package env_logger

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/mattn/go-colorable"
	logrus "github.com/sirupsen/logrus"
)

var (
	defaultLogger *logrus.Logger
	loggers       = make(map[string]*logrus.Logger)
)

// Pass through type to not have another import in packages using this lib
type Fields logrus.Fields

type Entry logrus.Entry

const (
	TraceV = iota
	DebugV = iota
	InfoV  = iota
	WarnV  = iota
	ErrV   = iota
	FatalV = iota
	PanicV = iota
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
	case "fatal":
		return FatalV
	case "panic":
		return PanicV
	default:
		return InfoV
	}
}

func configurePackageLogger(log *logrus.Logger, value int) *logrus.Logger {
	switch value {
	case PanicV:
		log.SetLevel(logrus.PanicLevel)
	case FatalV:
		log.SetLevel(logrus.FatalLevel)
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

var filelines = false
var printGoRoutines = false
var mainModuleName = ""

func init() {
	logger := logrus.New()
	debugConfig, _ := os.LookupEnv("LOG")
	if debugConfig == "" {
		debugConfig, _ = os.LookupEnv("GOLANG_LOG")
	}

	logger.Formatter.(*logrus.TextFormatter).EnvironmentOverrideColors = true
	logger.SetOutput(colorable.NewColorableStdout()) // make default work on windows
	ConfigureAllLoggers(logger, debugConfig)

	info, ok := debug.ReadBuildInfo()
	if ok {
		mainModuleName = info.Path
	}
}

// EnableLineNumbers log output of linenumbers as logerus fields
func EnableLineNumbers() {
	filelines = true
}

// GetLoggerForPrefix gets the logger for a certain prefix if it has been configured
func GetLoggerForPrefix(prefix string) *Entry {
	if logger, ok := loggers[prefix]; ok {
		return (*Entry)(logger.WithFields(logrus.Fields{"module": prefix}))
	}
	return (*Entry)((defaultLogger.WithFields(logrus.Fields{"module": prefix})))
}

// SetLevel sets the default loggers level
func SetLevel(level logrus.Level) {
	defaultLogger.SetLevel(level)
}

// ConfigureLogger takes in a logger object and configures the logger depending on environment variables.
// Configured based on the GOLANG_DEBUG environment variable
func ConfigureAllLoggers(newdefaultLogger *logrus.Logger, debugConfig string) {
	levels := make(map[string]int)

	startProfileServer := false
	if debugConfig != "" {
		packages := strings.Split(debugConfig, ",")

		for _, pkg := range packages {
			// check if a package name has been specified, if not default to main
			tmp := strings.Split(pkg, "=")
			if len(tmp) == 1 && tmp[0] == "ln" {
				filelines = true
			} else if len(tmp) == 1 && tmp[0] == "pp" { // pprof
				startProfileServer = true
			} else if len(tmp) == 1 && tmp[0] == "gr" { // go routine log
				printGoRoutines = true
			} else if len(tmp) == 1 && tmp[0] == "grl" { // go routine loop
				printGoRoutines = true
				go logGoRoutines()
			} else if len(tmp) == 1 {
				levels["global_log"] = toEnum(tmp[0])
			} else if len(tmp) == 2 {
				levels[tmp[0]] = toEnum(tmp[1])
			} else {
				newdefaultLogger.Fatal("line: '", pkg, "' is formatted incorrectly, please refer to the documentation for correct usage")
			}
		}
	}

	for key, value := range levels {
		// Copy some properties of the default logger
		pLogger := logrus.New()
		pLogger.Out = newdefaultLogger.Out
		pLogger.Formatter = newdefaultLogger.Formatter
		loggers[key] = configurePackageLogger(pLogger, value)
	}

	// configure main logger
	if value, ok := loggers["global_log"]; ok {
		defaultLogger = value
	} else {
		defaultLogger = newdefaultLogger
	}
	if startProfileServer {
		go profileServer()
	}
}

// Props to https://stackoverflow.com/a/35213181 for the code
func getPackage() (string, string, int) {

	// we get the callers as uintptrs - but we just need 1
	fpcs := make([]uintptr, 1)

	// skip 4 levels to get to the caller of whoever called getPackage()
	n := runtime.Callers(4, fpcs)
	if n == 0 {
		return "", "", 0 // proper error her would be better
	}

	// get the info of the actual function that's in the pointer
	fun := runtime.FuncForPC(fpcs[0] - 1)
	if fun == nil {
		return "", "", 0
	}

	name := fun.Name()
	firstSlash := strings.Index(name, "/")
	if firstSlash != -1 {
		if strings.Contains(name[0:firstSlash], ".com") || strings.Contains(name[0:firstSlash], ".org") || strings.Contains(name[0:firstSlash], ".io") {
			// Trim the url
			name = name[firstSlash+1:]
		}
	}

	lastSlash := strings.LastIndex(name, "/") + 1
	firstPoint := strings.Index(name[lastSlash:], ".")

	file, line := fun.FileLine(fpcs[0] - 1)

	if i := strings.Index(file, mainModuleName); i != -1 {
		file = file[i:]
	}

	if i := strings.Index(file, "@"); i != -1 {
		// Trim out the version info in case we run with -trimpath
		nextSlash := strings.Index(file[i:], "/")
		file = file[:i] + file[i+nextSlash:]
	}

	return strings.TrimPrefix(name[0:lastSlash+firstPoint], mainModuleName+"/"), strings.TrimPrefix(file, mainModuleName+"/"), line
}

func getLogger(e *Entry) *logrus.Entry {
	pkg, file, line := getPackage()

	var logentry *logrus.Entry
	if e != nil {
		logentry = (*logrus.Entry)(e)
	} else {
		if log, ok := loggers[pkg]; ok {
			logentry = log.WithFields(logrus.Fields{"module": pkg})
		} else {
			logentry = defaultLogger.WithFields(logrus.Fields{"module": pkg})
		}
	}

	if filelines {
		logentry = logentry.WithFields(logrus.Fields{"file": fmt.Sprintf("'%s:%d'", file, line)})
	}

	if printGoRoutines {
		logentry = logentry.WithFields(logrus.Fields{"routines": runtime.NumGoroutine()})
	}

	return logentry
}

func WithField(key string, value interface{}) *Entry {
	return (*Entry)(getLogger(nil).WithField(key, value))
}

func WithFields(fields logrus.Fields) *Entry {
	return (*Entry)(getLogger(nil).WithFields(fields))
}

func WithError(err error) *Entry {
	return (*Entry)(getLogger(nil).WithError(err))
}

// Warn prints a warning...
func Warn(args ...interface{}) {
	getLogger(nil).Warn(args...)
}

func Warnln(args ...interface{}) {
	getLogger(nil).Warnln(args...)
}

func Warnf(format string, args ...interface{}) {
	getLogger(nil).Warnf(format, args...)
}

func Info(args ...interface{}) {
	getLogger(nil).Info(args...)
}

func Infoln(args ...interface{}) {
	getLogger(nil).Infoln(args...)
}

func Infof(format string, args ...interface{}) {
	getLogger(nil).Infof(format, args...)
}

func Trace(args ...interface{}) {
	getLogger(nil).Trace(args...)
}

func Traceln(args ...interface{}) {
	getLogger(nil).Traceln(args...)
}

func Tracef(format string, args ...interface{}) {
	getLogger(nil).Tracef(format, args...)
}

func Debug(args ...interface{}) {
	getLogger(nil).Debug(args...)
}

func Debugln(args ...interface{}) {
	getLogger(nil).Debugln(args...)
}

func Debugf(format string, args ...interface{}) {
	getLogger(nil).Debugf(format, args...)
}

func Print(args ...interface{}) {
	getLogger(nil).Print(args...)
}

func Println(args ...interface{}) {
	getLogger(nil).Println(args...)
}

func Printf(format string, args ...interface{}) {
	getLogger(nil).Printf(format, args...)
}

func Error(args ...interface{}) {
	getLogger(nil).Error(args...)
}

func Errorf(format string, args ...interface{}) {
	getLogger(nil).Errorf(format, args...)
}

func Errorln(args ...interface{}) {
	getLogger(nil).Errorln(args...)
}

func Fatal(args ...interface{}) {
	getLogger(nil).Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	getLogger(nil).Fatalf(format, args...)
}

func Fatalln(args ...interface{}) {
	getLogger(nil).Fatalln(args...)
}

func Panic(args ...interface{}) {
	getLogger(nil).Panic(args...)
}

func Panicf(format string, args ...interface{}) {
	getLogger(nil).Panicf(format, args...)
}

func Panicln(args ...interface{}) {
	getLogger(nil).Panicln(args...)
}

func Log(level logrus.Level, args ...interface{}) {
	getLogger(nil).Log(level, args...)
}

func Logf(level logrus.Level, format string, args ...interface{}) {
	getLogger(nil).Logf(level, format, args...)
}

func Logln(level logrus.Level, args ...interface{}) {
	getLogger(nil).Logln(level, args...)
}
