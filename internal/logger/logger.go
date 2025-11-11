package logger

import (
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var globalLogger zerolog.Logger

// Init 初始化日志
func Init(cfg LogConfig) {
	var writers []io.Writer

	// 设置输出
	if cfg.Output == "stdout" || cfg.Output == "" {
		writers = append(writers, os.Stdout)
	} else {
		file, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal().Err(err).Msg("无法打开日志文件")
		}
		writers = append(writers, file)
	}

	// 设置格式
	var writer io.Writer
	if cfg.Format == "text" {
		writer = zerolog.ConsoleWriter{Out: os.Stdout}
	} else {
		writer = io.MultiWriter(writers...)
	}

	// 设置级别
	level, err := zerolog.ParseLevel(strings.ToLower(cfg.Level))
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// 创建 logger
	globalLogger = zerolog.New(writer).
		With().
		Timestamp().
		Logger()

	// 设置全局 logger
	log.Logger = globalLogger
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `yaml:"level" mapstructure:"level"`
	Format string `yaml:"format" mapstructure:"format"`
	Output string `yaml:"output" mapstructure:"output"`
}

// WithTraceID 添加 trace_id
func WithTraceID(traceID string) zerolog.Logger {
	return globalLogger.With().Str("trace_id", traceID).Logger()
}

// Error 返回错误级别日志
func Error() *zerolog.Event {
	return globalLogger.Error()
}

// Info 返回信息级别日志
func Info() *zerolog.Event {
	return globalLogger.Info()
}

// Debug 返回调试级别日志
func Debug() *zerolog.Event {
	return globalLogger.Debug()
}

// Warn 返回警告级别日志
func Warn() *zerolog.Event {
	return globalLogger.Warn()
}

// Fatal 返回致命级别日志
func Fatal() *zerolog.Event {
	return globalLogger.Fatal()
}

