package metrics

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	applister "github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
)

const (
	// MetricsPath is the endpoint to collect application metrics
	MetricsPath = "/metrics"
)

var (
	descAppDefaultLabels = []string{"namespace", "name"}

	descAppInfo = prometheus.NewDesc(
		"argocd_app_info",
		"Information about application.",
		append(descAppDefaultLabels, "project", "repo", "dest_server", "dest_namespace"),
		nil,
	)
	descAppCreated = prometheus.NewDesc(
		"argocd_app_created_time",
		"Creation time in unix timestamp for an application.",
		descAppDefaultLabels,
		nil,
	)
	descAppSyncStatus = prometheus.NewDesc(
		"argocd_app_sync_status",
		"The application current sync status.",
		append(descAppDefaultLabels, "sync_status"),
		nil,
	)
	descAppHealthStatus = prometheus.NewDesc(
		"argocd_app_health_status",
		"The application current health status.",
		append(descAppDefaultLabels, "health_status"),
		nil,
	)
)

// NewMetricsServer returns a new prometheus server which collects application metrics
func NewMetricsServer(port int, appLister applister.ApplicationLister) *http.Server {
	mux := http.NewServeMux()
	appRegistry := NewAppRegistry(appLister)
	mux.Handle(MetricsPath, promhttp.HandlerFor(appRegistry, promhttp.HandlerOpts{}))
	return &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: mux,
	}
}

type appCollector struct {
	store applister.ApplicationLister
}

// NewAppCollector returns a prometheus collector for application metrics
func NewAppCollector(appLister applister.ApplicationLister) prometheus.Collector {
	return &appCollector{
		store: appLister,
	}
}

// NewAppRegistry creates a new prometheus registry that collects applications
func NewAppRegistry(appLister applister.ApplicationLister) *prometheus.Registry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(NewAppCollector(appLister))
	return registry
}

// Describe implements the prometheus.Collector interface
func (c *appCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descAppInfo
	ch <- descAppCreated
	ch <- descAppSyncStatus
	ch <- descAppHealthStatus
}

// Collect implements the prometheus.Collector interface
func (c *appCollector) Collect(ch chan<- prometheus.Metric) {
	apps, err := c.store.List(labels.NewSelector())
	if err != nil {
		log.Warnf("Failed to collect applications: %v", err)
		return
	}
	for _, app := range apps {
		collectApps(ch, app)
	}
}

func boolFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func collectApps(ch chan<- prometheus.Metric, app *argoappv1.Application) {
	addConstMetric := func(desc *prometheus.Desc, t prometheus.ValueType, v float64, lv ...string) {
		lv = append([]string{app.Namespace, app.Name}, lv...)
		ch <- prometheus.MustNewConstMetric(desc, t, v, lv...)
	}
	addGauge := func(desc *prometheus.Desc, v float64, lv ...string) {
		addConstMetric(desc, prometheus.GaugeValue, v, lv...)
	}

	addGauge(descAppInfo, 1, app.Spec.Project, app.Spec.Source.RepoURL, app.Spec.Destination.Server, app.Spec.Destination.Namespace)

	addGauge(descAppCreated, float64(app.CreationTimestamp.Unix()))

	syncStatus := app.Status.ComparisonResult.Status
	addGauge(descAppSyncStatus, boolFloat64(syncStatus == argoappv1.ComparisonStatusSynced), string(argoappv1.ComparisonStatusSynced))
	addGauge(descAppSyncStatus, boolFloat64(syncStatus == argoappv1.ComparisonStatusOutOfSync), string(argoappv1.ComparisonStatusOutOfSync))
	addGauge(descAppSyncStatus, boolFloat64(syncStatus == argoappv1.ComparisonStatusUnknown || syncStatus == ""), string(argoappv1.ComparisonStatusUnknown))

	healthStatus := app.Status.Health.Status
	addGauge(descAppHealthStatus, boolFloat64(healthStatus == argoappv1.HealthStatusUnknown || healthStatus == ""), string(argoappv1.HealthStatusUnknown))
	addGauge(descAppHealthStatus, boolFloat64(healthStatus == argoappv1.HealthStatusProgressing), string(argoappv1.HealthStatusProgressing))
	addGauge(descAppHealthStatus, boolFloat64(healthStatus == argoappv1.HealthStatusHealthy), string(argoappv1.HealthStatusHealthy))
	addGauge(descAppHealthStatus, boolFloat64(healthStatus == argoappv1.HealthStatusDegraded), string(argoappv1.HealthStatusDegraded))
	addGauge(descAppHealthStatus, boolFloat64(healthStatus == argoappv1.HealthStatusMissing), string(argoappv1.HealthStatusMissing))
}
