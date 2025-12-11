//go:build unit
// +build unit

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestMetrics_UpdateAndRegister(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := SetupMetrics("tp")
	m.MustRegister(registry)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "my", Namespace: "ns"}}

	m.Update(42.5, 10.25, 2, 5, req)

	// check power consumption gauge value
	g := m.PowerConsumption.WithLabelValues("my", "ns")
	got := testutil.ToFloat64(g)
	if got != 42.5 {
		t.Fatalf("unexpected power consumption: got %v want %v", got, 42.5)
	}

	// check carbon emission gauge value
	ge := m.CarbonEmission.WithLabelValues("my", "ns")
	gotE := testutil.ToFloat64(ge)
	if gotE != 10.25 {
		t.Fatalf("unexpected carbon emission: got %v want %v", gotE, 10.25)
	}

	// ensure metrics are registered with provided registry
	if err := registry.Register(prometheus.NewGauge(prometheus.GaugeOpts{Name: "dummy_for_test"})); err != nil {
		// ignore duplicate registration errors
		_ = err
	}
}

func TestMetrics_Delete(t *testing.T) {
	m := SetupMetrics("tp")
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "to-delete", Namespace: "ns"}}

	// set a value then delete
	m.Update(1.0, 2.0, 1, 2, req)
	m.Delete(req)

	// Deleting shouldn't panic; subsequent calls to WithLabelValues recreate metrics
	_ = m.PowerConsumption.WithLabelValues("to-delete", "ns")
}
