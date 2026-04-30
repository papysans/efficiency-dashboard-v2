package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type LogLevel int

const (
	LogDebug LogLevel = 1
	LogInfo  LogLevel = 2
	LogWarn  LogLevel = 3
	LogError LogLevel = 4
)

func parseLogLevel(s string) LogLevel {
	switch strings.ToLower(s) {
	case "debug":
		return LogDebug
	case "info":
		return LogInfo
	case "warn":
		return LogWarn
	case "error":
		return LogError
	default:
		return LogDebug
	}
}

func (l LogLevel) String() string {
	switch l {
	case LogDebug:
		return "DEBUG"
	case LogInfo:
		return "INFO"
	case LogWarn:
		return "WARN"
	case LogError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

type Logger struct {
	consoleLevel LogLevel
	fileLevel    LogLevel
	file         *os.File
	mu           sync.Mutex
}

var logger *Logger

func InitLogger(consoleLevelStr, logFile, fileLevelStr string) error {
	l := &Logger{
		consoleLevel: parseLogLevel(consoleLevelStr),
		fileLevel:    parseLogLevel(fileLevelStr),
	}

	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("打开日志文件失败: %w", err)
		}
		l.file = f
	}

	logger = l
	return nil
}

func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
}

func (l *Logger) log(level LogLevel, msg string) {
	if l == nil {
		if level >= LogWarn {
			fmt.Fprintln(os.Stderr, msg)
		}
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] [%s] %s", timestamp, level.String(), msg)

	if level >= l.consoleLevel {
		if level >= LogError {
			fmt.Fprintln(os.Stderr, line)
		} else {
			fmt.Println(line)
		}
	}

	if l.file != nil && level >= l.fileLevel {
		l.mu.Lock()
		fmt.Fprintln(l.file, line)
		l.mu.Unlock()
	}
}

func (l *Logger) Debug(msg string) { l.log(LogDebug, msg) }
func (l *Logger) Info(msg string)  { l.log(LogInfo, msg) }
func (l *Logger) Warn(msg string)  { l.log(LogWarn, msg) }
func (l *Logger) Error(msg string) { l.log(LogError, msg) }
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(LogDebug, fmt.Sprintf(format, args...))
}
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(LogInfo, fmt.Sprintf(format, args...))
}
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(LogWarn, fmt.Sprintf(format, args...))
}
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(LogError, fmt.Sprintf(format, args...))
}

// 便捷函数，供全局调用
func logDebug(msg string)                          { logger.Debug(msg) }
func logInfo(msg string)                           { logger.Info(msg) }
func logWarn(msg string)                           { logger.Warn(msg) }
func logError(msg string)                          { logger.Error(msg) }
func logDebugf(format string, args ...interface{}) { logger.Debugf(format, args...) }
func logInfof(format string, args ...interface{})  { logger.Infof(format, args...) }
func logWarnf(format string, args ...interface{})  { logger.Warnf(format, args...) }
func logErrorf(format string, args ...interface{}) { logger.Errorf(format, args...) }
