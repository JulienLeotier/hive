// Package metrics exposes Prometheus counters, histograms et gauges pour
// Hive. Registered dans un registry isolé (pas le DefaultRegisterer) pour
// éviter les collisions avec d'éventuels tests qui initialisent
// plusieurs instances d'un même metric.
//
// Observability plan :
//   - hive_api_requests_total{method, path, status}        counter
//   - hive_api_request_duration_seconds{method, path}      histogram
//   - hive_events_published_total{type}                    counter
//   - hive_bmad_skill_cost_usd{project, command, status}   counter
//   - hive_bmad_skill_duration_seconds{command}            histogram
//   - hive_projects_by_status{status}                      gauge
//
// Exposé via GET /metrics par le serveur API.
package metrics

import (
	"net/http"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	Registry = prometheus.NewRegistry()

	APIRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hive_api_requests_total",
			Help: "Nombre total de requêtes HTTP servies par l'API Hive.",
		},
		[]string{"method", "path", "status"},
	)

	APIRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hive_api_request_duration_seconds",
			Help:    "Latence des requêtes API en secondes.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	EventsPublished = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hive_events_published_total",
			Help: "Nombre d'events publiés sur le bus, ventilés par type.",
		},
		[]string{"type"},
	)

	BMADSkillCost = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hive_bmad_skill_cost_usd",
			Help: "Coût cumulé (en USD d'équivalent API) des invocations BMAD par commande et statut.",
		},
		[]string{"command", "status"},
	)

	BMADSkillDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hive_bmad_skill_duration_seconds",
			Help:    "Temps écoulé pour chaque invocation BMAD, en secondes.",
			Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 600, 1200, 1800, 3600, 7200},
		},
		[]string{"command"},
	)

	ProjectsByStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hive_projects_by_status",
			Help: "Compte instantané des projets groupés par statut.",
		},
		[]string{"status"},
	)
)

func init() {
	Registry.MustRegister(
		APIRequests,
		APIRequestDuration,
		EventsPublished,
		BMADSkillCost,
		BMADSkillDuration,
		ProjectsByStatus,
	)
}

// Handler renvoie un http.Handler qui sert /metrics au format Prometheus
// text-exposition v0.0.4.
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
		Registry:          Registry,
	})
}

// AttachBus branche un hook qui incrémente EventsPublished sur chaque
// publish. Registered après NewBus, désactivable via bus=nil.
func AttachBus(bus *event.Bus) {
	if bus == nil {
		return
	}
	bus.Subscribe("*", func(e event.Event) {
		EventsPublished.WithLabelValues(e.Type).Inc()
	})
}
