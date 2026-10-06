package cliformatter

import (
	"bytes"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
)

func format(t *testing.T, f *Formatter, level logrus.Level) string {
	var buffer bytes.Buffer
	logger := logrus.New()
	logger.Out = &buffer
	logger.Formatter = f
	logger.SetLevel(logrus.TraceLevel)

	logger.WithFields(logrus.Fields{"b": 2, "a": "one", "c": true}).WithError(errors.New("broken")).Log(level, "message")
	return buffer.String()
}

func TestFormat(t *testing.T) {
	for _, test := range []struct {
		name      string
		formatter Formatter
		level     logrus.Level
		expected  string
	}{
		{"plain", Formatter{DisablePrintErrors: true}, logrus.InfoLevel,
			"▶  message\t\n"},
		{"colored", Formatter{DisablePrintErrors: true}, logrus.WarnLevel,
			"🚸 \x1b[93mmessage\x1b[0m\t\n"},
		{"error only", Formatter{}, logrus.WarnLevel,
			"🚸 \x1b[93mmessage\x1b[0m\t \x1b[93merror\x1b[0m=broken\n"},
		{"fields sorted", Formatter{PrintFields: true}, logrus.InfoLevel,
			"▶  message\t \x1b[0ma\x1b[0m=one \x1b[0mb\x1b[0m=2 \x1b[0mc\x1b[0m=true \x1b[0merror\x1b[0m=broken\n"},
	} {
		for i := 0; i < 5; i++ {
			if got := format(t, &test.formatter, test.level); got != test.expected {
				t.Fatalf("%s:\n got %q\nwant %q", test.name, got, test.expected)
			}
		}
	}
}

func BenchmarkFormat(b *testing.B) {
	f := &Formatter{PrintFields: true}
	logger := logrus.New()
	entry := logger.WithFields(logrus.Fields{"module": "pkg", "a": "one", "b": 2})
	entry.Message = "hello world"
	entry.Level = logrus.WarnLevel
	entry.Buffer = &bytes.Buffer{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry.Buffer.Reset()
		f.Format(entry)
	}
}
