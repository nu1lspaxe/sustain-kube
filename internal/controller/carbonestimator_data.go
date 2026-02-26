package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

var carbonIntensityURL string

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

// calculateConsumption sends an HTTP GET request to the specified Prometheus query URL
// to get the total power consumption of the cluster. It returns the result as a float64 in Watts.
//
// The query is determined by the powerMetricQuery argument. If empty, it defaults to:
//
//	sum(node_power_watts)
func calculateConsumption(prometheusURL, powerMetricQuery string) (float64, error) {

	query := powerMetricQuery
	if query == "" {
		// default fallback query assuming an exporter exposes 'node_power_watts'
		query = "sum(node_power_watts)"
	}

	fullURL := fmt.Sprintf(
		"%s/api/v1/query?query=%s",
		prometheusURL,
		url.QueryEscape(query),
	)

	consumption, err := fetchPrometheusMetric(fullURL)
	if err != nil {
		log.Log.Error(err, "Unable to fetch power consumption data")
		return -1, err
	}

	return consumption, nil
}

func getCarbonIntensity(token string) (float64, error) {
	// allow overriding in tests
	var targetURL = carbonIntensityURL // provide to internal test
	if targetURL == "" {
		targetURL = os.Getenv("CARBON_INTENSITY_URL") // provide to E2E test
	}
	if targetURL == "" {
		targetURL = "https://api.electricitymap.org/v3/carbon-intensity/latest?zone=TW"
	}

	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("auth-token", token)

	// HTTP Request
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Log.Error(err, "Error closing carbon intensity API response body")
		}
	}()

	// 若 API 請求失敗
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("carbon intensity API error: %s", string(body))
	}

	// 將回傳的 JSON 解析進 result
	var result struct {
		CarbonIntensity float64 `json:"carbonIntensity"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read carbon intensity response: %w", err)
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("failed to parse carbon intensity JSON: %w", err)
	}

	return result.CarbonIntensity, nil
}
