package env_logger

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	"sync/atomic"

	"github.com/mattn/go-colorable"
	logrus "github.com/sirupsen/logrus"
)

// loggerSet is an immutable snapshot of the active logger configuration.
// Published atomically via activeSet so the hot path is a single atomic load.
type loggerSet struct {
	defaultLogger *logrus.Logger
	loggers       map[string]*logrus.Logger
}

var activeSet atomic.Pointer[loggerSet]

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

var (
	filelines       atomic.Bool
	printGoRoutines atomic.Bool
	mainModuleName  string // written only from init(), then read-only
)

func init() {
	logger := logrus.New()
	debugConfig, _ := os.LookupEnv("LOG")
	if debugConfig == "" {
		debugConfig, _ = os.LookupEnv("GOLANG_LOG")
	}
	//logger.Formatter = &textformatter.TextFormatter{}
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
	filelines.Store(true)
}

// GetLoggerForPrefix gets the logger for a certain prefix if it has been configured
func GetLoggerForPrefix(prefix string) *Entry {
	set := activeSet.Load()
	if logger, ok := set.loggers[prefix]; ok {
		return (*Entry)(logger.WithFields(logrus.Fields{"module": prefix}))
	}
	return (*Entry)(set.defaultLogger.WithFields(logrus.Fields{"module": prefix}))
}

// SetLevel sets the default loggers level
func SetLevel(level logrus.Level) {
	activeSet.Load().defaultLogger.SetLevel(level)
}

var (
	startServer sync.Once

	// configMu serializes ConfigureAllLoggers calls (parsing the debug string,
	// resetting state, swapping the snapshot). Hot-path readers do not take it.
	configMu   sync.Mutex
	cancelFunc context.CancelFunc // guarded by configMu
)

var noCustomizations atomic.Bool

// ConfigureLogger takes in a logger object and configures the logger depending on environment variables.
// Configured based on the GOLANG_DEBUG environment variable
func ConfigureAllLoggers(newdefaultLogger *logrus.Logger, debugConfig string) {
	configMu.Lock()
	defer configMu.Unlock()

	noCustomizations.Store(debugConfig == "")
	levels := make(map[string]int)

	if cancelFunc != nil {
		cancelFunc()
		cancelFunc = nil
	}

	// reset all
	printGoRoutines.Store(false)
	filelines.Store(false)

	startProfileServer := false
	profileServerPort := uint16(11111)
	if debugConfig != "" {
		packages := strings.Split(debugConfig, ",")

		for _, pkg := range packages {
			// check if a package name has been specified, if not default to main
			tmp := strings.Split(pkg, "=")
			if len(tmp) == 1 && tmp[0] == "ln" {
				filelines.Store(true)
			} else if len(tmp) == 2 && tmp[0] == "mut" { // mut=10 to set it up
				if val, err := strconv.Atoi(tmp[1]); err == nil {
					runtime.SetMutexProfileFraction(val)
				}
			} else if len(tmp) == 2 && tmp[0] == "blk" { // blk=10 to set blockProfile
				if val, err := strconv.Atoi(tmp[1]); err == nil {
					runtime.SetBlockProfileRate(val)
				}
			} else if len(tmp) == 1 && tmp[0] == "pp" { // pprof
				startProfileServer = true
			} else if len(tmp) == 2 && tmp[0] == "ppport" { // pprof port
				if val, err := strconv.Atoi(tmp[1]); err == nil {
					profileServerPort = uint16(val)
				}
			} else if len(tmp) == 1 && tmp[0] == "gr" { // go routine log
				printGoRoutines.Store(true)
			} else if len(tmp) == 1 && tmp[0] == "grl" { // go routine loop
				printGoRoutines.Store(true)
				ctx, cancel := context.WithCancel(context.Background())
				cancelFunc = cancel
				go logGoRoutines(ctx)
			} else if len(tmp) == 1 {
				levels["global_log"] = toEnum(tmp[0])
			} else if len(tmp) == 2 {
				levels[tmp[0]] = toEnum(tmp[1])
			} else {
				newdefaultLogger.Fatal("line: '", pkg, "' is formatted incorrectly, please refer to the documentation for correct usage")
			}
		}
	}

	newLoggers := make(map[string]*logrus.Logger, len(levels))
	for key, value := range levels {
		// Copy some properties of the default logger
		pLogger := logrus.New()
		pLogger.Out = newdefaultLogger.Out
		pLogger.Formatter = newdefaultLogger.Formatter
		newLoggers[key] = configurePackageLogger(pLogger, value)
	}

	// configure main logger
	newDefault := newdefaultLogger
	if value, ok := newLoggers["global_log"]; ok {
		newDefault = value
	}

	// Publish the new snapshot atomically. Hot-path readers see either the
	// previous fully-built set or the new fully-built set, never a partial map.
	activeSet.Store(&loggerSet{
		defaultLogger: newDefault,
		loggers:       newLoggers,
	})

	if startProfileServer {
		startServer.Do(func() {
			go profileServer(profileServerPort)
		})
	}
}

func AutoStartProfileServer(port uint16) {
	if port == 0 {
		port = 11111
	}
	startServer.Do(func() {
		go profileServer(port)
	})
}

// frameInfo is the cached result of resolving a PC to a (pkg, file, line).
// The values are deterministic per PC (mainModuleName is set once in init),
// so we can cache and skip the FuncForPC + string surgery on every log call.
type frameInfo struct {
	pkg  string
	file string
	line int
}

// frameCache maps PC (uintptr) -> frameInfo. sync.Map fits the access pattern
// well: writes only happen on first observation of each call site, and reads
// vastly outnumber writes thereafter.
var frameCache sync.Map

// Props to https://stackoverflow.com/a/35213181 for the code
func getPackage() (string, string, int) {

	// Stack-allocated buffer; avoids a heap allocation per log call.
	var fpcs [1]uintptr

	// skip 4 levels to get to the caller of whoever called getPackage()
	n := runtime.Callers(4, fpcs[:])
	if n == 0 {
		return "", "", 0 // proper error her would be better
	}

	pc := fpcs[0]
	if v, ok := frameCache.Load(pc); ok {
		fi := v.(frameInfo)
		return fi.pkg, fi.file, fi.line
	}

	// get the info of the actual function that's in the pointer
	fun := runtime.FuncForPC(pc - 1)
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

	file, line := fun.FileLine(pc - 1)

	if i := strings.Index(file, mainModuleName); i != -1 {
		file = file[i:]
	}

	if i := strings.Index(file, "@"); i != -1 {
		// Trim out the version info in case we run with -trimpath
		nextSlash := strings.Index(file[i:], "/")
		file = file[:i] + file[i+nextSlash:]
	}

	pkg := strings.TrimPrefix(name[0:lastSlash+firstPoint], mainModuleName+"/")
	file = strings.TrimPrefix(file, mainModuleName+"/")

	frameCache.Store(pc, frameInfo{pkg: pkg, file: file, line: line})
	return pkg, file, line
}

func getLogger(e *Entry) *logrus.Entry {
	// One atomic load gives us a consistent (defaultLogger, loggers) view for
	// the duration of this call, even if ConfigureAllLoggers swaps mid-flight.
	set := activeSet.Load()
	pkg, file, line := getPackage()

	var logentry *logrus.Entry
	if e != nil {
		logentry = (*logrus.Entry)(e)
	} else {
		if log, ok := set.loggers[pkg]; ok {
			logentry = log.WithFields(logrus.Fields{"module": pkg})
		} else {
			logentry = set.defaultLogger.WithFields(logrus.Fields{"module": pkg})
		}
	}

	if filelines.Load() {
		logentry = logentry.WithFields(logrus.Fields{"file": fmt.Sprintf("'%s:%d'", file, line)})
	}

	if printGoRoutines.Load() {
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
	if noCustomizations.Load() {
		return
	}
	getLogger(nil).Trace(args...)
}

func Traceln(args ...interface{}) {
	if noCustomizations.Load() {
		return
	}
	getLogger(nil).Traceln(args...)
}

func Tracef(format string, args ...interface{}) {
	if noCustomizations.Load() {
		return
	}
	getLogger(nil).Tracef(format, args...)
}

func Debug(args ...interface{}) {
	if noCustomizations.Load() {
		return
	}
	getLogger(nil).Debug(args...)
}

func Debugln(args ...interface{}) {
	if noCustomizations.Load() {
		return
	}
	getLogger(nil).Debugln(args...)
}

func Debugf(format string, args ...interface{}) {
	if noCustomizations.Load() {
		return
	}
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
