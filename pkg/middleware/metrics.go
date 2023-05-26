package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/justinas/alice"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/negroni"
)

// DefaultMetricsHandler is the default http.Handler for serving metrics from
// the default prometheus.Registry
var DefaultMetricsHandler = NewMetricsHandlerWithDefaultRegistry()

// NewMetricsHandlerWithDefaultRegistry creates a new http.Handler for serving
// metrics from the default prometheus.Registry.
func NewMetricsHandlerWithDefaultRegistry() http.Handler {
	return NewMetricsHandler(prometheus.DefaultRegisterer, prometheus.DefaultGatherer)
}

// NewMetricsHandler creates a new http.Handler for serving metrics from the
// provided prometheus.Registerer and prometheus.Gatherer
func NewMetricsHandler(registerer prometheus.Registerer, gatherer prometheus.Gatherer) http.Handler {
	return promhttp.InstrumentMetricHandler(
		registerer, promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}),
	)
}

// NewRequestMetricsWithDefaultRegistry returns a middleware that will record
// metrics for HTTP requests to the default prometheus.Registry
func NewRequestMetricsWithDefaultRegistry() alice.Constructor {
	return NewRequestMetrics(prometheus.DefaultRegisterer)
}

// NewRequestMetrics returns a middleware that will record metrics for HTTP
// requests to the provided prometheus.Registerer
func NewRequestMetrics(registerer prometheus.Registerer) alice.Constructor {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {

			m := metrics{}
			start := time.Now()
			tenantID := getTenantIDFromRequest(req)
			lrw := negroni.NewResponseWriter(rw)
			tc := registerRequestsCounter(registerer, &m)
			tg := registerInflightRequestsGauge(registerer, &m)
			th := registerRequestsLatencyHistogram(registerer, &m)
			tg.With(prometheus.Labels{"tenantid": tenantID}).Inc()
			defer tg.With(prometheus.Labels{"tenantid": tenantID}).Dec()

			next.ServeHTTP(lrw, req)
			statusCode := lrw.Status()
			duration := time.Since(start)
			th.With(prometheus.Labels{"method": req.Method, "tenantid": tenantID}).Observe(duration.Seconds())
			tc.With(prometheus.Labels{"code": strconv.Itoa(statusCode), "tenantid": tenantID}).Inc()
		})
	}
}

func registerRequestsCounter(reg prometheus.Registerer, m *metrics) *prometheus.CounterVec {
	m.tenantRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "oauth2_proxy_requests_total",
			Help: "Total number of requests by HTTP status code.",
		},
		[]string{"code", "tenantid"},
	)

	if err := reg.Register(m.tenantRequests); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			m.tenantRequests = are.ExistingCollector.(*prometheus.CounterVec)
		} else {
			panic(err)
		}
	}
	return m.tenantRequests
}

// registerInflightRequestsGauge registers 'oauth2_proxy_requests_in_flight'
// This only keeps the count of currently in progress HTTP requests
func registerInflightRequestsGauge(registerer prometheus.Registerer, m *metrics) *prometheus.GaugeVec {
	m.tenantGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "oauth2_proxy_requests_in_flight",
		Help: "Current number of requests being served.",
	},
		[]string{"tenantid"},
	)

	if err := registerer.Register(m.tenantGauge); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			m.tenantGauge = are.ExistingCollector.(*prometheus.GaugeVec)
		} else {
			panic(err)
		}
	}

	return m.tenantGauge
}

// registerRequestsLatencyHistogram registers 'oauth2_proxy_response_duration_seconds'
// This keeps tally of the requests bucketed by the time taken to process the request
func registerRequestsLatencyHistogram(registerer prometheus.Registerer, m *metrics) *prometheus.HistogramVec {
	m.tenantHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "oauth2_proxy_response_duration_seconds",
			Help:    "A histogram of request latencies.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "tenantid"},
	)

	if err := registerer.Register(m.tenantHistogram); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			m.tenantHistogram = are.ExistingCollector.(*prometheus.HistogramVec)
		} else {
			panic(err)
		}
	}

	return m.tenantHistogram
}

type metrics struct {
	tenantRequests  *prometheus.CounterVec
	tenantHistogram *prometheus.HistogramVec
	tenantGauge     *prometheus.GaugeVec
}

func getTenantIDFromRequest(r *http.Request) string {
	query := r.URL.Query().Get("tenant-id")
	return query
}
