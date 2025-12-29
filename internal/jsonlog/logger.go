package jsonlog

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// Level represents the severity level for a log entry.
type Level int8

const (
	LevelInfo Level = iota
	LevelError
	LevelFatal
	LevelOff
)

func (l Level) String() string {
	switch l {
	case LevelInfo:
		return "INFO"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	case LevelOff:
		return "OFF"
	default:
		return ""
	}
}

// ParseLevel parses a log level string (case-insensitive).
func ParseLevel(s string) (Level, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "", "INFO":
		return LevelInfo, nil
	case "ERROR":
		return LevelError, nil
	case "FATAL":
		return LevelFatal, nil
	case "OFF":
		return LevelOff, nil
	default:
		return LevelInfo, errors.New("invalid log level (allowed: INFO, ERROR, FATAL, OFF)")
	}
}

// Logger writes structured JSON logs.
type Logger struct {
	out      io.Writer
	minLevel Level

	traceFor Level

	mu sync.Mutex
}

func New(out io.Writer, minLevel Level) *Logger {
	return &Logger{
		out:      out,
		minLevel: minLevel,
		traceFor: LevelFatal,
	}
}

func (l *Logger) SetTraceFor(level Level) {
	l.mu.Lock()
	l.traceFor = level
	l.mu.Unlock()
}

func (l *Logger) PrintInfo(message string, properties map[string]string) {
	l.print(LevelInfo, message, properties, false)
}

func (l *Logger) PrintError(err error, properties map[string]string) {
	if err == nil {
		return
	}
	// it's better to not print the stack trace on every error in a high-throughput service. but i am doing it for now for debugging purposes.
	// Avoid stack traces on every error in a high-throughput service.
	l.print(LevelError, err.Error(), properties, false)
}

func (l *Logger) PrintErrorWithTrace(err error, properties map[string]string) {
	if err == nil {
		return
	}
	l.print(LevelError, err.Error(), properties, true)
}

func (l *Logger) PrintFatal(err error, properties map[string]string) {
	if err == nil {
		os.Exit(1)
	}
	l.print(LevelFatal, err.Error(), properties, true)
	os.Exit(1)
}

type entry struct {
	Level      string            `json:"level"`
	Time       string            `json:"time"`
	Message    string            `json:"message"`
	Properties map[string]string `json:"properties,omitempty"`
	Trace      string            `json:"trace,omitempty"`
}

func (l *Logger) print(level Level, message string, properties map[string]string, forceTrace bool) (int, error) {
	l.mu.Lock()
	minLevel := l.minLevel
	traceFor := l.traceFor
	l.mu.Unlock()

	if level < minLevel || minLevel == LevelOff {
		return 0, nil
	}

	e := entry{
		Level:      level.String(),
		Time:       time.Now().UTC().Format(time.RFC3339Nano),
		Message:    message,
		Properties: properties,
	}

	if forceTrace || (traceFor != LevelOff && level >= traceFor) {
		e.Trace = string(debug.Stack())
	}

	line, err := json.Marshal(e)
	if err != nil {
		line = []byte(LevelError.String() + `: unable to marshal log entry: ` + err.Error())
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	return l.out.Write(append(line, '\n'))
}

func (l *Logger) Write(p []byte) (n int, err error) {
	return l.print(LevelError, strings.TrimSpace(string(p)), nil, false)
}
