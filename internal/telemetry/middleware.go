package telemetry

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware returns a middleware function that adds OpenTelemetry tracing
// to HTTP requests. It extracts trace context from incoming requests and
// starts a new span for each request.
func HTTPMiddleware(serviceName string) func(next http.Handler) http.Handler {
	tracer := Tracer(serviceName)
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from the request context
			ctx := r.Context()
			propagator := propagation.TraceContext{}
			ctx = propagator.Extract(ctx, propagation.HeaderCarrier(r.Header))
			
			// Get route pattern if using chi router
			routePattern := getRoutePattern(r)
			
			// Create span attributes
			attrs := []attribute.KeyValue{
				semconv.HTTPMethodKey.String(r.Method),
				semconv.HTTPRouteKey.String(routePattern),
				semconv.HTTPURLKey.String(r.URL.String()),
				semconv.HTTPUserAgentKey.String(r.UserAgent()),
				semconv.HTTPHostKey.String(r.Host),
				semconv.HTTPSchemeKey.String(r.URL.Scheme),
				semconv.NetHostIPKey.String(r.RemoteAddr),
			}
			
			// Start a new span
			spanName := routePattern
			if spanName == "" {
				spanName = r.URL.Path
			}
			
			spanCtx, span := tracer.Start(
				ctx,
				spanName,
				trace.WithAttributes(attrs...),
				trace.WithSpanKind(trace.SpanKindServer),
			)
			defer span.End()
			
			// Add span to request context and call next handler
			ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(ww, r.WithContext(spanCtx))
			
			// Add status code to span
			span.SetAttributes(semconv.HTTPStatusCodeKey.Int(ww.statusCode))
			
			// If error status code, mark span as error
			if ww.statusCode >= 400 {
				span.SetStatus(codes.Error, http.StatusText(ww.statusCode))
			}
		})
	}
}

// getRoutePattern retrieves the route pattern from a chi router if available
func getRoutePattern(r *http.Request) string {
	if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
		return routeContext.RoutePattern()
	}
	return r.URL.Path
}

// responseWriter is a wrapper for http.ResponseWriter that captures the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code and calls the underlying ResponseWriter
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
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