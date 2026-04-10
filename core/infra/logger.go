package infra

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/lib/pq"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger 全局日志器实例
var Logger *CustomLogger

// CustomLogger 自定义日志器
type CustomLogger struct {
	logger        *log.Logger
	consoleLogger *log.Logger
	level         LogLevel
	consoleMode   bool
	consoleLevel  LogLevel
}

// LogLevel 日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level      string `yaml:"level"`
	File       string `yaml:"file"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
}

// InitLogger 初始化日志
func InitLogger(config LoggingConfig, consoleMode bool, consoleLogLevel string) {
	// 创建日志目录
	logDir := filepath.Dir(config.File)
	if logDir != "" && logDir != "." {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			log.Fatalf("创建日志目录失败: %v", err)
		}
	}

	// 配置日志轮转
	logWriter := &lumberjack.Logger{
		Filename:   config.File,
		MaxSize:    config.MaxSize, // MB
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge, // days
		Compress:   true,
	}

	// 解析日志级别
	level := parseLogLevel(config.Level)

	// 解析控制台日志级别
	consoleLevel := parseLogLevel(consoleLogLevel)

	Logger = &CustomLogger{
		logger:        log.New(logWriter, "", log.LstdFlags|log.Lshortfile),
		level:         level,
		consoleMode:   consoleMode,
		consoleLevel:  consoleLevel,
		consoleLogger: log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile), // 始终创建consoleLogger
	}
}

// parseLogLevel 解析日志级别
func parseLogLevel(levelStr string) LogLevel {
	switch levelStr {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn":
		return WARN
	case "error":
		return ERROR
	case "fatal":
		return FATAL
	default:
		return DEBUG // 默认返回最低级别
	}
}

// getLevelFromString 从字符串获取日志级别枚举
func getLevelFromString(level string) LogLevel {
	switch level {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	case "FATAL":
		return FATAL
	default:
		return INFO
	}
}

// Debug 调试日志
func (l *CustomLogger) Debug(format string, v ...interface{}) {
	if l.level <= DEBUG {
		l.log("DEBUG", format, v...)
	}
}

// Info 信息日志
func (l *CustomLogger) Info(format string, v ...interface{}) {
	if l.level <= INFO {
		l.log("INFO", format, v...)
	}
}

// Warn 警告日志
func (l *CustomLogger) Warn(format string, v ...interface{}) {
	if l.level <= WARN {
		l.log("WARN", format, v...)
	}
}

// Error 错误日志
func (l *CustomLogger) Error(format string, v ...interface{}) {
	if l.level <= ERROR {
		l.log("ERROR", format, v...)
	}
}

// Fatal 致命错误日志
func (l *CustomLogger) Fatal(format string, v ...interface{}) {
	l.log("FATAL", format, v...)
	os.Exit(1)
}

// log 内部日志方法
func (l *CustomLogger) log(level string, format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	logMsg := fmt.Sprintf("[%s] %s", level, msg)

	// 写入文件日志
	l.logger.Output(3, logMsg)

	// 判断是否输出到控制台
	shouldOutputToConsole := false
	currentLevel := getLevelFromString(level)

	// 日志级别数值：DEBUG(0) < INFO(1) < WARN(2) < ERROR(3) < FATAL(4)
	// ERROR和FATAL级别总是输出到控制台
	if currentLevel >= ERROR {
		shouldOutputToConsole = true
	} else if l.consoleMode && l.consoleLogger != nil {
		// 其他级别只有在启用控制台模式时才根据设定的consoleLevel判断
		if currentLevel >= l.consoleLevel {
			shouldOutputToConsole = true
		}
	}

	if shouldOutputToConsole && l.consoleLogger != nil {
		l.consoleLogger.Output(3, logMsg)
	}
}

// ErrorDetailed 输出详细的错误信息（多层嵌套）
func (l *CustomLogger) ErrorDetailed(prefix string, err error) {
	if err == nil {
		return
	}

	// 格式化错误链，显示每一层错误的详细信息
	errorStack := formatErrorStack(err)

	// 输出到文件
	l.logger.Output(3, fmt.Sprintf("[ERROR] %s\n%s", prefix, errorStack))

	// ERROR级别总是输出到控制台
	if l.consoleLogger != nil {
		l.consoleLogger.Output(3, fmt.Sprintf("[ERROR] %s\n%s", prefix, errorStack))
	}
}

// formatErrorStack 格式化错误堆栈，显示嵌套的错误原因
func formatErrorStack(err error) string {
	var result strings.Builder

	// 提取根本原因
	rootCause := getRootCause(err)

	// 如果是数据库错误，提取关键信息
	if dbErr, ok := rootCause.(*pq.Error); ok {
		result.WriteString(fmt.Sprintf("  类型: 数据库错误\n"))
		result.WriteString(fmt.Sprintf("  代码: %s\n", dbErr.Code))
		result.WriteString(fmt.Sprintf("  约束: %s\n", dbErr.Constraint))
		result.WriteString(fmt.Sprintf("  表: %s\n", dbErr.Table))
		result.WriteString(fmt.Sprintf("  列: %s\n", dbErr.Column))
		result.WriteString(fmt.Sprintf("  详情: %s", dbErr.Detail))
	} else {
		// 普通错误，输出完整的错误链
		result.WriteString(fmt.Sprintf("  错误: %s", err.Error()))

		// 如果有深层错误，显示调用栈
		if wrappedErr := errors.Unwrap(err); wrappedErr != nil {
			result.WriteString(fmt.Sprintf("\n  原因: %s", wrappedErr.Error()))
		}
	}

	return result.String()
}

// getRootCause 获取错误的根本原因
func getRootCause(err error) error {
	for {
		wrappedErr := errors.Unwrap(err)
		if wrappedErr == nil {
			return err
		}
		err = wrappedErr
	}
}
