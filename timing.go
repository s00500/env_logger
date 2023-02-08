package env_logger

import (
	"fmt"
	"sync"
	"time"
)

var timers map[string]time.Time
var timersMu sync.Mutex

func Time(idkey string) string {
	timersMu.Lock()
	defer timersMu.Unlock()
	timers[idkey] = time.Now()

	return idkey
}

// Print time since the last call to the Time function with the same name
func TimeEnd(idkey string) string {
	timersMu.Lock()
	defer timersMu.Unlock()
	if t, ok := timers[idkey]; ok {
		res := fmt.Sprint(time.Since(t))
		delete(timers, idkey)
		return res
	}

	return "unknown timer"
}

func (e *Entry) Time(idkey string) string {
	return Time(idkey)
}

func (e *Entry) TimeEnd(idkey string) string {
	return Time(idkey)
}
