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
	// logger is the one logrus logger everything is emitted through. Per-
	// package levels are enforced by this package rather than by per-package
	// logger copies, so the output, formatter, hooks and write lock are
	// shared by every package.
	logger *logrus.Logger
	// levels holds the per-package levels from the config string.
	levels map[string]logrus.Level
	// defaultLevel applies to every package that has no entry in levels.
	defaultLevel logrus.Level
	// baseLevel is the level the logger itself was given by its owner, which
	// is the default level when the config string does not name one. It has
	// to be remembered because logger.Level is overwritten with maxLevel.
	baseLevel logrus.Level
	// maxLevel is the most permissive (highest numeric) level across
	// defaultLevel and every per-package level. Used as a cheap gate so
	// a Debug call under LOG=info can return without resolving the caller
	// frame at all. It is also the level set on logger, so that logrus lets
	// through whatever the most verbose package wants.
	maxLevel logrus.Level
	// moduleEntries caches the *logrus.Entry produced by
	// `logger.WithFields({"module": pkg})` keyed by pkg, so repeated log
	// calls from the same package reuse a single entry instead of
	// allocating a Fields map + Entry on every emit. The entries only depend
	// on logger, so snapshots of the same logger share one cache; a
	// reconfigure swaps in a fresh empty cache and the old one is GC'd along
	// with the old set.
	moduleEntries *sync.Map // key: string (pkg), value: *logrus.Entry
}

// moduleEntry returns the cached module-decorated entry for pkg, building it
// on first observation. Safe for concurrent callers thanks to sync.Map's
// LoadOrStore — duplicate work on a race is harmless and discarded.
func (s *loggerSet) moduleEntry(pkg string) *logrus.Entry {
	if cached, ok := s.moduleEntries.Load(pkg); ok {
		return cached.(*logrus.Entry)
	}
	entry := s.logger.WithFields(logrus.Fields{"module": pkg})
	actual, _ := s.moduleEntries.LoadOrStore(pkg, entry)
	return actual.(*logrus.Entry)
}

// levelFor returns the level configured for pkg.
// Used for the level gate before deciding to fetch the cached entry.
func (s *loggerSet) levelFor(pkg string) logrus.Level {
	if lvl, ok := s.levels[pkg]; ok {
		return lvl
	}
	return s.defaultLevel
}

// levelForEntry returns the level that applies to an already built entry,
// going by the module it was created for.
func (s *loggerSet) levelForEntry(e *logrus.Entry) logrus.Level {
	if len(s.levels) != 0 {
		if pkg, ok := e.Data["module"].(string); ok {
			return s.levelFor(pkg)
		}
	}
	return s.defaultLevel
}

var activeSet atomic.Pointer[loggerSet]

// publishSet builds a snapshot, raises logger to the most permissive level
// any package wants and makes the snapshot the active one.
// Caller must hold configMu.
func publishSet(logger *logrus.Logger, levels map[string]logrus.Level, baseLevel, defaultLevel logrus.Level, moduleEntries *sync.Map) *loggerSet {
	maxLevel := defaultLevel
	for _, lvl := range levels {
		if lvl > maxLevel {
			maxLevel = lvl
		}
	}

	set := &loggerSet{
		logger:        logger,
		levels:        levels,
		defaultLevel:  defaultLevel,
		baseLevel:     baseLevel,
		maxLevel:      maxLevel,
		moduleEntries: moduleEntries,
	}

	// Hot-path readers see either the previous fully-built set or the new
	// fully-built set, never a partial map.
	logger.SetLevel(maxLevel)
	activeSet.Store(set)
	return set
}

// loadSet returns the active snapshot for a level-gated log call. A logger
// level that differs from the one publishSet applied means SetLevel was
// called on the logger directly; resync so that it is honored instead of
// being masked by the snapshot's levels.
func loadSet() *loggerSet {
	set := activeSet.Load()
	if set.logger.GetLevel() != set.maxLevel {
		return resyncLevel()
	}
	return set
}

// resyncLevel adopts a level that was set on the logger directly as the new
// default level.
func resyncLevel() *loggerSet {
	configMu.Lock()
	defer configMu.Unlock()

	set := activeSet.Load()
	if lvl := set.logger.GetLevel(); lvl != set.maxLevel {
		set = publishSet(set.logger, set.levels, lvl, lvl, set.moduleEntries)
	}
	return set
}

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

func toLevel(value int) logrus.Level {
	switch value {
	case PanicV:
		return logrus.PanicLevel
	case FatalV:
		return logrus.FatalLevel
	case ErrV:
		return logrus.ErrorLevel
	case WarnV:
		return logrus.WarnLevel
	case InfoV:
		return logrus.InfoLevel
	case DebugV:
		return logrus.DebugLevel
	case TraceV:
		return logrus.TraceLevel
	default:
		return logrus.InfoLevel
	}
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
	return (*Entry)(activeSet.Load().moduleEntry(prefix))
}

// SetLevel sets the default level, the one used by every package that has no
// level of its own in the debug config
func SetLevel(level logrus.Level) {
	configMu.Lock()
	defer configMu.Unlock()

	set := activeSet.Load()
	publishSet(set.logger, set.levels, level, level, set.moduleEntries)
}

var (
	startServer sync.Once

	// configMu serializes ConfigureAllLoggers calls (parsing the debug string,
	// resetting state, swapping the snapshot). Hot-path readers do not take it.
	configMu   sync.Mutex
	cancelFunc context.CancelFunc // guarded by configMu
)

// ConfigureLogger takes in a logger object and configures the logger depending on environment variables.
// Configured based on the GOLANG_DEBUG environment variable
func ConfigureAllLoggers(newdefaultLogger *logrus.Logger, debugConfig string) {
	configMu.Lock()
	defer configMu.Unlock()

	levels := make(map[string]logrus.Level)

	// Without a global level in the config the logger keeps the level its
	// owner gave it. When the active logger is passed in again its level
	// holds our maxLevel instead, unless it was changed directly since.
	baseLevel := newdefaultLogger.GetLevel()
	if old := activeSet.Load(); old != nil && old.logger == newdefaultLogger && baseLevel == old.maxLevel {
		baseLevel = old.baseLevel
	}
	defaultLevel := baseLevel

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
				defaultLevel = toLevel(toEnum(tmp[0]))
			} else if len(tmp) == 2 {
				levels[tmp[0]] = toLevel(toEnum(tmp[1]))
			} else {
				newdefaultLogger.Fatal("line: '", pkg, "' is formatted incorrectly, please refer to the documentation for correct usage")
			}
		}
	}

	publishSet(newdefaultLogger, levels, baseLevel, defaultLevel, new(sync.Map))

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
	// One atomic load gives us a consistent (logger, levels) view for
	// the duration of this call, even if ConfigureAllLoggers swaps mid-flight.
	set := activeSet.Load()
	wantFile := filelines.Load()
	wantRoutines := printGoRoutines.Load()

	// Pre-existing entry: skip caller resolution entirely if no decoration
	// is needed.
	if e != nil && !wantFile && !wantRoutines {
		return (*logrus.Entry)(e)
	}

	pkg, file, line := getPackage()

	var logentry *logrus.Entry
	if e != nil {
		logentry = (*logrus.Entry)(e)
	} else {
		logentry = set.moduleEntry(pkg)
	}

	if wantFile {
		logentry = logentry.WithFields(logrus.Fields{"file": fmt.Sprintf("'%s:%d'", file, line)})
	}

	if wantRoutines {
		logentry = logentry.WithFields(logrus.Fields{"routines": runtime.NumGoroutine()})
	}

	return logentry
}

// getLoggerIfLevel is like getLogger but returns nil when the active
// snapshot does not accept the given level — letting callers skip the
// whole Log call (no runtime.Callers, no map lookup, no allocation).
//
// MUST NOT be used for Fatal or Panic levels: those have side effects
// (os.Exit / panic) that callers expect to fire even when the message is
// filtered.
func getLoggerIfLevel(e *Entry, level logrus.Level) *logrus.Entry {
	set := loadSet()

	// Global gate: if not even the most permissive package wants this level,
	// drop now — before resolving the caller frame.
	if level > set.maxLevel {
		return nil
	}

	wantFile := filelines.Load()
	wantRoutines := printGoRoutines.Load()

	if e != nil {
		logentry := (*logrus.Entry)(e)
		// Pre-existing entry: gate on the level of the module it was built for.
		if level > set.levelForEntry(logentry) {
			return nil
		}
		// Skip caller resolution entirely if no decoration is needed.
		if !wantFile && !wantRoutines {
			return logentry
		}
		_, file, line := getPackage()
		if wantFile {
			logentry = logentry.WithFields(logrus.Fields{"file": fmt.Sprintf("'%s:%d'", file, line)})
		}
		if wantRoutines {
			logentry = logentry.WithFields(logrus.Fields{"routines": runtime.NumGoroutine()})
		}
		return logentry
	}

	pkg, file, line := getPackage()

	// Per-package gate: the global gate above only proved *some* package
	// wants this level; *this* package might still reject it.
	if level > set.levelFor(pkg) {
		return nil
	}
	logentry := set.moduleEntry(pkg)

	if wantFile {
		logentry = logentry.WithFields(logrus.Fields{"file": fmt.Sprintf("'%s:%d'", file, line)})
	}
	if wantRoutines {
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
	if e := getLoggerIfLevel(nil, logrus.WarnLevel); e != nil {
		e.Warn(args...)
	}
}

func Warnln(args ...interface{}) {
	if e := getLoggerIfLevel(nil, logrus.WarnLevel); e != nil {
		e.Warnln(args...)
	}
}

func Warnf(format string, args ...interface{}) {
	if e := getLoggerIfLevel(nil, logrus.WarnLevel); e != nil {
		e.Warnf(format, args...)
	}
}

func Info(args ...interface{}) {
	if e := getLoggerIfLevel(nil, logrus.InfoLevel); e != nil {
		e.Info(args...)
	}
}

func Infoln(args ...interface{}) {
	if e := getLoggerIfLevel(nil, logrus.InfoLevel); e != nil {
		e.Infoln(args...)
	}
}

func Infof(format string, args ...interface{}) {
	if e := getLoggerIfLevel(nil, logrus.InfoLevel); e != nil {
		e.Infof(format, args...)
	}
}

func Trace(args ...interface{}) {
	if e := getLoggerIfLevel(nil, logrus.TraceLevel); e != nil {
		e.Trace(args...)
	}
}

func Traceln(args ...interface{}) {
	if e := getLoggerIfLevel(nil, logrus.TraceLevel); e != nil {
		e.Traceln(args...)
	}
}

func Tracef(format string, args ...interface{}) {
	if e := getLoggerIfLevel(nil, logrus.TraceLevel); e != nil {
		e.Tracef(format, args...)
	}
}

func Debug(args ...interface{}) {
	if e := getLoggerIfLevel(nil, logrus.DebugLevel); e != nil {
		e.Debug(args...)
	}
}

func Debugln(args ...interface{}) {
	if e := getLoggerIfLevel(nil, logrus.DebugLevel); e != nil {
		e.Debugln(args...)
	}
}

func Debugf(format string, args ...interface{}) {
	if e := getLoggerIfLevel(nil, logrus.DebugLevel); e != nil {
		e.Debugf(format, args...)
	}
}

// Print/Println/Printf are emitted at Info level by logrus.
func Print(args ...interface{}) {
	if e := getLoggerIfLevel(nil, logrus.InfoLevel); e != nil {
		e.Print(args...)
	}
}

func Println(args ...interface{}) {
	if e := getLoggerIfLevel(nil, logrus.InfoLevel); e != nil {
		e.Println(args...)
	}
}

func Printf(format string, args ...interface{}) {
	if e := getLoggerIfLevel(nil, logrus.InfoLevel); e != nil {
		e.Printf(format, args...)
	}
}

func Error(args ...interface{}) {
	if e := getLoggerIfLevel(nil, logrus.ErrorLevel); e != nil {
		e.Error(args...)
	}
}

func Errorf(format string, args ...interface{}) {
	if e := getLoggerIfLevel(nil, logrus.ErrorLevel); e != nil {
		e.Errorf(format, args...)
	}
}

func Errorln(args ...interface{}) {
	if e := getLoggerIfLevel(nil, logrus.ErrorLevel); e != nil {
		e.Errorln(args...)
	}
}

// Fatal/Panic stay un-gated: their side effects (os.Exit, panic) must run
// even when the underlying message would be filtered by the logger's level.
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

// Log/Logf/Logln dispatch on a runtime-supplied level. Fatal/Panic must still
// run their side effects even if filtered, so we only gate other levels.
func Log(level logrus.Level, args ...interface{}) {
	if level == logrus.FatalLevel || level == logrus.PanicLevel {
		getLogger(nil).Log(level, args...)
		return
	}
	if e := getLoggerIfLevel(nil, level); e != nil {
		e.Log(level, args...)
	}
}

func Logf(level logrus.Level, format string, args ...interface{}) {
	if level == logrus.FatalLevel || level == logrus.PanicLevel {
		getLogger(nil).Logf(level, format, args...)
		return
	}
	if e := getLoggerIfLevel(nil, level); e != nil {
		e.Logf(level, format, args...)
	}
}

func Logln(level logrus.Level, args ...interface{}) {
	if level == logrus.FatalLevel || level == logrus.PanicLevel {
		getLogger(nil).Logln(level, args...)
		return
	}
	if e := getLoggerIfLevel(nil, level); e != nil {
		e.Logln(level, args...)
	}
}
