package env_logger

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mattn/go-colorable"
	logrus "github.com/sirupsen/logrus"
)

// SetGlobalDebugConfig overrides the debug config, but with default logger at runtime
func SetGlobalDebugConfig(debugConfig string) {
	logger := logrus.New()

	logger.Formatter.(*logrus.TextFormatter).EnvironmentOverrideColors = true
	logger.SetOutput(colorable.NewColorableStdout()) // make default work on windows
	ConfigureAllLoggers(logger, debugConfig)
}

// maxLogstringSize limits the body accepted by the /logstring endpoint
const maxLogstringSize = 1 << 16

func profileServer(port uint16) {
	mux := newProfileMux()
	Warnf("profileserver startet on port %d", port)
	Error(http.ListenAndServe(fmt.Sprintf(":%d", port), mux))
}

func newProfileMux() *http.ServeMux {
	// Own mux, to neither collide with nor expose what the application
	// registers on http.DefaultServeMux.
	mux := http.NewServeMux()

	// only does something if built with the logpprof tag
	registerPprof(mux)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "")
	})

	mux.HandleFunc("/logstring", func(w http.ResponseWriter, r *http.Request) {
		// function to allow dynamicaly setting the logstring
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxLogstringSize))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Error: %s", err.Error())
			return
		}
		debugConfig := strings.TrimSpace(string(body))
		SetGlobalDebugConfig(debugConfig)

		fmt.Fprintf(w, "New log config: %s", debugConfig)
	})
	return mux
}
