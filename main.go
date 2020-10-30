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
	DebugV = iota
	InfoV  = iota
	WarnV  = iota
)

type Logger interface {
	// New()  Logger // used to instantiate a new logger

	Tracef(format string, args ...interface{})
	Traceln(...interface{})
	Trace(...interface{})

	Printf(format string, args ...interface{})
	Println(...interface{})
	Print(...interface{})

	Debugf(format string, args ...interface{})
	Debugln(...interface{})
	Debug(...interface{})

	Infof(format string, args ...interface{})
	Infoln(...interface{})
	Info(...interface{})

	Warnf(format string, args ...interface{})
	Warnln(...interface{})
	Warn(...interface{})

	Errorf(format string, args ...interface{})
	Errorln(...interface{})
	Error(...interface{})

	Panicf(format string, args ...interface{})
	Panicln(...interface{})
	Panic(...interface{})

	Fatalf(format string, args ...interface{})
	Fatalln(...interface{})
	Fatal(...interface{})
}

func toEnum(s string) int {
	switch strings.ToLower(s) {
	case "warn":
		return WarnV
	case "debug":
		return DebugV
	case "info":
		return InfoV
	default:
		return InfoV
	}

}

func configurePackageLogger(log *logrus.Logger, value int) *logrus.Logger {
	switch value {
	case WarnV:
		log.SetLevel(logrus.WarnLevel)
	case InfoV:
		log.SetLevel(logrus.InfoLevel)
	case DebugV:
		log.SetLevel(logrus.DebugLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
	}
	return log
}

// ConfigureDefaultLogger instantiates a default logger instance
func ConfigureInternalLogger(newInternalLogger *logrus.Logger) {
	internalLogger = newInternalLogger
}

// ConfigureDefaultLogger instantiates a default logger instance
func ConfigureDefaultLogger() {
	defaultLogger = logrus.New()
	ConfigureLogger(defaultLogger)
}

// ConfigureLogger takes in a prefix and a logger object and configures the logger depending on environment variables.
// Configured based on the GOLANG_DEBUG environment variable
func ConfigureLogger(newDefaultLogger *logrus.Logger) {
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
				newDefaultLogger.Fatal("line: '", pkg, "' is formatted incorrectly, please refer to the documentation for correct usage")
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
		defaultLogger = newDefaultLogger
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

type F func(Logger)

func printLog(f F) {
	pkg := getPackage()
	internalLogger.Debug("pkg: ", pkg)
	if log, ok := loggers[pkg]; ok {
		f(log.WithFields(logrus.Fields{"module": pkg}))
		return
	}
	f(defaultLogger.WithFields(logrus.Fields{"module": pkg}))
}

// Warn prints a warning...
func Warn(args ...interface{}) {
	lambda := func(log Logger) {
		log.Warn(args...)
	}
	printLog(lambda)
}

func Warnln(args ...interface{}) {
	lambda := func(log Logger) {
		log.Warnln(args...)
	}
	printLog(lambda)
}

func Warnf(format string, args ...interface{}) {
	lambda := func(log Logger) {
		log.Warnf(format, args...)
	}
	printLog(lambda)
}

func Info(args ...interface{}) {
	lambda := func(log Logger) {
		log.Info(args...)
	}
	printLog(lambda)
}

func Infoln(args ...interface{}) {
	lambda := func(log Logger) {
		log.Infoln(args...)
	}
	printLog(lambda)
}

func Infof(format string, args ...interface{}) {
	lambda := func(log Logger) {
		log.Infof(format, args...)
	}
	printLog(lambda)
}

func Trace(args ...interface{}) {
	lambda := func(log Logger) {
		log.Trace(args...)
	}
	printLog(lambda)
}

func Traceln(args ...interface{}) {
	lambda := func(log Logger) {
		log.Traceln(args...)
	}
	printLog(lambda)
}

func Tracef(format string, args ...interface{}) {
	lambda := func(log Logger) {
		log.Tracef(format, args...)
	}
	printLog(lambda)
}

func Debug(args ...interface{}) {
	lambda := func(log Logger) {
		log.Debug(args...)
	}
	printLog(lambda)
}

func Debugln(args ...interface{}) {
	lambda := func(log Logger) {
		log.Debugln(args...)
	}
	printLog(lambda)
}

func Debugf(format string, args ...interface{}) {
	lambda := func(log Logger) {
		log.Debugf(format, args...)
	}
	printLog(lambda)
}

func Print(args ...interface{}) {
	lambda := func(log Logger) {
		log.Print(args...)
	}
	printLog(lambda)
}

func Println(args ...interface{}) {
	lambda := func(log Logger) {
		log.Println(args...)
	}
	printLog(lambda)
}

func Printf(format string, args ...interface{}) {
	lambda := func(log Logger) {
		log.Printf(format, args...)
	}
	printLog(lambda)
}

func Error(args ...interface{}) {
	lambda := func(log Logger) {
		log.Error(args...)
	}
	printLog(lambda)
}

func Errorf(format string, args ...interface{}) {
	lambda := func(log Logger) {
		log.Errorf(format, args...)
	}
	printLog(lambda)
}

func Errorln(args ...interface{}) {
	lambda := func(log Logger) {
		log.Errorln(args...)
	}
	printLog(lambda)
}

func Fatal(args ...interface{}) {
	lambda := func(log Logger) {
		log.Fatal(args...)
	}
	printLog(lambda)
}

func Fatalf(format string, args ...interface{}) {
	lambda := func(log Logger) {
		log.Fatalf(format, args...)
	}
	printLog(lambda)
}

func Fatalln(args ...interface{}) {
	lambda := func(log Logger) {
		log.Fatalln(args...)
	}
	printLog(lambda)
}

func Panic(args ...interface{}) {
	lambda := func(log Logger) {
		log.Fatal(args...)
	}
	printLog(lambda)
}

func Panicf(format string, args ...interface{}) {
	lambda := func(log Logger) {
		log.Panicf(format, args...)
	}
	printLog(lambda)
}

func Panicln(args ...interface{}) {
	lambda := func(log Logger) {
		log.Panicln(args...)
	}
	printLog(lambda)
}
