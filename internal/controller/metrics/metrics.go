package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

type Metrics struct {
	PowerConsumption *prometheus.GaugeVec
	CarbonEmission   *prometheus.GaugeVec
}

func SetupMetrics(prefix string) Metrics {
	carbonEstimatorMetrics := Metrics{
		PowerConsumption: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: prefix,
			Name:      "carbon_estimator_carbon_emission",
			Help:      "Info about CarbonEstimator resource",
		}, []string{"name", "namespace"}),
		CarbonEmission: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: prefix,
			Name:      "carbon_estimator_power_consumption",
			Help:      "Info about CarbonEstimator resource",
		}, []string{"name", "namespace"}),
	}
	return carbonEstimatorMetrics
}

func (customMetrics Metrics) MustRegister(registry metrics.RegistererGatherer) Metrics {
	registry.MustRegister(
		customMetrics.PowerConsumption,
		customMetrics.CarbonEmission,
	)
	return customMetrics
}
