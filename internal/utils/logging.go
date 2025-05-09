package utils

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// LogLevel represents logging levels
type LogLevel int

// Log levels
const (
	LogError LogLevel = iota
	LogWarning
	LogInfo
	LogDebug
)

// Global log level and writer with mutex for thread safety
var (
	logLevel               = LogWarning // Default to warnings
	logWriter    io.Writer = os.Stdout
	loggerMutex  sync.Mutex
	logTimestamp = true
)

// SetLogLevel sets the global logging level
func SetLogLevel(level LogLevel) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logLevel = level
}

// GetLogLevel returns the current global logging level
func GetLogLevel() LogLevel {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	return logLevel
}

// SetLogWriter sets the writer for log output
func SetLogWriter(writer io.Writer) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logWriter = writer
}

// EnableTimestamp enables or disables timestamps in log messages
func EnableTimestamp(enable bool) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logTimestamp = enable
}

// Logf logs a message at the specified level
func Logf(level LogLevel, format string, args ...interface{}) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()

	if level <= logLevel {
		prefix := ""
		switch level {
		case LogError:
			prefix = "ERROR: "
		case LogWarning:
			prefix = "WARNING: "
		case LogInfo:
			prefix = "INFO: "
		case LogDebug:
			prefix = "DEBUG: "
		}

		if logTimestamp {
			timestamp := time.Now().Format("2006-01-02 15:04:05.000")
			prefix = timestamp + " " + prefix
		}

		fmt.Fprintf(logWriter, prefix+format+"\n", args...)
	}
}

// LogErrorf logs an error message
func LogErrorf(format string, args ...interface{}) {
	Logf(LogError, format, args...)
}

// LogWarningf logs a warning message
func LogWarningf(format string, args ...interface{}) {
	Logf(LogWarning, format, args...)
}

// LogInfof logs an info message
func LogInfof(format string, args ...interface{}) {
	Logf(LogInfo, format, args...)
}

// LogDebugf logs a debug message
func LogDebugf(format string, args ...interface{}) {
	Logf(LogDebug, format, args...)
}

// LogIfError logs an error message if err is not nil
func LogIfError(err error, format string, args ...interface{}) bool {
	if err != nil {
		combined := fmt.Sprintf("%s: %v", format, err)
		Logf(LogError, combined, args...)
		return true
	}
	return false
}

// NewLogger creates a new logger with a specific prefix
type Logger struct {
	prefix string
	level  LogLevel
}

// NewLogger creates a new logger with a specific prefix
func NewLogger(prefix string) *Logger {
	return &Logger{
		prefix: prefix,
		level:  logLevel,
	}
}

// SetLevel sets the log level for this logger
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	if l.level >= LogError {
		Logf(LogError, l.prefix+": "+format, args...)
	}
}

// Warning logs a warning message
func (l *Logger) Warning(format string, args ...interface{}) {
	if l.level >= LogWarning {
		Logf(LogWarning, l.prefix+": "+format, args...)
	}
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	if l.level >= LogInfo {
		Logf(LogInfo, l.prefix+": "+format, args...)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level >= LogDebug {
		Logf(LogDebug, l.prefix+": "+format, args...)
	}
}

// LogToFile sets up logging to a file
func LogToFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}

	SetLogWriter(file)
	return nil
}

// LogToFileAndConsole sets up logging to both a file and console
func LogToFileAndConsole(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}

	// Create a multi-writer
	mw := io.MultiWriter(file, os.Stdout)
	SetLogWriter(mw)
	return nil
}

// NewError creates a new error with formatting
func NewError(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}
