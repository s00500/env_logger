package env_logger

import (
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

// Indent transforms the structure into json by using MarshalIndent
func Indent(arg interface{}) string {
	indented, _ := json.MarshalIndent(arg, "", " ")
	return string(indented)
}

func logGoRoutines() {
	for {
		time.Sleep(time.Second)
		Info("Routines: ", runtime.NumGoroutine())
	}
}
