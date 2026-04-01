package logging

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	charmlog "charm.land/log/v2"
)

const (
	EnvLogLevel        = "STACKCTL_LOG_LEVEL"
	EnvLogFormat       = "STACKCTL_LOG_FORMAT"
	EnvLogFile         = "STACKCTL_LOG_FILE"
	EnvTUIDebugLogFile = "STACKCTL_TUI_DEBUG_LOG_FILE"
)

type loggerState struct {
	logger  *charmlog.Logger
	enabled bool
	closer  io.Closer
}

var (
	loggerOnce sync.Once
	state      loggerState
)

func Logger() *charmlog.Logger {
	loggerOnce.Do(initLogger)
	return state.logger
}

func Enabled() bool {
	loggerOnce.Do(initLogger)
	return state.enabled
}

func With(keyvals ...any) *charmlog.Logger {
	return Logger().With(keyvals...)
}

func Debug(msg any, keyvals ...any) {
	if Enabled() {
		Logger().Debug(msg, keyvals...)
	}
}

func Info(msg any, keyvals ...any) {
	if Enabled() {
		Logger().Info(msg, keyvals...)
	}
}

func Warn(msg any, keyvals ...any) {
	if Enabled() {
		Logger().Warn(msg, keyvals...)
	}
}

func Error(msg any, keyvals ...any) {
	if Enabled() {
		Logger().Error(msg, keyvals...)
	}
}

func TUIDebugLogPath() string {
	return strings.TrimSpace(os.Getenv(EnvTUIDebugLogFile))
}

func Reset() {
	if state.closer != nil {
		_ = state.closer.Close()
	}
	state = loggerState{}
	loggerOnce = sync.Once{}
}

func ResetForTests() {
	Reset()
}

func initLogger() {
	output, closer, enabled := logOutput()
	logger := charmlog.NewWithOptions(output, charmlog.Options{
		Level:           parseLevel(strings.TrimSpace(os.Getenv(EnvLogLevel))),
		Formatter:       parseFormatter(strings.TrimSpace(os.Getenv(EnvLogFormat))),
		Prefix:          "stackctl",
		ReportTimestamp: true,
		ReportCaller:    enabled,
		CallerFormatter: charmlog.ShortCallerFormatter,
	})

	state = loggerState{
		logger:  logger,
		enabled: enabled,
		closer:  closer,
	}
}

func logOutput() (io.Writer, io.Closer, bool) {
	target := strings.TrimSpace(os.Getenv(EnvLogFile))
	if target == "" {
		return io.Discard, nil, false
	}
	if target == "-" {
		return os.Stderr, nil, true
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return io.Discard, nil, false
	}

	file, err := os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return io.Discard, nil, false
	}

	return file, file, true
}

func parseLevel(value string) charmlog.Level {
	level, err := charmlog.ParseLevel(value)
	if err != nil {
		return charmlog.InfoLevel
	}
	return level
}

func ValidateLevel(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	_, err := charmlog.ParseLevel(trimmed)
	return err
}

func parseFormatter(value string) charmlog.Formatter {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "text":
		return charmlog.TextFormatter
	case "json":
		return charmlog.JSONFormatter
	case "logfmt":
		return charmlog.LogfmtFormatter
	default:
		return charmlog.TextFormatter
	}
}

func ValidateFormat(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "text", "json", "logfmt":
		return nil
	default:
		return os.ErrInvalid
	}
}
