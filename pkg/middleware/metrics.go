package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/justinas/alice"
	tenantutils "github.com/oauth2-proxy/oauth2-proxy/v7/pkg/tenant/utils"
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
	m := metrics{}
	return func(next http.Handler) http.Handler {
		// Counter for all requests
		// This is bucketed based on the response code we set
		counterHandler := func(next http.Handler) http.Handler {
			return metricsCounterHandler(registerRequestsCounter(registerer, &m), next)
		}

		// Gauge to all requests currently being handled
		inFlightHandler := func(next http.Handler) http.Handler {
			return metricsInFlightHandler(registerInflightRequestsGauge(registerer, &m), next)
		}

		// The latency of all requests bucketed by HTTP method
		durationHandler := func(next http.Handler) http.Handler {
			return metricsHandlerDuration(registerRequestsLatencyHistogram(registerer, &m), next)
		}

		return alice.New(counterHandler, inFlightHandler, durationHandler).Then(next)
	}
}

func metricsCounterHandler(counter *prometheus.CounterVec, next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		lrw := negroni.NewResponseWriter(rw)
		next.ServeHTTP(lrw, req)
		statusCode := lrw.Status()
		tid := tenantutils.FromContext(req.Context())

		counter.With(prometheus.Labels{"code": strconv.Itoa(statusCode), "tenantid": tid}).Inc()
	})
}

func metricsInFlightHandler(g *prometheus.GaugeVec, next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		tid := tenantutils.FromContext(req.Context())
		g.With(prometheus.Labels{"tenantid": tid}).Inc()
		defer g.With(prometheus.Labels{"tenantid": tid}).Dec()

		next.ServeHTTP(rw, req)
	})

}

func metricsHandlerDuration(obs prometheus.ObserverVec, next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		start := time.Now()
		next.ServeHTTP(rw, req)
		duration := time.Since(start)
		tid := tenantutils.FromContext(req.Context())
		obs.With(prometheus.Labels{"method": req.Method, "tenantid": tid}).Observe(duration.Seconds())
	})
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
