//go:build !logpprof
// +build !logpprof

package env_logger

func profileServer(port uint16) {
	Warn("pprof server not included at compiletime")
}
