// Package logger provides zerolog-based structured logging with correlation ID support.
package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ctxKey is the type for context keys used by the logger.
type ctxKey string

const (
	// CorrelationIDCxtKey is the context key for the correlation ID.
	CorrelationIDCxtKey ctxKey = "correlation_id"
	// RequestIDCxtKey is the context key for the HTTP request ID.
	RequestIDCxtKey ctxKey = "request_id"
	// UserIDCxtKey is the context key for the user ID.
	UserIDCxtKey ctxKey = "user_id"
	// TraceIDCxtKey is the context key for the OpenTelemetry trace ID.
	TraceIDCxtKey ctxKey = "trace_id"
)

// Logger wraps zerolog.Logger with service-name awareness and context helpers.
type Logger struct {
	zl      zerolog.Logger
	service string
	output  io.Writer
	mu      sync.RWMutex
}

// Option configures a Logger.
type Option func(*Logger)

// WithServiceName sets the service name field on every log entry.
func WithServiceName(name string) Option {
	return func(l *Logger) {
		l.service = name
		l.mu.RLock()
		out := l.output
		l.mu.RUnlock()
		l.zl = zerolog.New(out).
			Level(zerolog.InfoLevel).
			With().
			Timestamp().
			Caller().
			Str("service", name).
			Logger()
	}
}

// WithOutput sets the output writer for the logger.
func WithOutput(w io.Writer) Option {
	return func(l *Logger) {
		l.mu.Lock()
		l.output = w
		l.mu.Unlock()
		l.zl = l.zl.Output(w)
	}
}

// WithLevel sets the minimum log level.
func WithLevel(level string) Option {
	return func(l *Logger) {
		lvl, err := zerolog.ParseLevel(strings.ToLower(level))
		if err != nil {
			lvl = zerolog.InfoLevel
		}
		zerolog.SetGlobalLevel(lvl)
	}
}

// WithTimeFormat sets the time format for log timestamps.
func WithTimeFormat(format string) Option {
	return func(l *Logger) {
		switch format {
		case "unix":
			zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
		case "unixms":
			zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
		default:
			zerolog.TimeFieldFormat = time.RFC3339
		}
	}
}

// WithConsoleFormat writes human-readable logs for local development.
// Preserves any custom output writer set via WithOutput.
func WithConsoleFormat() Option {
	return func(l *Logger) {
		var w io.Writer = os.Stdout
		l.mu.Lock()
		if l.output != nil {
			w = l.output
		}
		l.output = w
		l.mu.Unlock()
		l.zl = l.zl.Output(zerolog.ConsoleWriter{
			Out:        w,
			TimeFormat: "15:04:05",
			FormatLevel: func(i interface{}) string {
				return strings.ToUpper(fmt.Sprintf("%-5s", i))
			},
		})
	}
}

// WithJSONFormat keeps structured JSON output. JSON is the default logger mode.
func WithJSONFormat() Option {
	return func(l *Logger) {}
}

// New creates a new Logger with the given options.
// Default: JSON output, info level, RFC3339 timestamps.
func New(opts ...Option) *Logger {
	l := &Logger{
		service: "erg-service",
		output:  os.Stdout,
	}
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		short := file
		for i := len(file) - 1; i > 0; i-- {
			if file[i] == '/' {
				short = file[i+1:]
				break
			}
		}
		return short + ":" + strconv.Itoa(line)
	}

	l.zl = zerolog.New(os.Stdout).
		Level(zerolog.InfoLevel).
		With().
		Timestamp().
		Caller().
		Str("service", l.service).
		Logger()

	for _, o := range opts {
		o(l)
	}

	return l
}

// NoOp returns a logger that discards all output (useful for nil dependencies).
func NoOp() *Logger {
	l := zerolog.Nop()
	return &Logger{zl: l, service: "nop"}
}

// WithContext returns a logger with context fields injected from ctx.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	if ctx == nil {
		return l
	}

	fields := make([]func(zc *zerolog.Context), 0, 4)

	if cid, ok := ctx.Value(CorrelationIDCxtKey).(string); ok && cid != "" {
		fields = append(fields, func(zc *zerolog.Context) {
			zc.Str("correlation_id", cid)
		})
	}
	if rid, ok := ctx.Value(RequestIDCxtKey).(string); ok && rid != "" {
		fields = append(fields, func(zc *zerolog.Context) {
			zc.Str("request_id", rid)
		})
	}
	if uid, ok := ctx.Value(UserIDCxtKey).(string); ok && uid != "" {
		fields = append(fields, func(zc *zerolog.Context) {
			zc.Str("user_id", uid)
		})
	}
	if tid, ok := ctx.Value(TraceIDCxtKey).(string); ok && tid != "" {
		fields = append(fields, func(zc *zerolog.Context) {
			zc.Str("trace_id", tid)
		})
	}

	if len(fields) == 0 {
		return l
	}

	zl := l.zl.With()
	for _, f := range fields {
		f(&zl)
	}
	return &Logger{zl: zl.Logger(), service: l.service}
}

// WithCorrelationID sets the correlation ID field on this logger.
func (l *Logger) WithCorrelationID(id string) *Logger {
	zl := l.zl.With().Str("correlation_id", id).Logger()
	return &Logger{zl: zl, service: l.service}
}

// WithRequestID sets the request ID field on this logger.
func (l *Logger) WithRequestID(id string) *Logger {
	zl := l.zl.With().Str("request_id", id).Logger()
	return &Logger{zl: zl, service: l.service}
}

// WithUserID sets the user ID field on this logger.
func (l *Logger) WithUserID(id string) *Logger {
	zl := l.zl.With().Str("user_id", id).Logger()
	return &Logger{zl: zl, service: l.service}
}

// WithField adds a single field to the logger.
func (l *Logger) WithField(key string, value interface{}) *Logger {
	zl := l.zl.With().Interface(key, RedactValue(key, value)).Logger()
	return &Logger{zl: zl, service: l.service}
}

// WithFields adds multiple fields to the logger.
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	zc := l.zl.With()
	for k, v := range fields {
		zc = zc.Interface(k, RedactValue(k, v))
	}
	return &Logger{zl: zc.Logger(), service: l.service}
}

// RedactValue masks sensitive field values before they enter structured logs.
func RedactValue(key string, value interface{}) interface{} {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "_", "-"))
	for _, marker := range []string{"authorization", "cookie", "password", "secret", "token", "api-key", "apikey", "access-key"} {
		if strings.Contains(normalized, marker) {
			return "[REDACTED]"
		}
	}
	return value
}

// Debug logs a debug message.
func (l *Logger) Debug() *zerolog.Event {
	return l.zl.Debug()
}

// Info logs an info message.
func (l *Logger) Info() *zerolog.Event {
	return l.zl.Info()
}

// Warn logs a warning message.
func (l *Logger) Warn() *zerolog.Event {
	return l.zl.Warn()
}

// Error logs an error message.
func (l *Logger) Error() *zerolog.Event {
	return l.zl.Error()
}

// Fatal logs a fatal message and exits.
func (l *Logger) Fatal() *zerolog.Event {
	return l.zl.Fatal()
}

// DebugContext logs a debug message with context fields.
func (l *Logger) DebugContext(ctx context.Context) *zerolog.Event {
	return l.WithContext(ctx).Debug()
}

// InfoContext logs an info message with context fields.
func (l *Logger) InfoContext(ctx context.Context) *zerolog.Event {
	return l.WithContext(ctx).Info()
}

// WarnContext logs a warning message with context fields.
func (l *Logger) WarnContext(ctx context.Context) *zerolog.Event {
	return l.WithContext(ctx).Warn()
}

// ErrorContext logs an error message with context fields.
func (l *Logger) ErrorContext(ctx context.Context) *zerolog.Event {
	return l.WithContext(ctx).Error()
}

// FatalContext logs a fatal message and exits.
func (l *Logger) FatalContext(ctx context.Context) *zerolog.Event {
	return l.WithContext(ctx).Fatal()
}

// FromContext extracts a Logger from a context. Falls back to a no-op logger.
func FromContext(ctx context.Context) *Logger {
	if ctx == nil {
		return NoOp()
	}
	if l, ok := ctx.Value(ctxKey("logger")).(*Logger); ok && l != nil {
		return l
	}
	return NoOp()
}

// ToContext stores a Logger in a context.
func ToContext(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, ctxKey("logger"), l)
}

// NewContext is a shorthand for ToContext.
func NewContext(parent context.Context, l *Logger) context.Context {
	return ToContext(parent, l)
}

// Zerolog returns the underlying zerolog.Logger for use in adapters/wrappers.
func (l *Logger) Zerolog() *zerolog.Logger {
	return &l.zl
}

// Sync flushes any buffered log entries (primarily for file-based outputs).
func (l *Logger) Sync() error {
	if w, ok := l.output.(interface{ Sync() error }); ok {
		return w.Sync()
	}
	return nil
}
