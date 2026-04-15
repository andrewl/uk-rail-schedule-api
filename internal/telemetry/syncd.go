package telemetry

import (
	"context"
	"os"
	"runtime"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"gorm.io/gorm"
)

// syncdMetrics holds all counters used by the syncd daemon.
type syncdMetrics struct {
	vstpProcessed    metric.Int64Counter
	vstpFailed       metric.Int64Counter
	stompReconnects  metric.Int64Counter
	feedRefreshTotal metric.Int64Counter
}

var (
	sm     syncdMetrics
	smOnce sync.Once
)

func getSyncdMetrics() syncdMetrics {
	smOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter(meterName)
		sm.vstpProcessed, _ = meter.Int64Counter(
			"vstp_messages_processed_total",
			metric.WithDescription("Total number of VSTP messages successfully processed"),
		)
		sm.vstpFailed, _ = meter.Int64Counter(
			"vstp_messages_failed_total",
			metric.WithDescription("Total number of VSTP messages that failed to process"),
		)
		sm.stompReconnects, _ = meter.Int64Counter(
			"vstp_stomp_reconnects_total",
			metric.WithDescription("Total number of STOMP reconnection attempts"),
		)
		sm.feedRefreshTotal, _ = meter.Int64Counter(
			"feed_refresh_total",
			metric.WithDescription("Total number of schedule feed refreshes completed"),
		)
	})
	return sm
}

// RecordVSTPProcessed increments the counter for successfully processed VSTP messages.
func RecordVSTPProcessed(ctx context.Context) {
	getSyncdMetrics().vstpProcessed.Add(ctx, 1)
}

// RecordVSTPFailed increments the counter for VSTP messages that failed to process.
func RecordVSTPFailed(ctx context.Context) {
	getSyncdMetrics().vstpFailed.Add(ctx, 1)
}

// RecordStompReconnect increments the counter for STOMP reconnection attempts.
func RecordStompReconnect(ctx context.Context) {
	getSyncdMetrics().stompReconnects.Add(ctx, 1)
}

// RecordFeedRefreshCompleted increments the feed refresh counter and reports
// the number of schedule and tiploc records loaded in that refresh.
func RecordFeedRefreshCompleted(ctx context.Context, scheduleCount, tiplocCount int64) {
	m := getSyncdMetrics()
	m.feedRefreshTotal.Add(ctx, 1)

	// Report per-refresh record counts as a one-shot counter increment so the
	// cumulative total of records ever loaded is visible in Grafana.
	meter := otel.GetMeterProvider().Meter(meterName)
	schedLoaded, _ := meter.Int64Counter(
		"feed_schedules_loaded_total",
		metric.WithDescription("Total number of schedule records loaded from feed files"),
	)
	tipLoaded, _ := meter.Int64Counter(
		"feed_tiplocs_loaded_total",
		metric.WithDescription("Total number of tiploc records loaded from feed files"),
	)
	schedLoaded.Add(ctx, scheduleCount)
	tipLoaded.Add(ctx, tiplocCount)
}

// RegisterSyncdObservables registers observable gauges that are evaluated
// lazily each time the OTel SDK exports metrics (every 30 s by default).
//
// db is the GORM database handle used for live record counts.
// dbPath is the filesystem path to the SQLite file, used to read its size.
func RegisterSyncdObservables(db *gorm.DB, dbPath string) {
	meter := otel.GetMeterProvider().Meter(meterName)

	// --- Database size ---
	dbSizeGauge, _ := meter.Int64ObservableGauge(
		"db_size_bytes",
		metric.WithDescription("Size of the SQLite database file in bytes"),
	)

	// --- Record counts ---
	feedSchedulesGauge, _ := meter.Int64ObservableGauge(
		"db_schedules_feed_total",
		metric.WithDescription("Number of schedule records sourced from the feed file"),
	)
	vstpSchedulesGauge, _ := meter.Int64ObservableGauge(
		"db_schedules_vstp_total",
		metric.WithDescription("Number of schedule records sourced from VSTP"),
	)
	tiplocGauge, _ := meter.Int64ObservableGauge(
		"db_tiplocs_total",
		metric.WithDescription("Number of TIPLOC records in the database"),
	)

	// --- Runtime / memory ---
	heapAllocGauge, _ := meter.Int64ObservableGauge(
		"process_memory_heap_alloc_bytes",
		metric.WithDescription("Bytes of allocated heap objects (live + GC'd)"),
		metric.WithUnit("By"),
	)
	sysGauge, _ := meter.Int64ObservableGauge(
		"process_memory_sys_bytes",
		metric.WithDescription("Total bytes of memory obtained from the OS"),
		metric.WithUnit("By"),
	)
	goroutineGauge, _ := meter.Int64ObservableGauge(
		"process_goroutines_total",
		metric.WithDescription("Number of goroutines currently running"),
	)

	_, _ = meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			// DB file size
			if fi, err := os.Stat(dbPath); err == nil {
				o.ObserveInt64(dbSizeGauge, fi.Size())
			}

			// Record counts via raw SQL (avoids loading all rows)
			var feedCount, vstpCount, tiplocCount int64
			db.Raw("SELECT COUNT(*) FROM schedules WHERE source = 'Feed'").Scan(&feedCount)
			db.Raw("SELECT COUNT(*) FROM schedules WHERE source = 'VSTP'").Scan(&vstpCount)
			db.Raw("SELECT COUNT(*) FROM tiplocs").Scan(&tiplocCount)

			o.ObserveInt64(feedSchedulesGauge, feedCount, metric.WithAttributes(attribute.String("source", "feed")))
			o.ObserveInt64(vstpSchedulesGauge, vstpCount, metric.WithAttributes(attribute.String("source", "vstp")))
			o.ObserveInt64(tiplocGauge, tiplocCount)

			// Runtime memory stats
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			o.ObserveInt64(heapAllocGauge, int64(ms.HeapAlloc))
			o.ObserveInt64(sysGauge, int64(ms.Sys))
			o.ObserveInt64(goroutineGauge, int64(runtime.NumGoroutine()))

			return nil
		},
		dbSizeGauge,
		feedSchedulesGauge,
		vstpSchedulesGauge,
		tiplocGauge,
		heapAllocGauge,
		sysGauge,
		goroutineGauge,
	)
}
