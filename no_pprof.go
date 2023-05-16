//go:build !logpprof
// +build !logpprof

package env_logger

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

func profileServer(port uint16) {
	Warn("pprof server not included at compiletime")

	// make dynamic config still work
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "")
	})

	http.HandleFunc("/logstring", func(w http.ResponseWriter, r *http.Request) {
		// function to allow dynamicaly setting the logstring
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Error: "+err.Error())
			return
		}
		debugConfig := strings.TrimSpace(string(body))
		SetGlobalDebugConfig(debugConfig)

		fmt.Fprintf(w, "New log config: %s", debugConfig)
	})
	Warnf("profileserver startet on port %d", port)
	Error(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
