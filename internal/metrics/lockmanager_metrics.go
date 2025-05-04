package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// LockManagerMetrics contains metrics for the lock manager
type LockManagerMetrics struct {
	// LockAcquisitions counts lock acquisitions
	LockAcquisitions *prometheus.CounterVec

	// LockReleases counts lock releases
	LockReleases *prometheus.CounterVec

	// LockFailures counts lock acquisition failures
	LockFailures *prometheus.CounterVec

	// LockContention counts contention for locks
	LockContention *prometheus.CounterVec

	// ActiveLocks tracks the number of active locks
	ActiveLocks prometheus.Gauge

	// LockAcquisitionDuration tracks the time taken to acquire locks
	LockAcquisitionDuration *prometheus.HistogramVec

	// LockHoldDuration tracks how long locks are held
	LockHoldDuration *prometheus.HistogramVec

	// LocksExpired counts the number of expired locks
	LocksExpired prometheus.Counter

	// LocksCleanedUp counts the number of locks cleaned up
	LocksCleanedUp prometheus.Counter
}

// NewLockManagerMetrics creates a new set of lock manager metrics
func NewLockManagerMetrics() *LockManagerMetrics {
	return &LockManagerMetrics{
		LockAcquisitions: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "engram_lock_acquisitions_total",
				Help: "Total number of lock acquisitions",
			},
			[]string{"resource_type", "success"},
		),
		LockReleases: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "engram_lock_releases_total",
				Help: "Total number of lock releases",
			},
			[]string{"resource_type", "success"},
		),
		LockFailures: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "engram_lock_failures_total",
				Help: "Total number of lock acquisition failures",
			},
			[]string{"reason"},
		),
		LockContention: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "engram_lock_contention_total",
				Help: "Total number of lock contentions",
			},
			[]string{"resource_type"},
		),
		ActiveLocks: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "engram_active_locks",
				Help: "Number of currently active locks",
			},
		),
		LockAcquisitionDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "engram_lock_acquisition_duration_seconds",
				Help:    "Duration of lock acquisitions",
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to ~1s
			},
			[]string{"resource_type"},
		),
		LockHoldDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "engram_lock_hold_duration_seconds",
				Help:    "Duration locks are held",
				Buckets: prometheus.ExponentialBuckets(0.1, 2, 12), // 100ms to ~7 minutes
			},
			[]string{"resource_type"},
		),
		LocksExpired: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "engram_locks_expired_total",
				Help: "Total number of locks that expired",
			},
		),
		LocksCleanedUp: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "engram_locks_cleaned_up_total",
				Help: "Total number of locks that were cleaned up",
			},
		),
	}
}

// GetLockManagerMetrics returns the singleton instance of lock manager metrics
func GetLockManagerMetrics() *LockManagerMetrics {
	metricsInit.Do(func() {
		if lockManagerMetrics == nil {
			lockManagerMetrics = NewLockManagerMetrics()
		}
	})
	return lockManagerMetrics
}

// singleton instance
var lockManagerMetrics *LockManagerMetrics