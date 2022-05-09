package logger

import (
	"log"
	"os"
)

const (
	ERROR = 1 << 4
	WARNING = 1 << 3
	INFO = 1 << 2
	DEBUG = 1 << 1
)

var (
	ErrorLogger *log.Logger
	WarningLogger *log.Logger
	InfoLogger *log.Logger
	DebugLogger *log.Logger
	LogLevel int
)

func init() {
	ErrorLogger = log.New(os.Stderr, "[Error]: ", log.Ldate|log.Ltime)
	WarningLogger = log.New(os.Stderr, "[Warning]: ", log.Ldate|log.Ltime)
	InfoLogger = log.New(os.Stderr, "[Info]: ", log.Ldate|log.Ltime)
	DebugLogger = log.New(os.Stderr, "[Debug]: ", log.Ldate|log.Ltime)
}

func Debug(v ...interface{}) {
	if LogLevel <= DEBUG {
		DebugLogger.Println(v...)
	}
}

func Info(v ...interface{}) {
	if LogLevel <= INFO {
		InfoLogger.Println(v...)
	}
}

func Warn(v ...interface{}) {
	if LogLevel <= WARNING {
		WarningLogger.Println(v...)
	}
}

func Error(v ...interface{}) {
	if LogLevel <= ERROR {
		ErrorLogger.Println(v...)
	}
	os.Exit(1)
}

func Debugf(format string, v ...interface{}) {
	if LogLevel <= DEBUG {
		DebugLogger.Printf(format, v...)
	}
}

func Infof(format string, v ...interface{}) {
	if LogLevel <= INFO {
		InfoLogger.Printf(format, v...)
	}
}

func Warnf(format string, v ...interface{}) {
	if LogLevel <= WARNING {
		WarningLogger.Printf(format, v...)
	}
}

func Errorf(format string, v ...interface{}) {
	if LogLevel <= ERROR {
		ErrorLogger.Printf(format, v...)
	}
	os.Exit(1)
}