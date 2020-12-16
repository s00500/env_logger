package testutils

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	. "github.com/s00500/env_logger"
	"github.com/sirupsen/logrus"

	"github.com/stretchr/testify/require"
)

func LogAndAssertJSON(t *testing.T, log func(logrus.FieldLogger), assertions func(fields logrus.Fields)) {
	var buffer bytes.Buffer
	var fields logrus.Fields

	loggerMain := logrus.New()
	loggerMain.Out = &buffer
	loggerMain.Formatter = new(logrus.JSONFormatter)

	ConfigureAllLoggers(loggerMain, "info")

	logger := GetLoggerForPrefix("Testing")
	log(logger)

	err := json.Unmarshal(buffer.Bytes(), &fields)
	require.Nil(t, err)

	assertions(fields)
}

func LogAndAssertText(t *testing.T, log func(logrus.FieldLogger), assertions func(fields map[string]string)) {
	var buffer bytes.Buffer

	loggerMain := logrus.New()
	loggerMain.Out = &buffer
	loggerMain.Formatter = &logrus.TextFormatter{
		DisableColors: true,
	}

	ConfigureAllLoggers(loggerMain, "info")

	logger := GetLoggerForPrefix("Testing")

	log(logger)

	fields := make(map[string]string)
	for _, kv := range strings.Split(strings.TrimRight(buffer.String(), "\n"), " ") {
		if !strings.Contains(kv, "=") {
			continue
		}
		kvArr := strings.Split(kv, "=")
		key := strings.TrimSpace(kvArr[0])
		val := kvArr[1]
		if kvArr[1][0] == '"' {
			var err error
			val, err = strconv.Unquote(val)
			require.NoError(t, err)
		}
		fields[key] = val
	}
	assertions(fields)
}
