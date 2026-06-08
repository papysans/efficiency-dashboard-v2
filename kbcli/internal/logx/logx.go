// Package logx 是 kbcli 的轻量日志（从 main 的 logger.go 抽出，去掉全局散落）。
// 用法：Init() 初始化；Infof/Warnf/Errorf/... 包级便捷函数；Close() 收尾。
package logx

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

// Init 初始化全局 logger（原 InitLogger）。
func Init(consoleLevelStr, logFile, fileLevelStr string) error {
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

// Close 关闭日志文件句柄（nil-safe）。
func Close() {
	if logger != nil {
		logger.Close()
	}
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

func (l *Logger) prompt(msg string) {
	if l == nil {
		fmt.Fprintln(os.Stderr, msg)
		return
	}
	fmt.Fprintln(os.Stderr, msg)
	if l.file != nil {
		l.mu.Lock()
		fmt.Fprintln(l.file, msg)
		l.mu.Unlock()
	}
}

// 包级便捷函数（原 main 的 logXxx）。
func Debug(msg string)                          { logger.log(LogDebug, msg) }
func Info(msg string)                           { logger.log(LogInfo, msg) }
func Warn(msg string)                           { logger.log(LogWarn, msg) }
func Error(msg string)                          { logger.log(LogError, msg) }
func Debugf(format string, args ...interface{}) { logger.log(LogDebug, fmt.Sprintf(format, args...)) }
func Infof(format string, args ...interface{})  { logger.log(LogInfo, fmt.Sprintf(format, args...)) }
func Warnf(format string, args ...interface{})  { logger.log(LogWarn, fmt.Sprintf(format, args...)) }
func Errorf(format string, args ...interface{}) { logger.log(LogError, fmt.Sprintf(format, args...)) }
func Prompt(msg string)                         { logger.prompt(msg) }
func Promptf(format string, args ...interface{}) {
	logger.prompt(fmt.Sprintf(format, args...))
}

// PromptProgress 进度点（进度条长度 linecnt，到末尾复位）。
func PromptProgress(cnt, linecnt int) {
	if logger.consoleLevel < LogInfo {
		return
	}
	if linecnt < 2 {
		linecnt = 2
	}
	fmt.Print(".")
	if cnt%linecnt == linecnt-1 {
		fmt.Print("\r\033[K")
	}
}

// Progress 在同一行刷新 "label done/total (pct%)"，done>=total 时换行收尾。
func Progress(label string, done, total, step int) {
	if logger.consoleLevel < LogInfo {
		return
	}
	if step < 1 {
		step = 1
	}
	if done != total && done%step != 0 {
		return
	}
	pct := 0
	if total > 0 {
		pct = done * 100 / total
	}
	fmt.Printf("\r\033[K%s %d/%d (%d%%)", label, done, total, pct)
	if done >= total {
		fmt.Println()
	}
}
