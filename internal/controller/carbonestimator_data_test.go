//go:build unit
// +build unit

package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestCheckPrometheusHealth_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/-/healthy" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	log.SetLogger(logr.Discard())

	if err := checkPrometheusHealth(ts.URL); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestFetchPrometheusMetricAndCalculateConsumption(t *testing.T) {
	// create a fake Prometheus server that returns different values depending on query
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		resp := struct {
			Status string `json:"status"`
			Data   struct {
				Result []struct {
					Value []interface{} `json:"value"`
				} `json:"result"`
			} `json:"data"`
		}{
			Status: "success",
		}
		var val string
		switch {
		case q == "sum(rate(container_cpu_usage_seconds_total{container!=''}[5m]))":
			val = "2.5"
		case q == "sum(container_memory_working_set_bytes / 1073741824)":
			val = "1.5"
		default:
			// fallback
			val = "0"
		}
		resp.Data.Result = []struct {
			Value []interface{} `json:"value"`
		}{{Value: []interface{}{123, val}}}

		b, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	defer ts.Close()

	// calculateConsumption will query ts.URL/api/v1/query...; supply cpu and mem power values
	c, err := calculateConsumption(ts.URL, "10", "20")
	if err != nil {
		t.Fatalf("calculateConsumption failed: %v", err)
	}

	// expected: (cpuUsage * cpuPower) + (memUsage * memPower)
	// cpuUsage=2.5, cpuPower=10 => 25
	// memUsage=1.5, memPower=20 => 30
	// total = 55
	if c != 55 {
		t.Fatalf("unexpected consumption: got %v want %v", c, 55)
	}
}

func TestGetCarbonIntensity_UsesOverridableURL(t *testing.T) {
	// mock server to return carbonIntensity
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]float64{"carbonIntensity": 123.45}
		b, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	defer ts.Close()

	// override package variable
	old := carbonIntensityURL
	carbonIntensityURL = ts.URL
	defer func() { carbonIntensityURL = old }()

	v, err := getCarbonIntensity("token")
	if err != nil {
		t.Fatalf("getCarbonIntensity failed: %v", err)
	}
	if v != 123.45 {
		t.Fatalf("unexpected carbon intensity: got %v want %v", v, 123.45)
	}
}
