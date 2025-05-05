package telemetry

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const (
	loggerContextKey = contextKey("logger")
	spanContextKey   = contextKey("span")
)

// ContextWithLogger adds a zerolog.Logger to the context
func ContextWithLogger(ctx context.Context, logger zerolog.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, logger)
}

// LoggerFromContext extracts the zerolog.Logger from the context
func LoggerFromContext(ctx context.Context) zerolog.Logger {
	if logger, ok := ctx.Value(loggerContextKey).(zerolog.Logger); ok {
		return logger
	}
	// Return a no-op logger if none is found
	return zerolog.Nop()
}

// StartSpan starts a new span with the given name
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	// Get tracer name from caller if possible
	tracer := Tracer("engram")
	
	// Start span
	ctx, span := tracer.Start(ctx, name, opts...)
	
	// Add span to context
	ctx = context.WithValue(ctx, spanContextKey, span)
	
	// Add trace information to logger if present
	if logger, ok := ctx.Value(loggerContextKey).(zerolog.Logger); ok {
		traceID := span.SpanContext().TraceID().String()
		spanID := span.SpanContext().SpanID().String()
		
		logger = logger.With().
			Str("trace_id", traceID).
			Str("span_id", spanID).
			Logger()
		
		ctx = ContextWithLogger(ctx, logger)
	}
	
	return ctx, span
}

// SpanFromContext extracts the current span from the context
func SpanFromContext(ctx context.Context) trace.Span {
	if span, ok := ctx.Value(spanContextKey).(trace.Span); ok {
		return span
	}
	return trace.SpanFromContext(ctx)
}

// AddSpanAttributes adds attributes to the current span
func AddSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	if span := SpanFromContext(ctx); span != nil {
		span.SetAttributes(attrs...)
	}
}

// AddSpanEvent adds an event to the current span
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	if span := SpanFromContext(ctx); span != nil {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// MarkSpanError marks the current span as having an error
func MarkSpanError(ctx context.Context, err error) {
	if span := SpanFromContext(ctx); span != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}
}

// LogAndTraceError logs an error and records it in the current span
func LogAndTraceError(ctx context.Context, err error, msg string) {
	// Log the error
	logger := LoggerFromContext(ctx)
	logger.Error().Err(err).Msg(msg)
	
	// Record in span
	MarkSpanError(ctx, err)
}

// LogAndTraceInfo logs an informational message and adds it as a span event
func LogAndTraceInfo(ctx context.Context, msg string, fields map[string]interface{}) {
	// Log the message
	logger := LoggerFromContext(ctx)
	event := logger.Info()
	for k, v := range fields {
		event = addField(event, k, v)
	}
	event.Msg(msg)
	
	// Add as span event
	attrs := make([]attribute.KeyValue, 0, len(fields))
	for k, v := range fields {
		attrs = append(attrs, attributeFromValue(k, v))
	}
	AddSpanEvent(ctx, msg, attrs...)
}

// addField adds a field to a zerolog event based on its type
func addField(event *zerolog.Event, key string, value interface{}) *zerolog.Event {
	switch v := value.(type) {
	case string:
		return event.Str(key, v)
	case int:
		return event.Int(key, v)
	case int64:
		return event.Int64(key, v)
	case float64:
		return event.Float64(key, v)
	case bool:
		return event.Bool(key, v)
	default:
		return event.Interface(key, v)
	}
}

// attributeFromValue creates an attribute from a value based on its type
func attributeFromValue(key string, value interface{}) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	case bool:
		return attribute.Bool(key, v)
	default:
		return attribute.String(key, fmt.Sprintf("%v", v))
	}
}