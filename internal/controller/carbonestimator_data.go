package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// checkPrometheusHealth verifies if Prometheus is healthy by querying its health endpoint.
func checkPrometheusHealth(prometheusURL string) error {
	healthURL := fmt.Sprintf("%s/-/healthy", prometheusURL)
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(healthURL)
	if err != nil {
		return fmt.Errorf("failed to connect to Prometheus: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Log.Error(err, "Error closing response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Prometheus is not healthy: received status code %d", resp.StatusCode)
	}

	log.Log.Info("Prometheus is healthy")

	return nil
}

// fetchPrometheusMetric sends an HTTP GET request to the specified Prometheus query URL
// and returns the first metric value as a float64.

func fetchPrometheusMetric(queryURL string) (float64, error) {
	resp, err := http.Get(queryURL)
	if err != nil {
		return 0, fmt.Errorf("error fetching data from Prometheus: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Log.Error(err, "Error closing response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response body: %v", err)
	}

	// HTTP API: https://prometheus.io/docs/prometheus/latest/querying/api/
	var result struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Value  []interface{}     `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("error unmarshalling JSON: %v", err)
	}

	if result.Status != "success" || len(result.Data.Result) == 0 {
		return 0, fmt.Errorf("no data returned from Prometheus")
	}

	valueStr, ok := result.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, fmt.Errorf("unexpected data format in Prometheus response")
	}

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing value: %v", err)
	}

	return value, nil
}

// calculateConsumption sends two HTTP GET requests to the specified Prometheus query URL
// and uses the two returned values to calculate the total power consumption of the
// cluster. It returns the result as a float64 in Watts.
//
// The two queries are:
//  1. sum(rate(container_cpu_usage_seconds_total[1h])) - the average CPU usage of all
//     containers in the cluster over the last hour.
//  2. sum(container_memory_working_set_bytes / 1073741824) - the total memory usage
//     of all containers in the cluster, in GiB.
//
// The function takes the CPU and memory power consumption values as parameters and
// multiplies them with the respective usage values to get the total power consumption
// in Watts.
func calculateConsumption(prometheusURL, cpuPowerConsumption, memPowerConsumption string) (float64, error) {

	cpuQuery := fmt.Sprintf(
		"%s/api/v1/query?query=%s",
		prometheusURL,
		url.QueryEscape("sum(rate(container_cpu_usage_seconds_total[1h]))"),
	)

	cpuUsage, err := fetchPrometheusMetric(cpuQuery)
	if err != nil {
		log.Log.Error(err, "Unable to fetch CPU data")
		return -1, err
	}

	memQuery := fmt.Sprintf(
		"%s/api/v1/query?query=%s",
		prometheusURL,
		url.QueryEscape("sum(container_memory_working_set_bytes / 1073741824)"))

	memoryUsage, err := fetchPrometheusMetric(memQuery)
	if err != nil {
		log.Log.Error(err, "Unable to fetch memory data")
		return -1, err
	}

	cpuPowerConsumptionFloat, err := strconv.ParseFloat(cpuPowerConsumption, 64)
	if err != nil {
		return -1, fmt.Errorf("error parsing CPU power consumption: %v", err)
	}

	memPowerConsumptionFloat, err := strconv.ParseFloat(memPowerConsumption, 64)
	if err != nil {
		return -1, fmt.Errorf("error parsing memory power consumption: %v", err)
	}

	totalConsumption := (cpuUsage * cpuPowerConsumptionFloat) + (memoryUsage * memPowerConsumptionFloat)

	return totalConsumption, nil
}
