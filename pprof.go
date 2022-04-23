//go:build logpprof
// +build logpprof

package env_logger

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
)

func init() {

}

func profileServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "")
	})

	http.HandleFunc("/logstring", func(w http.ResponseWriter, r *http.Request) {
		// function to allow dynamicaly setting the logstring
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			ServeJSON(w, r, "error", err.Error(), http.StatusBadRequest)
			return
		}
		debugConfig := string(strings.TrimSpace(body))

		logger := logrus.New()

		logger.Formatter.(*logrus.TextFormatter).EnvironmentOverrideColors = true
		logger.SetOutput(colorable.NewColorableStdout()) // make default work on windows
		ConfigureAllLoggers(logger, debugConfig)

		fmt.Fprintf(w, "New log config: ", debugConfig)
	})
	Error(http.ListenAndServe(":11111", nil))
}
