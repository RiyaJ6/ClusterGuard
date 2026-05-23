// Package metrics registers and exposes Prometheus metrics for ClusterGuard.
package metrics

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds all Prometheus collectors for ClusterGuard.
type Metrics struct {
	EventsProcessed  prometheus.Counter
	AnomaliesTotal   prometheus.Counter
	ProcessingTime   prometheus.Histogram
	ConsumerLag      *prometheus.GaugeVec
	WebhookErrors    prometheus.Counter
}

// New registers all metrics with the default Prometheus registry and returns them.
func New() *Metrics {
	m := &Metrics{
		EventsProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "clusterguard_events_processed_total",
			Help: "Total number of events consumed from Kafka.",
		}),
		AnomaliesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "clusterguard_anomalies_detected_total",
			Help: "Total number of events flagged as anomalies.",
		}),
		ProcessingTime: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "clusterguard_processing_duration_seconds",
			Help:    "Time spent processing a single event.",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 12),
		}),
		ConsumerLag: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "clusterguard_consumer_lag",
			Help: "Kafka consumer lag by partition.",
		}, []string{"topic", "partition"}),
		WebhookErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "clusterguard_webhook_errors_total",
			Help: "Number of failed webhook alert deliveries.",
		}),
	}

	prometheus.MustRegister(
		m.EventsProcessed,
		m.AnomaliesTotal,
		m.ProcessingTime,
		m.ConsumerLag,
		m.WebhookErrors,
	)

	return m
}
