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
	Error(http.ListenAndServe(":11111", nil))
}
