package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// singleton instance
	instance *Metrics
	once     sync.Once
)

// Metrics holds Prometheus metrics for Engram
type Metrics struct {
	// API metrics
	APIRequestsTotal      *prometheus.CounterVec
	APIRequestDuration    *prometheus.HistogramVec
	APIErrorsTotal        *prometheus.CounterVec
	APIActiveConnections  prometheus.Gauge
	APIRequestSizeBytes   *prometheus.HistogramVec
	APIResponseSizeBytes  *prometheus.HistogramVec
	
	// Storage metrics
	WorkUnitsTotal        prometheus.Counter
	ContextsTotal         prometheus.Counter
	StorageOperations     *prometheus.CounterVec
	StorageOperationDuration *prometheus.HistogramVec
	WALBatchSize          prometheus.Histogram
	WALSyncDuration       prometheus.Histogram
	DBSize                prometheus.Gauge
	
	// Lock metrics
	LocksAcquiredTotal    prometheus.Counter
	LocksReleasedTotal    prometheus.Counter
	LocksTimeoutTotal     prometheus.Counter
	LocksActiveGauge      prometheus.Gauge
	LockContentionTotal   prometheus.Counter
	LockAcquisitionDuration prometheus.Histogram
	
	// Router metrics
	RouterEventsTotal     *prometheus.CounterVec
	RouterQueueSize       prometheus.Gauge
	RouterEventDuration   prometheus.Histogram
	
	// Notifier metrics
	NotifierConnectionsActive prometheus.Gauge
	NotifierEventsPublished   *prometheus.CounterVec
	NotifierEventDelay        prometheus.Histogram
	
	// Search metrics
	SearchQueriesTotal    prometheus.Counter
	SearchDuration        prometheus.Histogram
	SearchResultsTotal    prometheus.Counter
	SearchIndexSize       prometheus.Gauge
}

// GetMetrics returns the metrics singleton
func GetMetrics() *Metrics {
	once.Do(func() {
		instance = newMetrics()
	})
	return instance
}

// newMetrics initializes and registers all metrics
func newMetrics() *Metrics {
	m := &Metrics{}
	
	// API metrics
	m.APIRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "engram_api_requests_total",
			Help: "Total number of API requests",
		},
		[]string{"method", "path", "status"},
	)
	
	m.APIRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "engram_api_request_duration_seconds",
			Help:    "API request duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // from 1ms to ~16s
		},
		[]string{"method", "path"},
	)
	
	m.APIErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "engram_api_errors_total",
			Help: "Total number of API errors",
		},
		[]string{"method", "path", "error_type"},
	)
	
	m.APIActiveConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "engram_api_active_connections",
			Help: "Number of active API connections",
		},
	)
	
	m.APIRequestSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "engram_api_request_size_bytes",
			Help:    "Size of API requests in bytes",
			Buckets: prometheus.ExponentialBuckets(128, 2, 10), // from 128B to ~64KB
		},
		[]string{"method", "path"},
	)
	
	m.APIResponseSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "engram_api_response_size_bytes",
			Help:    "Size of API responses in bytes",
			Buckets: prometheus.ExponentialBuckets(128, 2, 10), // from 128B to ~64KB
		},
		[]string{"method", "path"},
	)
	
	// Storage metrics
	m.WorkUnitsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "engram_work_units_total",
			Help: "Total number of work units created",
		},
	)
	
	m.ContextsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "engram_contexts_total",
			Help: "Total number of contexts created",
		},
	)
	
	m.StorageOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "engram_storage_operations_total",
			Help: "Total number of storage operations",
		},
		[]string{"operation", "success"},
	)
	
	m.StorageOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "engram_storage_operation_duration_seconds",
			Help:    "Duration of storage operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 15), // from 0.1ms to ~1.6s
		},
		[]string{"operation"},
	)
	
	m.WALBatchSize = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "engram_wal_batch_size",
			Help:    "Size of WAL batches",
			Buckets: prometheus.LinearBuckets(1, 8, 8), // from 1 to 57 in steps of 8
		},
	)
	
	m.WALSyncDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "engram_wal_sync_duration_seconds",
			Help:    "Duration of WAL sync operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 10), // from 0.1ms to ~51ms
		},
	)
	
	m.DBSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "engram_db_size_bytes",
			Help: "Size of the database in bytes",
		},
	)
	
	// Lock metrics
	m.LocksAcquiredTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "engram_locks_acquired_total",
			Help: "Total number of locks acquired",
		},
	)
	
	m.LocksReleasedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "engram_locks_released_total",
			Help: "Total number of locks released",
		},
	)
	
	m.LocksTimeoutTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "engram_locks_timeout_total",
			Help: "Total number of lock acquisitions that timed out",
		},
	)
	
	m.LocksActiveGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "engram_locks_active",
			Help: "Number of currently active locks",
		},
	)
	
	m.LockContentionTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "engram_lock_contention_total",
			Help: "Total number of lock contentions",
		},
	)
	
	m.LockAcquisitionDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "engram_lock_acquisition_duration_seconds",
			Help:    "Duration of lock acquisition attempts in seconds",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 10), // from 0.1ms to ~51ms
		},
	)
	
	// Router metrics
	m.RouterEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "engram_router_events_total",
			Help: "Total number of events processed by the router",
		},
		[]string{"event_type"},
	)
	
	m.RouterQueueSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "engram_router_queue_size",
			Help: "Current size of the router event queue",
		},
	)
	
	m.RouterEventDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "engram_router_event_duration_seconds",
			Help:    "Duration of event processing in the router in seconds",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 10), // from 0.1ms to ~51ms
		},
	)
	
	// Notifier metrics
	m.NotifierConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "engram_notifier_connections_active",
			Help: "Number of active notifier connections",
		},
	)
	
	m.NotifierEventsPublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "engram_notifier_events_published_total",
			Help: "Total number of events published by the notifier",
		},
		[]string{"protocol"}, // websocket, sse
	)
	
	m.NotifierEventDelay = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "engram_notifier_event_delay_seconds",
			Help:    "Delay between event creation and notification in seconds",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 10), // from 0.1ms to ~51ms
		},
	)
	
	// Search metrics
	m.SearchQueriesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "engram_search_queries_total",
			Help: "Total number of search queries",
		},
	)
	
	m.SearchDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "engram_search_duration_seconds",
			Help:    "Duration of search queries in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // from 1ms to ~512ms
		},
	)
	
	m.SearchResultsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "engram_search_results_total",
			Help: "Total number of search results returned",
		},
	)
	
	m.SearchIndexSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "engram_search_index_size_bytes",
			Help: "Size of the search index in bytes",
		},
	)
	
	return m
}