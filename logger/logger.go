package logger

import (
	"log"
	"os"
)

type Logger struct {
	info  *log.Logger
	error *log.Logger
}

func NewLogger() *Logger {
	return &Logger{
		info:  log.New(os.Stdout, "INFO: ", log.LstdFlags),
		error: log.New(os.Stderr, "ERROR: ", log.LstdFlags),
	}
}

func (l *Logger) Info(format string, args ...any) {
	l.info.Printf(format, args...)
}

func (l *Logger) Error(format string, args ...any) {
	l.error.Printf(format, args...)
}
