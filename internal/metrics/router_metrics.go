package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// RouterMetrics contains Prometheus metrics for the router
type RouterMetrics struct {
	// General metrics
	EventsTotal        *prometheus.CounterVec
	EventDuration      *prometheus.HistogramVec
	QueueSize          prometheus.Gauge
	DroppedEventsTotal *prometheus.CounterVec
	
	// Subscription metrics
	SubscriptionsActive prometheus.Gauge
	EventsDelivered     *prometheus.CounterVec
	SubscriberLag       *prometheus.GaugeVec
	ProcessingErrors    *prometheus.CounterVec
	
	// Performance metrics
	EventsBatchSize     prometheus.Histogram
	ProcessingBatchTime prometheus.Histogram
	BackpressureEvents  prometheus.Counter
}

// NewRouterMetrics initializes and registers router metrics
func NewRouterMetrics() *RouterMetrics {
	m := &RouterMetrics{}
	
	// General metrics
	m.EventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "engram_router_events_total",
			Help: "Total number of events processed by the router",
		},
		[]string{"event_type"}, // work_unit, context, lock, etc.
	)
	
	m.EventDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "engram_router_event_duration_seconds",
			Help:    "Duration of event processing in the router in seconds",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 10), // from 0.1ms to ~51ms
		},
		[]string{"event_type"},
	)
	
	m.QueueSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "engram_router_queue_size",
			Help: "Current size of the router event queue",
		},
	)
	
	m.DroppedEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "engram_router_dropped_events_total",
			Help: "Total number of events dropped due to queue overflow",
		},
		[]string{"event_type", "reason"}, // reason: queue_full, subscriber_slow, etc.
	)
	
	// Subscription metrics
	m.SubscriptionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "engram_router_subscriptions_active",
			Help: "Number of active subscriptions to the router",
		},
	)
	
	m.EventsDelivered = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "engram_router_events_delivered_total",
			Help: "Total number of events delivered to subscribers",
		},
		[]string{"subscription_type"}, // all, filtered, etc.
	)
	
	m.SubscriberLag = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "engram_router_subscriber_lag",
			Help: "Number of events waiting to be processed by subscribers",
		},
		[]string{"subscription_id"},
	)
	
	m.ProcessingErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "engram_router_processing_errors_total",
			Help: "Total number of errors encountered while processing events",
		},
		[]string{"event_type", "error_type"},
	)
	
	// Performance metrics
	m.EventsBatchSize = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "engram_router_events_batch_size",
			Help:    "Size of event batches processed by the router",
			Buckets: prometheus.LinearBuckets(1, 10, 10), // 1, 11, 21, ..., 91
		},
	)
	
	m.ProcessingBatchTime = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "engram_router_processing_batch_seconds",
			Help:    "Time taken to process a batch of events in seconds",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 10), // from 0.1ms to ~51ms
		},
	)
	
	m.BackpressureEvents = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "engram_router_backpressure_events_total",
			Help: "Total number of times backpressure was applied due to slow subscribers",
		},
	)
	
	return m
}