package env_logger

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"time"
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

// Wrap an error, this is useful in combination with Should and Must
func PanicHandler() {
	if r := recover(); r != nil {
		Panic(r)
	}
}

func (e *Entry) PanicHandler() {
	if r := recover(); r != nil {
		e.Panic(r)
	}
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
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			Info("Routines: ", runtime.NumGoroutine())
		}
	}
}
