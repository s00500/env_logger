//go:build logpprof
// +build logpprof

package env_logger

import (
	"net/http"
	"net/http/pprof"
)

func registerPprof(mux *http.ServeMux) {
	// Register the standard pprof endpoints on our own mux.
	// (net/http/pprof's init() only registers them on http.DefaultServeMux.)
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}
