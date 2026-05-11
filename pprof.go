//go:build logpprof
// +build logpprof

package env_logger

import (
	"fmt"
	"io"
	"net/http"
	"net/http/pprof"
	"strings"
)

func profileServer(port uint16) {
	mux := http.NewServeMux()

	// Register the standard pprof endpoints on our own mux.
	// (net/http/pprof's init() only registers them on http.DefaultServeMux.)
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "")
	})

	mux.HandleFunc("/logstring", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Error: %s", err.Error())
			return
		}
		debugConfig := strings.TrimSpace(string(body))
		SetGlobalDebugConfig(debugConfig)

		fmt.Fprintf(w, "New log config: %s", debugConfig)
	})
	Warnf("profileserver startet on port %d", port)
	Error(http.ListenAndServe(fmt.Sprintf(":%d", port), mux))
}
