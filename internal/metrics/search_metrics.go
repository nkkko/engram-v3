package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// SearchMetrics contains Prometheus metrics for the search engine
type SearchMetrics struct {
	// Search metrics
	QueriesTotal       *prometheus.CounterVec
	IndexingTotal      *prometheus.CounterVec
	QueryDuration      *prometheus.HistogramVec
	IndexingDuration   *prometheus.HistogramVec
	ResultsTotal       *prometheus.CounterVec
	ResultsPerQuery    prometheus.Histogram
	IndexSize          prometheus.Gauge
	CacheHitRatio      prometheus.Gauge
	
	// Vector search specific metrics
	VectorQueriesTotal    *prometheus.CounterVec
	VectorIndexingTotal   *prometheus.CounterVec
	VectorQueryDuration   *prometheus.HistogramVec
	VectorIndexingDuration *prometheus.HistogramVec
	SemanticSimilarityScore prometheus.Histogram
}

// NewSearchMetrics initializes and registers search metrics
func NewSearchMetrics() *SearchMetrics {
	m := &SearchMetrics{}
	
	// Regular search metrics
	m.QueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "engram_search_queries_total",
			Help: "Total number of search queries",
		},
		[]string{"backend", "type"}, // backend: badger, weaviate; type: text, vector, relationship
	)
	
	m.IndexingTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "engram_search_indexing_total",
			Help: "Total number of indexed items",
		},
		[]string{"backend", "batch"}, // batch: single, batch
	)
	
	m.QueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "engram_search_query_duration_seconds",
			Help:    "Duration of search queries in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // from 1ms to ~512ms
		},
		[]string{"backend", "type"},
	)
	
	m.IndexingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "engram_search_indexing_duration_seconds",
			Help:    "Duration of indexing operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // from 1ms to ~512ms
		},
		[]string{"backend", "batch"},
	)
	
	m.ResultsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "engram_search_results_total",
			Help: "Total number of search results returned",
		},
		[]string{"backend"},
	)
	
	m.ResultsPerQuery = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "engram_search_results_per_query",
			Help:    "Number of results returned per query",
			Buckets: prometheus.LinearBuckets(0, 10, 11), // 0, 10, 20, ..., 100
		},
	)
	
	m.IndexSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "engram_search_index_size_bytes",
			Help: "Size of the search index in bytes",
		},
	)
	
	m.CacheHitRatio = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "engram_search_cache_hit_ratio",
			Help: "Ratio of cache hits to total queries",
		},
	)
	
	// Vector search specific metrics
	m.VectorQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "engram_vector_search_queries_total",
			Help: "Total number of vector search queries",
		},
		[]string{"backend"},
	)
	
	m.VectorIndexingTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "engram_vector_search_indexing_total",
			Help: "Total number of items indexed for vector search",
		},
		[]string{"backend"},
	)
	
	m.VectorQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "engram_vector_search_query_duration_seconds",
			Help:    "Duration of vector search queries in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // from 1ms to ~512ms
		},
		[]string{"backend"},
	)
	
	m.VectorIndexingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "engram_vector_search_indexing_duration_seconds",
			Help:    "Duration of vector search indexing operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // from 1ms to ~512ms
		},
		[]string{"backend"},
	)
	
	m.SemanticSimilarityScore = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "engram_vector_search_similarity_score",
			Help:    "Distribution of similarity scores in vector search results",
			Buckets: prometheus.LinearBuckets(0.5, 0.05, 11), // 0.5, 0.55, 0.6, ..., 1.0
		},
	)
	
	return m
}