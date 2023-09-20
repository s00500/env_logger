package env_logger

import (
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

// FUnction to list all modules that have logged ? kinda hard to do...
func ListModules() {

}
