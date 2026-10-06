//go:build !logpprof
// +build !logpprof

package env_logger

import "net/http"

// dynamic config still works without pprof
func registerPprof(mux *http.ServeMux) {
	Warn("pprof server not included at compiletime")
}
