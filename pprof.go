//go:build logpprof
// +build logpprof

package env_logger

import (
	"fmt"
	"github.com/mattn/go-colorable"
	logrus "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"strings"
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
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Error: "+err.Error())
			return
		}
		debugConfig := strings.TrimSpace(string(body))

		logger := logrus.New()

		logger.Formatter.(*logrus.TextFormatter).EnvironmentOverrideColors = true
		logger.SetOutput(colorable.NewColorableStdout()) // make default work on windows
		ConfigureAllLoggers(logger, debugConfig)

		fmt.Fprintf(w, "New log config: %s", debugConfig)
	})
	Error(http.ListenAndServe(":11111", nil))
}
