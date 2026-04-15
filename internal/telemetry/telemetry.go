// Package telemetry sets up an OpenTelemetry metrics provider that pushes to
// Grafana Cloud (or any OTLP-compatible backend) via the OTLP HTTP exporter.
//
// Configuration is entirely via standard OTel environment variables:
//
//	OTEL_EXPORTER_OTLP_ENDPOINT   — e.g. https://otlp-gateway-prod-eu-west-0.grafana.net/otlp
//	OTEL_EXPORTER_OTLP_HEADERS    — e.g. Authorization=Basic <base64(instanceId:token)>
//	OTEL_SERVICE_NAME             — e.g. uk-rail-schedule-api
//
// If OTEL_EXPORTER_OTLP_ENDPOINT is not set the provider is a no-op, so
// telemetry is safely skipped in local development.
package telemetry

import (
	"context"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// Setup initialises the global OTel metrics provider. The returned function
// must be called (typically via defer) to flush and shut down the exporter
// before the process exits.
//
// If OTEL_EXPORTER_OTLP_ENDPOINT is empty, Setup is a no-op and returns a
// no-op shutdown function.
func Setup(ctx context.Context, serviceName, serviceVersion string) (shutdown func(context.Context) error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		slog.Info("OTEL_EXPORTER_OTLP_ENDPOINT not set — telemetry disabled")
		return func(_ context.Context) error { return nil }
	}

	exporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		slog.Error("Failed to create OTLP metric exporter", "error", err)
		return func(_ context.Context) error { return nil }
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		slog.Warn("Failed to merge OTel resource", "error", err)
		res = resource.Default()
	}

	provider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(
			metric.NewPeriodicReader(exporter, metric.WithInterval(30*time.Second)),
		),
	)

	otel.SetMeterProvider(provider)
	slog.Info("OpenTelemetry metrics enabled", "endpoint", endpoint)

	return provider.Shutdown
}
