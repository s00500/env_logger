package env_logger

import (
	"context"
	stdlog "log"
	"log/slog"
	"slices"

	logrus "github.com/sirupsen/logrus"
)

// slogHandler is a slog.Handler that logs through env_logger, so statements
// made with log/slog get the same output, levels from the debug config and
// sinks as everything else.
//
// It holds no state of the active configuration itself, every call reads the
// active snapshot, so it follows ConfigureAllLoggers, SetLevel, the
// /logstring endpoint and AddSink like the package level functions do.
type slogHandler struct {
	// name is the module the statements are logged for, and the name its
	// level is looked up by in the debug config. Empty means the module is
	// the package of the code that made the slog call.
	name string
	// fields are the attributes added with WithAttrs, flattened.
	fields []slogField
	// prefix is the dotted path of the groups opened with WithGroup, ending
	// in a dot, prepended to the keys of every attribute added after.
	prefix string
}

// slogUnknownModule is the module of statements made through a handler of
// SlogHandler("") that carry no caller, eg. records built by hand.
const slogUnknownModule = "slog"

type slogField struct {
	key   string
	value interface{}
}

// SlogHandler returns a slog.Handler that logs through env_logger as the
// module name, so `LOG=name=debug` sets its level. Use it to hand libraries
// that take a *slog.Logger a logger that writes like the rest of the
// application, see Slog.
//
// With an empty name the module is the package that makes the slog call,
// just like with the package level functions. That is what SetSlogDefault
// uses. Statements that carry no caller are logged as module "slog". As that package is only known once the call is made, the level can
// then only be checked up front against the most verbose level of any
// package; a statement the package of the caller does not want is dropped
// after slog has built it.
//
// slog levels map to Trace (below Debug), Debug, Info, Warn and Error (Error
// and above). slog never logs at Fatal or Panic. Groups are flattened into
// dotted keys, error values are kept as errors.
func SlogHandler(name string) slog.Handler {
	return &slogHandler{name: name}
}

// Slog returns a *slog.Logger that logs through env_logger as the module name,
// see SlogHandler. Eg: lbsauth.Config{Logger: log.Slog("lbsauth")} together
// with LOG=lbsauth=debug
func Slog(name string) *slog.Logger {
	return slog.New(SlogHandler(name))
}

// SetSlogDefault makes slog.Default log through env_logger, every statement
// as the package that makes it, see SlogHandler with an empty name.
//
// slog.SetDefault also sends the output of the standard library's log
// package through the new handler, so log.Printf and the like end up in
// env_logger as well, at Info and as the package that calls them. env_logger
// itself never writes through the log package, so that does not loop, as
// long as the logrus logger passed to ConfigureAllLoggers does not write to
// log.Writer() either.
func SetSlogDefault() {
	// slog only records the caller of log.Printf and the like if the log
	// package was asked for file names when the default is set. It resets
	// the flags to 0 itself right after, they only decide that.
	stdlog.SetFlags(stdlog.Flags() | stdlog.Lshortfile)
	slog.SetDefault(Slog(""))
}

// slogLevel maps a slog level to the logrus level it is logged at.
func slogLevel(level slog.Level) logrus.Level {
	switch {
	case level < slog.LevelDebug:
		return logrus.TraceLevel
	case level < slog.LevelInfo:
		return logrus.DebugLevel
	case level < slog.LevelWarn:
		return logrus.InfoLevel
	case level < slog.LevelError:
		return logrus.WarnLevel
	default:
		return logrus.ErrorLevel
	}
}

// Enabled reports whether a statement at level would be logged or passed
// to a sink. No allocation, no caller lookup.
func (h *slogHandler) Enabled(_ context.Context, level slog.Level) bool {
	set := loadSet()
	lvl := slogLevel(level)
	if lvl > set.maxLevel {
		return false
	}
	if h.name == "" {
		// the package is not known yet, Handle checks it
		return true
	}
	return lvl <= set.levelFor(h.name) || lvl <= set.sinkLevel
}

// Handle logs r through env_logger.
func (h *slogHandler) Handle(ctx context.Context, r slog.Record) error {
	set := loadSet()
	lvl := slogLevel(r.Level)
	if lvl > set.maxLevel {
		return nil
	}

	name := h.name
	if name == "" && r.PC == 0 {
		// no caller to take the package from
		name = slogUnknownModule
	}

	var entry *logrus.Entry
	if name == "" {
		site := set.siteForPC(r.PC)
		switch {
		case lvl <= site.level:
			entry = site.entry
		case lvl <= set.sinkLevel:
			entry = site.sinkEntry
		default:
			return nil
		}
	} else {
		switch {
		case lvl <= set.levelFor(name):
			entry = set.moduleEntry(name)
		case lvl <= set.sinkLevel:
			entry = toSink(set.moduleEntry(name))
		default:
			return nil
		}
		if set.filelines && r.PC != 0 {
			entry = entry.WithFields(logrus.Fields{"file": resolveFrame(r.PC).file})
		}
	}
	entry = set.withRoutines(entry)

	fields := make(logrus.Fields, len(h.fields)+r.NumAttrs())
	for _, f := range h.fields {
		fields[f.key] = f.value
	}
	r.Attrs(func(a slog.Attr) bool {
		addSlogAttr(fields, h.prefix, a)
		return true
	})

	// WithFields returns a new entry that belongs to this call alone
	entry = entry.WithFields(fields)
	entry.Time = r.Time
	entry.Context = ctx
	entry.Log(lvl, r.Message)
	return nil
}

// WithAttrs returns a handler that adds attrs to every statement.
func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	next := *h
	next.fields = slices.Clip(h.fields)
	for _, a := range attrs {
		// keep the order of attrs, so a later attr with the same key wins
		// as it does in a statement
		collectSlogAttr(&next.fields, h.prefix, a)
	}
	return &next
}

// WithGroup returns a handler that puts every attribute added after it into
// the group name, as a "name." prefix of its key.
func (h *slogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	next := *h
	next.prefix = h.prefix + name + "."
	return &next
}

// addSlogAttr adds a to fields, with groups flattened into dotted keys.
func addSlogAttr(fields logrus.Fields, prefix string, a slog.Attr) {
	walkSlogAttr(prefix, a, func(key string, value interface{}) {
		fields[key] = value
	})
}

// collectSlogAttr appends a to list, with groups flattened into dotted keys.
func collectSlogAttr(list *[]slogField, prefix string, a slog.Attr) {
	walkSlogAttr(prefix, a, func(key string, value interface{}) {
		*list = append(*list, slogField{key: key, value: value})
	})
}

// walkSlogAttr resolves a and calls add for it, or for every attribute in it
// if it is a group, following the rules of slog.Handler: empty attributes
// are dropped, as are empty groups, and the attributes of a group without a
// key are added as if they were not in a group.
func walkSlogAttr(prefix string, a slog.Attr, add func(key string, value interface{})) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			prefix += a.Key + "."
		}
		for _, member := range a.Value.Group() {
			walkSlogAttr(prefix, member, add)
		}
		return
	}
	add(prefix+a.Key, slogValue(a.Value))
}

// slogValue unwraps v, so the formatter gets the value itself. Errors stay
// errors.
func slogValue(v slog.Value) interface{} {
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindInt64:
		return v.Int64()
	case slog.KindUint64:
		return v.Uint64()
	case slog.KindFloat64:
		return v.Float64()
	case slog.KindBool:
		return v.Bool()
	case slog.KindDuration:
		return v.Duration()
	case slog.KindTime:
		return v.Time()
	default:
		return v.Any()
	}
}
