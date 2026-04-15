package telemetry

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "uk-rail-schedule-api"

// Middleware returns a chi-compatible middleware that records HTTP request
// metrics via the global OTel meter:
//
//   - http_requests_total        (counter)   method, route, status_code
//   - http_request_duration_seconds (histogram) method, route, status_code
func Middleware() func(http.Handler) http.Handler {
	meter := otel.GetMeterProvider().Meter(meterName)

	requestCount, _ := meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)

	requestDuration, _ := meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request latency in seconds"),
		metric.WithExplicitBucketBoundaries(
			0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
		),
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(ww, r)

			// Use the matched chi route pattern (e.g. /api/schedules) rather
			// than the raw URL path, to avoid high-cardinality label values.
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = r.URL.Path
			}

			attrs := []attribute.KeyValue{
				attribute.String("method", r.Method),
				attribute.String("route", route),
				attribute.String("status_code", strconv.Itoa(ww.statusCode)),
			}

			requestCount.Add(r.Context(), 1, metric.WithAttributes(attrs...))
			requestDuration.Record(r.Context(), time.Since(start).Seconds(), metric.WithAttributes(attrs...))
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the written status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
