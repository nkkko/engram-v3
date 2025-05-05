package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"go.opentelemetry.io/otel/trace"
)

// LogFormat represents the output format for logs
type LogFormat string

const (
	// FormatJSON outputs logs in JSON format
	FormatJSON LogFormat = "json"
	
	// FormatConsole outputs logs in a human-readable format
	FormatConsole LogFormat = "console"
)

// LogLevel represents the logging level
type LogLevel string

const (
	// LevelDebug shows all logs
	LevelDebug LogLevel = "debug"
	
	// LevelInfo shows info and above
	LevelInfo LogLevel = "info"
	
	// LevelWarn shows warnings and above
	LevelWarn LogLevel = "warn"
	
	// LevelError shows errors and above
	LevelError LogLevel = "error"
)

// Config contains logger configuration
type Config struct {
	// Logging level
	Level LogLevel
	
	// Output format (json or console)
	Format LogFormat
	
	// Whether to include caller information
	IncludeCaller bool
	
	// Whether to include stack traces for errors
	IncludeStacktrace bool
	
	// Whether to include OpenTelemetry trace context
	IncludeTraceContext bool
	
	// Output writer (defaults to os.Stdout)
	Output io.Writer
	
	// Additional global context fields
	GlobalFields map[string]string
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		Level:              LevelInfo,
		Format:             FormatJSON,
		IncludeCaller:      true,
		IncludeStacktrace:  true,
		IncludeTraceContext: true,
		Output:             os.Stdout,
		GlobalFields:       map[string]string{},
	}
}

// Setup configures global logging
func Setup(config Config) error {
	// Configure time formatting
	zerolog.TimeFieldFormat = time.RFC3339Nano
	
	// Configure output format
	var output io.Writer
	if config.Output == nil {
		output = os.Stdout
	} else {
		output = config.Output
	}
	
	// Use console writer if console format is requested
	if config.Format == FormatConsole {
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: time.RFC3339,
		}
	}
	
	// Enable error stack marshaling
	if config.IncludeStacktrace {
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	}
	
	// Set global logger
	logger := zerolog.New(output).With().Timestamp()
	
	// Include caller if requested
	if config.IncludeCaller {
		logger = logger.Caller()
	}
	
	// Add any global fields
	for k, v := range config.GlobalFields {
		logger = logger.Str(k, v)
	}
	
	// Set global logger
	log.Logger = logger.Logger()
	
	// Set log level
	level, err := parseLevel(config.Level)
	if err != nil {
		return err
	}
	zerolog.SetGlobalLevel(level)
	
	return nil
}

// parseLevel converts a LogLevel to zerolog.Level
func parseLevel(level LogLevel) (zerolog.Level, error) {
	switch level {
	case LevelDebug:
		return zerolog.DebugLevel, nil
	case LevelInfo:
		return zerolog.InfoLevel, nil
	case LevelWarn:
		return zerolog.WarnLevel, nil
	case LevelError:
		return zerolog.ErrorLevel, nil
	default:
		return zerolog.InfoLevel, fmt.Errorf("invalid log level: %s", level)
	}
}

// FromContext returns a logger with trace context if available
func FromContext(ctx context.Context) zerolog.Logger {
	logger := log.Ctx(ctx).With()
	
	// Add trace context if available
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		traceID := span.SpanContext().TraceID().String()
		spanID := span.SpanContext().SpanID().String()
		logger = logger.Str("trace_id", traceID).Str("span_id", spanID)
	}
	
	return logger.Logger()
}

// WithContext returns a context with the given logger attached
func WithContext(ctx context.Context, logger zerolog.Logger) context.Context {
	return log.Logger.WithContext(ctx)
}

// Component returns a logger with a component field
func Component(name string) zerolog.Logger {
	return log.With().Str("component", name).Logger()
}