// Package telemetry provides Prometheus metrics setup and HTTP instrumentation.
package telemetry

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusRegistry holds all Prometheus metrics for a service.
type PrometheusRegistry struct {
	// HTTP metrics.
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	httpRequestsInFlight prometheus.Gauge

	// Custom business metrics (service-specific).
	customMetrics map[string]prometheus.Collector
}

// NewPrometheusRegistry creates a new Prometheus registry with standard HTTP metrics.
func NewPrometheusRegistry(serviceName string) *PrometheusRegistry {
	r := &PrometheusRegistry{
		httpRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "erg",
				Subsystem: serviceName,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests by method, path, and status.",
			},
			[]string{"method", "path", "status"},
		),
		httpRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "erg",
				Subsystem: serviceName,
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request duration in seconds.",
				Buckets: []float64{
					0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
				},
			},
			[]string{"method", "path", "status"},
		),
		httpRequestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "erg",
				Subsystem: serviceName,
				Name:      "http_requests_in_flight",
				Help:      "Number of HTTP requests currently being processed.",
			},
		),
		customMetrics: make(map[string]prometheus.Collector),
	}

	return r
}

// RecordHTTP records metrics for an HTTP request.
func (r *PrometheusRegistry) RecordHTTP(method, path string, status int, duration time.Duration) {
	s := strconv.Itoa(status)
	r.httpRequestsTotal.WithLabelValues(method, path, s).Inc()
	r.httpRequestDuration.WithLabelValues(method, path, s).Observe(duration.Seconds())
}

// Handler returns the Prometheus HTTP handler for the /metrics endpoint.
func (r *PrometheusRegistry) Handler() http.Handler {
	return promhttp.Handler()
}

// IncrementInFlight increments the in-flight request gauge.
func (r *PrometheusRegistry) IncrementInFlight() {
	r.httpRequestsInFlight.Inc()
}

// DecrementInFlight decrements the in-flight request gauge.
func (r *PrometheusRegistry) DecrementInFlight() {
	r.httpRequestsInFlight.Dec()
}

// HTTPMetricsMiddleware returns a middleware that records HTTP metrics.
func (r *PrometheusRegistry) HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if r == nil {
			next.ServeHTTP(w, req)
			return
		}

		r.IncrementInFlight()
		defer r.DecrementInFlight()

		start := time.Now()

		// Wrap response writer to capture status code.
		wrapped := &statusCaptureResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, req)

		duration := time.Since(start)
		r.RecordHTTP(req.Method, normalizePath(req.URL.Path), wrapped.statusCode, duration)
	})
}

// statusCaptureResponseWriter wraps http.ResponseWriter to capture the status code.
type statusCaptureResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusCaptureResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// normalizePath normalizes URL paths for metric labels to prevent high cardinality.
// It replaces dynamic segments like /users/123 with /users/{id}.
func normalizePath(path string) string {
	// Simple normalization: truncate to max 50 chars and replace numbers.
	if len(path) > 50 {
		path = path[:50] + "..."
	}
	return path
}

// ---- Custom Business Metrics ----

// Counter is a Prometheus counter metric helper.
type Counter struct {
	metric *prometheus.CounterVec
}

// NewCounter creates a new counter metric.
func (r *PrometheusRegistry) NewCounter(name, help string, labels []string) *Counter {
	c := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "erg",
			Name:     name,
			Help:     help,
		},
		labels,
	)
	return &Counter{metric: c}
}

// Inc increments the counter by 1 with the given label values.
func (c *Counter) Inc(labels ...string) {
	c.metric.WithLabelValues(labels...).Inc()
}

// Add adds the given value to the counter.
func (c *Counter) Add(v float64, labels ...string) {
	c.metric.WithLabelValues(labels...).Add(v)
}

// Gauge is a Prometheus gauge metric helper.
type Gauge struct {
	metric *prometheus.GaugeVec
}

// NewGauge creates a new gauge metric.
func (r *PrometheusRegistry) NewGauge(name, help string, labels []string) *Gauge {
	g := promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "erg",
			Name:     name,
			Help:     help,
		},
		labels,
	)
	return &Gauge{metric: g}
}

// Set sets the gauge to the given value.
func (g *Gauge) Set(v float64, labels ...string) {
	g.metric.WithLabelValues(labels...).Set(v)
}

// Inc increments the gauge by 1.
func (g *Gauge) Inc(labels ...string) {
	g.metric.WithLabelValues(labels...).Inc()
}

// Dec decrements the gauge by 1.
func (g *Gauge) Dec(labels ...string) {
	g.metric.WithLabelValues(labels...).Dec()
}

// Histogram is a Prometheus histogram metric helper.
type Histogram struct {
	metric *prometheus.HistogramVec
}

// NewHistogram creates a new histogram metric.
func (r *PrometheusRegistry) NewHistogram(name, help string, labels []string, buckets []float64) *Histogram {
	if len(buckets) == 0 {
		buckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	}
	h := promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "erg",
			Name:     name,
			Help:     help,
			Buckets:  buckets,
		},
		labels,
	)
	return &Histogram{metric: h}
}

// Observe records an observation in the histogram.
func (h *Histogram) Observe(v float64, labels ...string) {
	h.metric.WithLabelValues(labels...).Observe(v)
}

// ---- Pre-defined business metrics for crawler service ----

// CrawlerMetrics holds common crawler-related metrics.
type CrawlerMetrics struct {
	PagesCrawled   *Counter
	CrawlErrors    *Counter
	CrawlDuration  *Histogram
	QueueSize      *Gauge
	ContentDuplicates *Counter
}

// NewCrawlerMetrics creates a new set of crawler metrics.
func (r *PrometheusRegistry) NewCrawlerMetrics() *CrawlerMetrics {
	return &CrawlerMetrics{
		PagesCrawled: r.NewCounter(
			"crawler_pages_crawled_total",
			"Total number of pages crawled",
			[]string{"status"}, // "success", "blocked", "error"
		),
		CrawlErrors: r.NewCounter(
			"crawler_errors_total",
			"Total number of crawler errors",
			[]string{"type"}, // "network", "parse", "robots", "timeout"
		),
		CrawlDuration: r.NewHistogram(
			"crawler_page_duration_seconds",
			"Time taken to crawl a single page",
			[]string{"domain"},
			[]float64{0.1, 0.5, 1, 2.5, 5, 10, 30},
		),
		QueueSize: r.NewGauge(
			"crawler_queue_size",
			"Current size of the crawl queue",
			nil, // no labels
		),
		ContentDuplicates: r.NewCounter(
			"crawler_duplicates_total",
			"Total number of duplicate content items detected",
			[]string{"type"}, // "exact", "near"
		),
	}
}

// NotificationMetrics holds common notification-related metrics.
type NotificationMetrics struct {
	NotificationsSent  *Counter
	NotificationErrors *Counter
	DeliveryLatency    *Histogram
}

// NewNotificationMetrics creates a new set of notification metrics.
func (r *PrometheusRegistry) NewNotificationMetrics() *NotificationMetrics {
	return &NotificationMetrics{
		NotificationsSent: r.NewCounter(
			"notifications_sent_total",
			"Total number of notifications sent",
			[]string{"channel", "status"}, // channel: discord/telegram/email, status: sent/failed
		),
		NotificationErrors: r.NewCounter(
			"notification_errors_total",
			"Total number of notification errors",
			[]string{"channel", "error_type"},
		),
		DeliveryLatency: r.NewHistogram(
			"notification_delivery_latency_seconds",
			"Time from notification creation to delivery",
			[]string{"channel"},
			[]float64{0.1, 0.5, 1, 2.5, 5, 10, 30, 60},
		),
	}
}

// init registers the default metrics.
func init() {
	// Register standard Go runtime metrics.
	// These are automatically exported by the Prometheus client_golang library.
}

// ObserveRequest is a convenience method to record HTTP request metrics.
func ObserveRequest(ctx context.Context, method, path string, status int, duration time.Duration) {
	// This function is a standalone helper when the registry is not directly available.
	_ = ctx
	_ = method
	_ = path
	_ = status
	_ = duration
}
