package logger

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// traceIDKey 用于在 context 中存储 trace_id 的键
type traceIDKey struct{}

// WithTraceIDContext 将 trace_id 添加到 context
func WithTraceIDContext(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

// TraceIDFromContext 从 context 中获取 trace_id
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if traceID, ok := ctx.Value(traceIDKey{}).(string); ok {
		return traceID
	}
	return ""
}

var globalLogger zerolog.Logger

// Init 初始化日志
func Init(cfg LogConfig) {
	var writers []io.Writer

	// 设置输出
	if cfg.Output == "stdout" || cfg.Output == "" {
		writers = append(writers, os.Stdout)
	} else {
		// #nosec G302 -- 日志文件需要组可读权限，0600 可能过于严格
		// 在生产环境中，建议使用 0640 或通过文件系统 ACL 控制访问
		file, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
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

// FromContext 从 context 创建带 trace_id 的 logger
func FromContext(ctx context.Context) *zerolog.Logger {
	traceID := TraceIDFromContext(ctx)
	if traceID != "" {
		logger := globalLogger.With().Str("trace_id", traceID).Logger()
		return &logger
	}
	return &globalLogger
}

// Error 返回错误级别日志
func Error() *zerolog.Event {
	return globalLogger.Error()
}

// ErrorCtx 从 context 返回错误级别日志（包含 trace_id）
func ErrorCtx(ctx context.Context) *zerolog.Event {
	return FromContext(ctx).Error()
}

// Info 返回信息级别日志
func Info() *zerolog.Event {
	return globalLogger.Info()
}

// InfoCtx 从 context 返回信息级别日志（包含 trace_id）
func InfoCtx(ctx context.Context) *zerolog.Event {
	return FromContext(ctx).Info()
}

// Debug 返回调试级别日志
func Debug() *zerolog.Event {
	return globalLogger.Debug()
}

// DebugCtx 从 context 返回调试级别日志（包含 trace_id）
func DebugCtx(ctx context.Context) *zerolog.Event {
	return FromContext(ctx).Debug()
}

// Warn 返回警告级别日志
func Warn() *zerolog.Event {
	return globalLogger.Warn()
}

// WarnCtx 从 context 返回警告级别日志（包含 trace_id）
func WarnCtx(ctx context.Context) *zerolog.Event {
	return FromContext(ctx).Warn()
}

// Fatal 返回致命级别日志
func Fatal() *zerolog.Event {
	return globalLogger.Fatal()
}

// FatalCtx 从 context 返回致命级别日志（包含 trace_id）
func FatalCtx(ctx context.Context) *zerolog.Event {
	return FromContext(ctx).Fatal()
}
