package telemetry

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	errorCounter     metric.Int64Counter
	errorCounterOnce sync.Once
)

func getErrorCounter() metric.Int64Counter {
	errorCounterOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter(meterName)
		errorCounter, _ = meter.Int64Counter(
			"application_errors_total",
			metric.WithDescription("Total number of application errors"),
		)
	})
	return errorCounter
}

// RecordError increments the application_errors_total counter.
// errorType should be a short, low-cardinality label such as "db", "handler",
// "sync", or "parse".
func RecordError(ctx context.Context, errorType string) {
	getErrorCounter().Add(ctx, 1, metric.WithAttributes(
		attribute.String("error_type", errorType),
	))
}
