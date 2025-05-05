package logging

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware returns a middleware function that logs HTTP requests
func HTTPMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Start timer
			start := time.Now()
			
			// Create request ID if not present
			requestID := r.Header.Get("X-Request-ID")
			
			// Create basic log event
			event := log.With().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Str("request_id", requestID)
			
			// Add trace context if available
			if span := trace.SpanFromContext(r.Context()); span.SpanContext().IsValid() {
				traceID := span.SpanContext().TraceID().String()
				spanID := span.SpanContext().SpanID().String()
				event = event.Str("trace_id", traceID).Str("span_id", spanID)
			}
			
			// Get route pattern if using chi router
			if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil {
				event = event.Str("route", routeCtx.RoutePattern())
			}
			
			// Create logger
			logger := event.Logger()
			
			// Add logger to context
			ctx := logger.WithContext(r.Context())
			
			// Create response wrapper to capture status code
			ww := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				responseSize:   0,
			}
			
			// Log request
			logger.Debug().Msg("Request started")
			
			// Call next handler with context containing logger
			next.ServeHTTP(ww, r.WithContext(ctx))
			
			// Calculate duration
			duration := time.Since(start)
			
			// Determine log level based on status code
			var logEvent *zerolog.Event
			switch {
			case ww.statusCode >= 500:
				logEvent = logger.Error()
			case ww.statusCode >= 400:
				logEvent = logger.Warn()
			default:
				logEvent = logger.Info()
			}
			
			// Log response details
			logEvent.
				Int("status", ww.statusCode).
				Dur("duration", duration).
				Int64("response_size", ww.responseSize).
				Msg("Request completed")
		})
	}
}

// responseWriter is a wrapper for http.ResponseWriter that captures response details
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	responseSize int64
}

// WriteHeader captures the status code and calls the underlying ResponseWriter
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the response size and calls the underlying ResponseWriter
func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.responseSize += int64(size)
	return size, err
}

// Flush implements http.Flusher if the underlying ResponseWriter supports it
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements http.Hijacker if the underlying ResponseWriter supports it
func (rw *responseWriter) Hijack() (interface{}, interface{}, error) {
	if hj, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}