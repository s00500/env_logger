package env_logger

import (
	"runtime"
	"os"
	"strings"
	logrus "github.com/sirupsen/logrus"
)

var (
	internalLogger = logrus.New()
	defaultLogger *logrus.Logger
	loggers = make(map[string]*logrus.Logger)
)

const (
	DebugV = iota
	InfoV = iota
	WarnV = iota
)

type Logger interface {
        // New()  Logger // used to instantiate a new logger
	Debug(...interface{})
	Info(...interface{})
	Warn(...interface{})
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
func ConfigureInternalLogger(newInternalLogger *logrus.Logger)  {
	internalLogger = newInternalLogger
}

// ConfigureDefaultLogger instantiates a default logger instance
func ConfigureDefaultLogger()  {
    defaultLogger = logrus.New()
    ConfigureLogger(defaultLogger)
}

// ConfigureLogger takes in a prefix and a logger object and configures the logger depending on environment variables. 
// Configured based on the GOLANG_DEBUG environment variable
func ConfigureLogger(newDefaultLogger *logrus.Logger)  {
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
func getPackage () string {

    // we get the callers as uintptrs - but we just need 1
    fpcs := make([]uintptr, 1)

    // skip 4 levels to get to the caller of whoever called getPackage()
    n := runtime.Callers(4, fpcs)
    if n == 0 {
       return "" // proper error her would be better
    }

    // get the info of the actual function that's in the pointer
    fun := runtime.FuncForPC(fpcs[0]-1)
    if fun == nil {
      return ""
    }

    name := fun.Name()
    // return its name
	return strings.Split(name, ".")[0]
}

type F func(Logger)
func printLog(f F) {
	pkg := getPackage()
	internalLogger.Debug("pkg: ", pkg)
	if log, ok := loggers[pkg]; ok {
		f(log)
		return
	}
	f(defaultLogger)
}

// Warn prints a warning...
func Warn(args ...interface{})  {
	lambda := func(log Logger) {
		log.Warn(args...)
	}
	printLog(lambda)
}

func Info(args ...interface{})  {
	lambda := func(log Logger) {
		log.Info(args...)
	}
	printLog(lambda)
}

func Debug(args ...interface{})  {
	lambda := func(log Logger) {
		log.Debug(args...)
	}
	printLog(lambda)
}

func Fatal(args ...interface{})  {
	lambda := func(log Logger) {
		log.Fatal(args...)
	}
	printLog(lambda)
}
