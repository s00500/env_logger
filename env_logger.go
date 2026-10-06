package env_logger

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
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
	// filelines adds the file field (ln), printGoRoutines the routines
	// field (gr) to every log statement.
	filelines       bool
	printGoRoutines bool
	// moduleEntries caches the module-decorated entries of logger. They only
	// depend on logger, so snapshots of the same logger share one cache; a
	// reconfigure swaps in a fresh empty cache and the old one is GC'd along
	// with the old set.
	moduleEntries *entryCache
	// sites caches what a log call resolves to in this snapshot, keyed by
	// the PC of the call, so the hot path is a single lookup.
	sites *sync.Map // key: uintptr (pc), value: *callSite
}

// maxModuleEntries bounds entryCache, so that prefixes generated at runtime
// (per connection, per device, ...) can not grow it forever.
const maxModuleEntries = 1024

// entryCache caches the *logrus.Entry produced by
// `logger.WithFields({"module": pkg})` keyed by pkg, so repeated log calls
// from the same package reuse a single entry instead of allocating a Fields
// map + Entry on every emit.
type entryCache struct {
	entries sync.Map // key: string (pkg), value: *logrus.Entry
	size    atomic.Int32
}

// callSite is what a log call from one PC resolves to in a snapshot.
type callSite struct {
	pkg string
	// file is the value of the file field, "'file:line'"
	file string
	// level is the level configured for pkg
	level logrus.Level
	// entry carries the module field, plus the file field if filelines is on
	entry *logrus.Entry
}

// moduleEntry returns the cached module-decorated entry for pkg, building it
// on first observation. Safe for concurrent callers thanks to sync.Map's
// LoadOrStore — duplicate work on a race is harmless and discarded.
func (s *loggerSet) moduleEntry(pkg string) *logrus.Entry {
	cache := s.moduleEntries
	if cached, ok := cache.entries.Load(pkg); ok {
		return cached.(*logrus.Entry)
	}
	entry := s.logger.WithFields(logrus.Fields{"module": pkg})
	if cache.size.Load() >= maxModuleEntries {
		return entry
	}
	actual, loaded := cache.entries.LoadOrStore(pkg, entry)
	if !loaded {
		cache.size.Add(1)
	}
	return actual.(*logrus.Entry)
}

// levelFor returns the level configured for pkg.
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

// rebind makes sure e logs through the active logger. Entries that were
// built before a reconfigure still point at the logger of that time; they
// have to follow the new output, formatter and hooks like everything else.
func (s *loggerSet) rebind(e *logrus.Entry) *logrus.Entry {
	if e.Logger == s.logger {
		return e
	}
	rebound := *e
	rebound.Logger = s.logger
	return &rebound
}

// withRoutines adds the routines field if it is switched on.
func (s *loggerSet) withRoutines(logentry *logrus.Entry) *logrus.Entry {
	if s.printGoRoutines {
		return logentry.WithFields(logrus.Fields{"routines": runtime.NumGoroutine()})
	}
	return logentry
}

// callSite resolves the PC found skip frames up the stack, see
// runtime.Callers. It has to be called directly by the function that is
// called directly by the user for the skip of 4 the log functions use.
func (s *loggerSet) callSite(skip int) *callSite {
	// Stack-allocated buffer; avoids a heap allocation per log call.
	var fpcs [1]uintptr
	if runtime.Callers(skip, fpcs[:]) == 0 {
		return s.siteForPC(0) // proper error her would be better
	}
	return s.siteForPC(fpcs[0])
}

// siteForPC returns the cached callSite for pc, building it on first
// observation.
func (s *loggerSet) siteForPC(pc uintptr) *callSite {
	if cached, ok := s.sites.Load(pc); ok {
		return cached.(*callSite)
	}
	fi := resolveFrame(pc)
	site := &callSite{
		pkg:   fi.pkg,
		file:  fi.file,
		level: s.levelFor(fi.pkg),
		entry: s.moduleEntry(fi.pkg),
	}
	if s.filelines {
		site.entry = site.entry.WithFields(logrus.Fields{"file": site.file})
	}
	actual, _ := s.sites.LoadOrStore(pc, site)
	return actual.(*callSite)
}

var activeSet atomic.Pointer[loggerSet]

// publishSet completes the snapshot, raises its logger to the most
// permissive level any package wants and makes the snapshot the active one.
// Caller must hold configMu.
func publishSet(set *loggerSet) *loggerSet {
	set.maxLevel = set.defaultLevel
	for _, lvl := range set.levels {
		if lvl > set.maxLevel {
			set.maxLevel = lvl
		}
	}
	set.sites = new(sync.Map)

	// Hot-path readers see either the previous fully-built set or the new
	// fully-built set, never a partial one.
	set.logger.SetLevel(set.maxLevel)
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
		next := *set
		next.defaultLevel, next.baseLevel = lvl, lvl
		set = publishSet(&next)
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

var mainModuleName string // written only from init(), then read-only

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
	configMu.Lock()
	defer configMu.Unlock()

	next := *activeSet.Load()
	next.filelines = true
	publishSet(&next)
}

// GetLoggerForPrefix gets the logger for a certain prefix if it has been configured
func GetLoggerForPrefix(prefix string) *Entry {
	return (*Entry)(activeSet.Load().moduleEntry(prefix))
}

// GetLoggerForPackage gets the logger for the package it is called from.
// Logging through it gives the same output as the package level functions,
// but without looking up the caller on every call, which makes it the
// cheaper choice for hot code: var log = env_logger.GetLoggerForPackage()
func GetLoggerForPackage() *Entry {
	set := activeSet.Load()
	return (*Entry)(set.moduleEntry(set.callSite(3).pkg))
}

// SetLevel sets the default level, the one used by every package that has no
// level of its own in the debug config
func SetLevel(level logrus.Level) {
	configMu.Lock()
	defer configMu.Unlock()

	next := *activeSet.Load()
	next.defaultLevel, next.baseLevel = level, level
	publishSet(&next)
}

var (
	startServer sync.Once

	// configMu serializes ConfigureAllLoggers calls (parsing the debug string,
	// resetting state, swapping the snapshot). Hot-path readers do not take it.
	configMu   sync.Mutex
	cancelFunc context.CancelFunc // guarded by configMu

	// What mut= and blk= changed, to undo it once the config drops them.
	// Guarded by configMu.
	mutexProfileSet  bool
	mutexProfilePrev int
	blockProfileSet  bool
)

// ConfigureLogger takes in a logger object and configures the logger depending on environment variables.
// Configured based on the GOLANG_DEBUG environment variable
//
// Parts of the debugConfig that can not be understood are ignored and
// reported as a warning on the logger.
func ConfigureAllLoggers(newdefaultLogger *logrus.Logger, debugConfig string) {
	configMu.Lock()
	defer configMu.Unlock()

	configureLocked(newdefaultLogger, debugConfig)
}

// reconfigureActiveLogger applies a new debug config to the logger that is
// in use, keeping its output, formatter and hooks.
func reconfigureActiveLogger(debugConfig string) {
	configMu.Lock()
	defer configMu.Unlock()

	configureLocked(activeSet.Load().logger, debugConfig)
}

// configureLocked does the work of ConfigureAllLoggers.
// Caller must hold configMu.
func configureLocked(newdefaultLogger *logrus.Logger, debugConfig string) {
	// Without a global level in the config the logger keeps the level its
	// owner gave it. When the active logger is passed in again its level
	// holds our maxLevel instead, unless it was changed directly since.
	baseLevel := newdefaultLogger.GetLevel()
	if old := activeSet.Load(); old != nil && old.logger == newdefaultLogger && baseLevel == old.maxLevel {
		baseLevel = old.baseLevel
	}

	set := &loggerSet{
		logger:        newdefaultLogger,
		levels:        make(map[string]logrus.Level),
		defaultLevel:  baseLevel,
		baseLevel:     baseLevel,
		moduleEntries: new(entryCache),
	}

	if cancelFunc != nil {
		cancelFunc()
		cancelFunc = nil
	}

	var problems []string

	startProfileServer := false
	profileServerPort := uint16(11111)
	routineLoop := false
	mutexFraction, setMutexFraction := 0, false
	blockRate, setBlockRate := 0, false

	for _, option := range strings.Split(debugConfig, ",") {
		option = strings.TrimSpace(option)
		if option == "" {
			continue
		}

		// check if a package name has been specified, if not default to main
		key, value, hasValue := strings.Cut(option, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)

		switch {
		case !hasValue && key == "ln":
			set.filelines = true
		case !hasValue && key == "pp": // pprof
			startProfileServer = true
		case !hasValue && key == "gr": // go routine log
			set.printGoRoutines = true
		case !hasValue && key == "grl": // go routine loop
			set.printGoRoutines = true
			routineLoop = true
		case hasValue && key == "mut": // mut=10 to set it up
			if val, err := strconv.Atoi(value); err == nil {
				mutexFraction, setMutexFraction = val, true
			} else {
				problems = append(problems, fmt.Sprintf("'%s': '%s' is not a number", option, value))
			}
		case hasValue && key == "blk": // blk=10 to set blockProfile
			if val, err := strconv.Atoi(value); err == nil {
				blockRate, setBlockRate = val, true
			} else {
				problems = append(problems, fmt.Sprintf("'%s': '%s' is not a number", option, value))
			}
		case hasValue && key == "ppport": // pprof port
			if val, err := strconv.Atoi(value); err == nil && val > 0 && val <= 65535 {
				profileServerPort = uint16(val)
			} else {
				problems = append(problems, fmt.Sprintf("'%s': '%s' is not a port", option, value))
			}
		case !hasValue:
			if level, err := logrus.ParseLevel(key); err == nil {
				set.defaultLevel = level
			} else {
				problems = append(problems, fmt.Sprintf("'%s' is neither a level nor an option", option))
			}
		case key == "" || strings.Contains(value, "="):
			problems = append(problems, fmt.Sprintf("'%s' is formatted incorrectly, please refer to the documentation for correct usage", option))
		default:
			if level, err := logrus.ParseLevel(value); err == nil {
				set.levels[key] = level
			} else {
				problems = append(problems, fmt.Sprintf("'%s': '%s' is not a level", option, value))
			}
		}
	}

	// reset what an earlier config changed and this one no longer asks for
	if setMutexFraction {
		prev := runtime.SetMutexProfileFraction(mutexFraction)
		if !mutexProfileSet {
			mutexProfileSet, mutexProfilePrev = true, prev
		}
	} else if mutexProfileSet {
		runtime.SetMutexProfileFraction(mutexProfilePrev)
		mutexProfileSet = false
	}
	if setBlockRate {
		runtime.SetBlockProfileRate(blockRate)
		blockProfileSet = true
	} else if blockProfileSet {
		runtime.SetBlockProfileRate(0)
		blockProfileSet = false
	}

	publishSet(set)

	// Reported on the logger directly: the log functions of this package
	// may need configMu, which is held here.
	for _, problem := range problems {
		newdefaultLogger.WithFields(logrus.Fields{"module": "env_logger"}).Warn("ignoring log config ", problem)
	}

	if routineLoop {
		ctx, cancel := context.WithCancel(context.Background())
		cancelFunc = cancel
		go logGoRoutines(ctx)
	}

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

// frameInfo is the cached result of resolving a PC to a (pkg, file:line).
// The values are deterministic per PC (mainModuleName is set once in init),
// so we can cache and skip the FuncForPC + string surgery on every log call.
type frameInfo struct {
	pkg string
	// file is preformatted as the value of the file field, "'file:line'"
	file string
}

// frameCache maps PC (uintptr) -> frameInfo. sync.Map fits the access pattern
// well: writes only happen on first observation of each call site, and reads
// vastly outnumber writes thereafter. Unlike loggerSet.sites it survives a
// reconfigure.
var frameCache sync.Map

// Props to https://stackoverflow.com/a/35213181 for the code
func resolveFrame(pc uintptr) frameInfo {
	if v, ok := frameCache.Load(pc); ok {
		return v.(frameInfo)
	}

	unknown := frameInfo{file: "':0'"}
	if pc == 0 {
		return unknown
	}

	// get the info of the actual function that's in the pointer
	fun := runtime.FuncForPC(pc - 1)
	if fun == nil {
		return unknown
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

	fi := frameInfo{pkg: pkg, file: fmt.Sprintf("'%s:%d'", file, line)}
	frameCache.Store(pc, fi)
	return fi
}

// ListModules lists the modules that have logged so far, sorted. These are
// the names to use for per-package levels in the debug config. Calls that
// were dropped because no package at all wants their level are not seen.
func ListModules() []string {
	seen := make(map[string]struct{})
	frameCache.Range(func(_, v interface{}) bool {
		seen[v.(frameInfo).pkg] = struct{}{}
		return true
	})
	activeSet.Load().moduleEntries.entries.Range(func(k, _ interface{}) bool {
		seen[k.(string)] = struct{}{}
		return true
	})

	modules := make([]string, 0, len(seen))
	for module := range seen {
		modules = append(modules, module)
	}
	sort.Strings(modules)
	return modules
}

func getLogger(e *Entry) *logrus.Entry {
	// One atomic load gives us a consistent (logger, levels) view for
	// the duration of this call, even if ConfigureAllLoggers swaps mid-flight.
	set := activeSet.Load()

	if e != nil {
		logentry := set.rebind((*logrus.Entry)(e))
		// Pre-existing entry: the caller is only needed for the file field.
		if set.filelines {
			logentry = logentry.WithFields(logrus.Fields{"file": set.callSite(4).file})
		}
		return set.withRoutines(logentry)
	}

	return set.withRoutines(set.callSite(4).entry)
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

	if e != nil {
		logentry := (*logrus.Entry)(e)
		// Pre-existing entry: gate on the level of the module it was built for.
		if level > set.levelForEntry(logentry) {
			return nil
		}
		logentry = set.rebind(logentry)
		// The caller is only needed for the file field.
		if set.filelines {
			logentry = logentry.WithFields(logrus.Fields{"file": set.callSite(4).file})
		}
		return set.withRoutines(logentry)
	}

	site := set.callSite(4)

	// Per-package gate: the global gate above only proved *some* package
	// wants this level; *this* package might still reject it.
	if level > site.level {
		return nil
	}
	return set.withRoutines(site.entry)
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
