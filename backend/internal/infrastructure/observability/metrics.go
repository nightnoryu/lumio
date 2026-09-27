package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry        *prometheus.Registry
	requests        *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	operations      *prometheus.CounterVec
	duration        *prometheus.HistogramVec
}

func New() *Metrics {
	m := &Metrics{
		registry:        prometheus.NewRegistry(),
		requests:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "lumio_http_requests_total", Help: "Requests by bounded route name and HTTP status."}, []string{"route", "status"}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "lumio_http_request_duration_seconds", Help: "Request duration by bounded route name."}, []string{"route"}),
		operations:      prometheus.NewCounterVec(prometheus.CounterOpts{Name: "lumio_media_operations_total", Help: "Upload creation, verification, processing and cleanup attempts by outcome."}, []string{"operation", "outcome"}),
		duration:        prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "lumio_media_operation_duration_seconds", Help: "Media operation duration.", Buckets: []float64{.1, .5, 1, 5, 15, 60, 180}}, []string{"operation"}),
	}
	m.registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), m.requests, m.requestDuration, m.operations, m.duration)
	return m
}
func (m *Metrics) Request(route string, status int, duration time.Duration) {
	m.requests.WithLabelValues(route, strconv.Itoa(status)).Inc()
	m.requestDuration.WithLabelValues(route).Observe(duration.Seconds())
}
func (m *Metrics) Media(operation, outcome string, duration time.Duration) {
	m.operations.WithLabelValues(operation, outcome).Inc()
	m.duration.WithLabelValues(operation).Observe(duration.Seconds())
}
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
