package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestGetMetrics(t *testing.T) {
	// Get metrics instance
	metrics := GetMetrics()
	
	// Verify it's not nil
	assert.NotNil(t, metrics, "Metrics should not be nil")
	
	// Call again to test singleton behavior
	metrics2 := GetMetrics()
	
	// Verify both instances are the same
	assert.Equal(t, metrics, metrics2, "GetMetrics should return the same instance")
}

func TestAllMetricsInitialized(t *testing.T) {
	// Get metrics instance
	m := GetMetrics()
	
	// API metrics should be initialized
	assert.NotNil(t, m.APIRequestsTotal, "APIRequestsTotal should be initialized")
	assert.NotNil(t, m.APIRequestDuration, "APIRequestDuration should be initialized")
	assert.NotNil(t, m.APIErrorsTotal, "APIErrorsTotal should be initialized")
	assert.NotNil(t, m.APIActiveConnections, "APIActiveConnections should be initialized")
	assert.NotNil(t, m.APIRequestSizeBytes, "APIRequestSizeBytes should be initialized")
	assert.NotNil(t, m.APIResponseSizeBytes, "APIResponseSizeBytes should be initialized")
	
	// Storage metrics should be initialized
	assert.NotNil(t, m.WorkUnitsTotal, "WorkUnitsTotal should be initialized")
	assert.NotNil(t, m.ContextsTotal, "ContextsTotal should be initialized")
	assert.NotNil(t, m.StorageOperations, "StorageOperations should be initialized")
	assert.NotNil(t, m.StorageOperationDuration, "StorageOperationDuration should be initialized")
	assert.NotNil(t, m.WALBatchSize, "WALBatchSize should be initialized")
	assert.NotNil(t, m.WALSyncDuration, "WALSyncDuration should be initialized")
	assert.NotNil(t, m.DBSize, "DBSize should be initialized")
	
	// Lock metrics should be initialized
	assert.NotNil(t, m.LocksAcquiredTotal, "LocksAcquiredTotal should be initialized")
	assert.NotNil(t, m.LocksReleasedTotal, "LocksReleasedTotal should be initialized")
	assert.NotNil(t, m.LocksTimeoutTotal, "LocksTimeoutTotal should be initialized")
	assert.NotNil(t, m.LocksActiveGauge, "LocksActiveGauge should be initialized")
	assert.NotNil(t, m.LockContentionTotal, "LockContentionTotal should be initialized")
	assert.NotNil(t, m.LockAcquisitionDuration, "LockAcquisitionDuration should be initialized")
	
	// Router metrics should be initialized
	assert.NotNil(t, m.RouterEventsTotal, "RouterEventsTotal should be initialized")
	assert.NotNil(t, m.RouterQueueSize, "RouterQueueSize should be initialized")
	assert.NotNil(t, m.RouterEventDuration, "RouterEventDuration should be initialized")
	
	// Notifier metrics should be initialized
	assert.NotNil(t, m.NotifierConnectionsActive, "NotifierConnectionsActive should be initialized")
	assert.NotNil(t, m.NotifierEventsPublished, "NotifierEventsPublished should be initialized")
	assert.NotNil(t, m.NotifierEventDelay, "NotifierEventDelay should be initialized")
	
	// Search metrics should be initialized
	assert.NotNil(t, m.SearchQueriesTotal, "SearchQueriesTotal should be initialized")
	assert.NotNil(t, m.SearchDuration, "SearchDuration should be initialized")
	assert.NotNil(t, m.SearchResultsTotal, "SearchResultsTotal should be initialized")
	assert.NotNil(t, m.SearchIndexSize, "SearchIndexSize should be initialized")
}

func TestMetricsOperations(t *testing.T) {
	// Create a new registry for isolated testing
	registry := prometheus.NewRegistry()
	
	// Helper function to create test metrics
	createTestMetrics := func() *Metrics {
		m := &Metrics{}
		
		// Initialize a few test metrics
		m.APIRequestsTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_api_requests_total",
				Help: "Test metric",
			},
			[]string{"method", "path", "status"},
		)
		registry.MustRegister(m.APIRequestsTotal)
		
		m.WALBatchSize = prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "test_wal_batch_size",
				Help:    "Test metric",
				Buckets: prometheus.LinearBuckets(1, 8, 8),
			},
		)
		registry.MustRegister(m.WALBatchSize)
		
		m.LocksActiveGauge = prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "test_locks_active",
				Help: "Test metric",
			},
		)
		registry.MustRegister(m.LocksActiveGauge)
		
		return m
	}
	
	// Create test metrics
	m := createTestMetrics()
	
	// Test Counter operations
	m.APIRequestsTotal.WithLabelValues("GET", "/api", "200").Inc()
	m.APIRequestsTotal.WithLabelValues("GET", "/api", "200").Add(5)
	
	// Test Gauge operations
	m.LocksActiveGauge.Set(10)
	m.LocksActiveGauge.Inc()
	m.LocksActiveGauge.Dec()
	
	// Test Histogram operations
	m.WALBatchSize.Observe(5)
	m.WALBatchSize.Observe(10)
	
	// We can't easily verify the actual values without accessing internal state,
	// but we can verify that the operations don't panic
}

func BenchmarkMetricsOperations(b *testing.B) {
	// Create a new registry for isolated benchmarking
	registry := prometheus.NewRegistry()
	
	// Initialize test metrics
	counter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "benchmark_counter",
			Help: "Benchmark counter",
		},
	)
	registry.MustRegister(counter)
	
	gauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "benchmark_gauge",
			Help: "Benchmark gauge",
		},
	)
	registry.MustRegister(gauge)
	
	histogram := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "benchmark_histogram",
			Help:    "Benchmark histogram",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
	)
	registry.MustRegister(histogram)
	
	counterVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "benchmark_counter_vec",
			Help: "Benchmark counter vec",
		},
		[]string{"method", "path"},
	)
	registry.MustRegister(counterVec)
	
	// Benchmark Counter
	b.Run("Counter.Inc", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			counter.Inc()
		}
	})
	
	// Benchmark Gauge
	b.Run("Gauge.Set", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			gauge.Set(float64(i))
		}
	})
	
	// Benchmark Histogram
	b.Run("Histogram.Observe", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			histogram.Observe(float64(i) / 1000.0)
		}
	})
	
	// Benchmark CounterVec
	b.Run("CounterVec.WithLabelValues", func(b *testing.B) {
		methods := []string{"GET", "POST", "PUT", "DELETE"}
		paths := []string{"/api/v1/resource", "/api/v1/other", "/api/v2/resource"}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			method := methods[i%len(methods)]
			path := paths[i%len(paths)]
			counterVec.WithLabelValues(method, path).Inc()
		}
	})
}