package env_logger

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	logrus "github.com/sirupsen/logrus"
)

// Wrap an error, this is useful in combination with Should and Must
func Wrap(err error, msg string, args ...interface{}) error {
	if err != nil {
		args = append(args, err)
		return fmt.Errorf(msg+": %w", args...)
	}
	return nil
}

func (e *Entry) Wrap(err error, msg string, args ...interface{}) error {
	return Wrap(err, msg, args...)
}

// Wrap an error, this is useful in combination with Should and Must
func WrapFinal(err *error, msg string, args ...interface{}) {
	if err != nil && *err != nil {
		args = append(args, *err)
		*err = fmt.Errorf(msg+": %w", args...) // Change actual value
	}
}

func (e *Entry) WrapFinal(err *error, msg string, args ...interface{}) {
	WrapFinal(err, msg, args...)
}

// PanicHandler logs a panic before passing it on, use it with defer. The
// panic is logged for the module (and line) that caused it.
func PanicHandler() {
	if r := recover(); r != nil {
		set := activeSet.Load()
		set.withRoutines(set.siteForPC(panicPC()).entry).Panic(r)
	}
}

func (e *Entry) PanicHandler() {
	if r := recover(); r != nil {
		set := activeSet.Load()
		logentry := set.rebind((*logrus.Entry)(e))
		if set.filelines {
			logentry = logentry.WithFields(logrus.Fields{"file": set.siteForPC(panicPC()).file})
		}
		set.withRoutines(logentry).Panic(r)
	}
}

// panicPC finds the PC of the code that panicked. Has to be called directly
// by the deferred function that recovered the panic.
func panicPC() uintptr {
	var pcs [16]uintptr

	// skip runtime.Callers, panicPC and the deferred function, what follows
	// is the runtime's panic machinery and then the code that set it off
	n := runtime.Callers(3, pcs[:])
	for _, pc := range pcs[:n] {
		if fun := runtime.FuncForPC(pc - 1); fun != nil && !strings.HasPrefix(fun.Name(), "runtime.") {
			return pc
		}
	}
	return 0
}

// Indent transforms the structure into json by using MarshalIndent
func Indent(arg interface{}) string {
	indented, _ := json.MarshalIndent(arg, "", " ")
	return string(indented)
}

func (e *Entry) Indent(arg interface{}) string {
	return Indent(arg)
}

func logGoRoutines(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			Info("Routines: ", runtime.NumGoroutine())
		}
	}
}
