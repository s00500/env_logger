package env_logger

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	logrus "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// A logstring that can not be understood must not take the program down
func TestLogstringEndpoint(t *testing.T) {
	defer SetGlobalDebugConfig("")

	mux := newProfileMux()

	for _, config := range []string{"a=b=c", "nonsense", "pkg=debug"} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/logstring", strings.NewReader(config)))
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "New log config: "+config, recorder.Body.String())
	}
	assert.Equal(t, logrus.DebugLevel, activeSet.Load().levels["pkg"])

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/logstring", strings.NewReader(strings.Repeat("x", maxLogstringSize+1))))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, logrus.DebugLevel, activeSet.Load().levels["pkg"])
}

// Prefixes made up at runtime must not grow the entry cache forever
func TestModuleEntryCacheIsBounded(t *testing.T) {
	logger := logrus.New()
	logger.Out = &bytes.Buffer{}
	ConfigureAllLoggers(logger, "")

	for i := 0; i < 2*maxModuleEntries; i++ {
		GetLoggerForPrefix(fmt.Sprintf("conn-%d", i))
	}

	cached := 0
	activeSet.Load().moduleEntries.entries.Range(func(_, _ interface{}) bool {
		cached++
		return true
	})
	assert.LessOrEqual(t, cached, maxModuleEntries)
}
