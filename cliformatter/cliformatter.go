package cliformatter

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

const (
	reset = 0

	colorBlack   = 30
	colorBlue    = 34
	colorRed     = 31
	colorGreen   = 32
	colorYellow  = 33
	colorMagenta = 35
	colorCyan    = 36
	colorWhite   = 37

	colorGray          = 90
	colorBlueBright    = 94
	colorRedBright     = 91
	colorGreenBright   = 92
	colorYellowBright  = 93
	colorMagentaBright = 95
	colorCyanBright    = 96
	colorWhiteBright   = 97

	bgBlack   = 40
	bgBlue    = 44
	bgRed     = 41
	bgGreen   = 42
	bgYellow  = 43
	bgMagenta = 45
	bgCyan    = 46
	bgWhite   = 47

	bgBlackBright   = 100
	bgBlueBright    = 104
	bgRedBright     = 101
	bgGreenBright   = 102
	bgYellowBright  = 103
	bgMagentaBright = 105
	bgCyanBright    = 106
	bgWhiteBright   = 107

	modifierBold       = 1
	modifierDim        = 2
	modifierUnderscore = 4
	modifierBlink      = 5
	modifierReverse    = 7
	modifierHidden     = 8
)

// Formatter implements logrus.Formatter interface.
type Formatter struct {
	PrintFields        bool
	DisablePrintErrors bool
}

func getLevelMarkup(level logrus.Level) (icon string, color int) {
	switch level {
	case logrus.PanicLevel:
		return "🤯", colorRed
	case logrus.FatalLevel:
		return "💀", colorRed
	case logrus.ErrorLevel:
		return "🛑", colorRedBright
	case logrus.WarnLevel:
		return "🚸", colorYellowBright
	case logrus.InfoLevel:
		return "▶ ", 0
	case logrus.DebugLevel:
		return "🐛", colorGreenBright
	case logrus.TraceLevel:
		return "🔧", 0
	default:
		return "▶ ", 0
	}
}

// Format building log message.
func (f *Formatter) Format(entry *logrus.Entry) ([]byte, error) {
	icon, color := getLevelMarkup(entry.Level)
	output := ""
	if color == 0 {
		output = fmt.Sprintf("%s %s\t", icon, entry.Message)
	} else {
		output = fmt.Sprintf("%s \x1b[%dm%s\x1b[0m\t", icon, color, entry.Message)
	}

	for k, v := range entry.Data {
		if f.PrintFields || !f.DisablePrintErrors && k == "error" {
			output = fmt.Sprintf("%s \x1b[%dm%s\x1b[0m=%v", output, color, k, v)
		}
	}

	output = fmt.Sprintf("%s\n", output)

	return []byte(output), nil
}
