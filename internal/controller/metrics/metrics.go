package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

type Metrics struct {
	PowerConsumption *prometheus.GaugeVec
	CarbonEmission   *prometheus.GaugeVec
	WarningLevel     *prometheus.GaugeVec
	CriticalLevel    *prometheus.GaugeVec
}

func SetupMetrics(prefix string) Metrics {
	carbonEstimatorMetrics := Metrics{
		PowerConsumption: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: prefix,
			Name:      "carbon_estimator_power_consumption",
			Help:      "Power consumption of the CarbonEstimator resource in Watts",
		}, []string{"name", "namespace"}),
		CarbonEmission: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: prefix,
			Name:      "carbon_estimator_carbon_emission",
			Help:      "Carbon emission of the CarbonEstimator resource in CO2e",
		}, []string{"name", "namespace"}),
		WarningLevel: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: prefix,
			Name:      "carbon_estimator_warning_level",
			Help:      "Info about CarbonEstimator resource",
		}, []string{"name", "namespace"}),
		CriticalLevel: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: prefix,
			Name:      "carbon_estimator_critical_level",
			Help:      "Info about CarbonEstimator resource",
		}, []string{"name", "namespace"}),
	}
	return carbonEstimatorMetrics
}

func (m Metrics) MustRegister(registry metrics.RegistererGatherer) Metrics {
	registry.MustRegister(
		m.PowerConsumption,
		m.CarbonEmission,
		m.WarningLevel,
		m.CriticalLevel,
	)
	return m
}

func (m *Metrics) Update(consumption, emission float64, warningLevel, criticalLevel uint, req ctrl.Request) {
	m.PowerConsumption.With(prometheus.Labels{
		"name":      req.Name,
		"namespace": req.Namespace,
	}).Set(consumption)

	m.CarbonEmission.With(prometheus.Labels{
		"name":      req.Name,
		"namespace": req.Namespace,
	}).Set(emission)

	m.WarningLevel.With(prometheus.Labels{
		"name":      req.Name,
		"namespace": req.Namespace,
	}).Set(float64(warningLevel))

	m.CriticalLevel.With(prometheus.Labels{
		"name":      req.Name,
		"namespace": req.Namespace,
	}).Set(float64(criticalLevel))
}

func (m *Metrics) Delete(req ctrl.Request) {
	m.PowerConsumption.Delete(prometheus.Labels{
		"name":      req.Name,
		"namespace": req.Namespace,
	})

	m.CarbonEmission.Delete(prometheus.Labels{
		"name":      req.Name,
		"namespace": req.Namespace,
	})

	m.WarningLevel.Delete(prometheus.Labels{
		"name":      req.Name,
		"namespace": req.Namespace,
	})

	m.CriticalLevel.Delete(prometheus.Labels{
		"name":      req.Name,
		"namespace": req.Namespace,
	})
}
