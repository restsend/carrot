package carrot

import (
	"fmt"
	"log"
	"os"

	"github.com/mattn/go-isatty"
)

const (
	green   = "\033[97;42m"
	white   = "\033[90;47m"
	yellow  = "\033[90;43m"
	red     = "\033[97;41m"
	blue    = "\033[97;44m"
	magenta = "\033[97;45m"
	cyan    = "\033[97;46m"
	reset   = "\033[0m"
)

const (
	LevelDebug = iota
	LevelInfo
	LevelWarning
	LevelError
)

var Loglevel = LevelDebug
var EnabledConsoleColor = false

func colorize(color, message any) string {
	if !EnabledConsoleColor {
		return fmt.Sprintf("%v", message)
	}
	return fmt.Sprintf("%s%v%s", color, message, reset)
}

func colorLevel(level int) string {
	switch level {
	case LevelDebug:
		return colorize(blue, "DEBUG")
	case LevelInfo:
		return colorize(green, "INFO")
	case LevelWarning:
		return colorize(yellow, "WARNING")
	case LevelError:
		return colorize(red, "ERROR")
	default:
		return colorize(white, "???")
	}
}

func SetLogLevel(level int) {
	Loglevel = level
	out := log.Default().Writer()
	if w, ok := out.(*os.File); !ok || os.Getenv("TERM") == "dumb" ||
		(!isatty.IsTerminal(w.Fd()) && !isatty.IsCygwinTerminal(w.Fd())) {
		EnabledConsoleColor = false
	} else {
		EnabledConsoleColor = true
	}
}

func Debug(v ...interface{}) {
	if Loglevel <= LevelDebug {
		log.Default().Output(2, fmt.Sprintf("%s %v", colorLevel(LevelDebug), fmt.Sprintln(v...)))
	}
}

func Info(v ...interface{}) {
	if Loglevel <= LevelInfo {
		log.Default().Output(2, fmt.Sprintf("%s %v", colorLevel(LevelInfo), fmt.Sprintln(v...)))
	}
}

func Warning(v ...interface{}) {
	if Loglevel <= LevelWarning {
		log.Default().Output(2, fmt.Sprintf("%s %v", colorLevel(LevelWarning), fmt.Sprintln(v...)))
	}
}

func Error(v ...interface{}) {
	if Loglevel <= LevelError {
		log.Default().Output(2, fmt.Sprintf("[%s] %v", colorLevel(LevelError), fmt.Sprintln(v...)))
	}
}
