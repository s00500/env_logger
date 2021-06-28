// +build !logpprof

package env_logger

func profileServer() {
	Warn("pprof server not included at compiletime")
}
