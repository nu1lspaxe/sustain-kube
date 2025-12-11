/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sustain_kube/test/utils"
)

// namespace where the project is deployed in
const namespace = "sustain-kube-system"

// serviceAccountName created for the project
const serviceAccountName = "sustain-kube-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "sustain-kube-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "controller-manager-metrics-service"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// installing CRDs, and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			cmd := exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=sustain-kube-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("validating that the ServiceMonitor for Prometheus is applied in the namespace")
			cmd = exec.Command("kubectl", "get", "ServiceMonitor", "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "ServiceMonitor should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("controller-runtime.metrics\tServing metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:7.78.0",
				"--", "/bin/sh", "-c", fmt.Sprintf(
					"curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics",
					token, metricsServiceName, namespace))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			metricsOutput := getMetricsOutput()
			Expect(metricsOutput).To(ContainSubstring(
				"controller_runtime_reconcile_total",
			))
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks

		// ------------------------------------------------------------------
		// [語法修正版] Happy Path - 同時 Mock Prometheus 與 Carbon API
		// ------------------------------------------------------------------
		Context("When connecting to a Mock Prometheus service", func() {
			const (
				mockPromName     = "mock-prometheus"
				mockPromConfig   = "mock-prometheus-config"
				mockCarbonName   = "mock-carbon"
				mockCarbonConfig = "mock-carbon-config"
				estimatorName    = "happy-path-estimator"
				secretName       = "carbon-intensity-secret"
			)

			BeforeEach(func() {
				// 1. 準備 Secret
				By("creating the prerequisite Secret")
				// 忽略錯誤的刪除指令不需要檢查 error
				_, _ = utils.Run(exec.Command("kubectl", "delete", "secret", secretName, "-n", namespace, "--ignore-not-found"))

				cmd := exec.Command("kubectl", "create", "secret", "generic", secretName,
					"--from-literal=token=dummy-e2e-token",
					"-n", namespace)

				_, err := utils.Run(cmd)
				Expect(err).To(Succeed())

				// ==========================================
				// 2. 部署 Mock Prometheus (Nginx)
				// ==========================================
				By("creating Mock Prometheus ConfigMap")
				nginxPromConf := fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  nginx.conf: |
    events {}
    http {
      server {
        listen 80;
        location /-/healthy { return 200 'ok'; }
        location / {
          default_type application/json;
          return 200 '{"status":"success","data":{"result":[{"value":[1716300000,"100"]}]}}';
        }
      }
    }
`, mockPromConfig, namespace)

				tmpPromCM := filepath.Join(os.TempDir(), "mock_prom_cm.yaml")
				Expect(os.WriteFile(tmpPromCM, []byte(nginxPromConf), 0644)).To(Succeed())

				_, _ = utils.Run(exec.Command("kubectl", "delete", "configmap", mockPromConfig, "-n", namespace, "--ignore-not-found"))

				cmd = exec.Command("kubectl", "apply", "-f", tmpPromCM)

				_, err = utils.Run(cmd)
				Expect(err).To(Succeed())

				By("deploying Mock Prometheus Pod & Service")
				_, _ = utils.Run(exec.Command("kubectl", "delete", "pod", mockPromName, "-n", namespace, "--ignore-not-found"))

				podPromYAML := fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
spec:
  containers:
  - name: nginx
    image: nginx:alpine
    ports:
    - containerPort: 80
    volumeMounts:
    - name: config
      mountPath: /etc/nginx/nginx.conf
      subPath: nginx.conf
  volumes:
  - name: config
    configMap:
      name: %s
`, mockPromName, namespace, mockPromName, mockPromConfig)

				tmpPromPod := filepath.Join(os.TempDir(), "mock_prom_pod.yaml")
				Expect(os.WriteFile(tmpPromPod, []byte(podPromYAML), 0644)).To(Succeed())

				cmd = exec.Command("kubectl", "apply", "-f", tmpPromPod)

				_, err = utils.Run(cmd)
				Expect(err).To(Succeed())

				_, _ = utils.Run(exec.Command("kubectl", "delete", "service", mockPromName, "-n", namespace, "--ignore-not-found"))

				cmd = exec.Command("kubectl", "expose", "pod", mockPromName, "--port=80", "--target-port=80", "-n", namespace)

				_, err = utils.Run(cmd)
				Expect(err).To(Succeed())

				// ==========================================
				// 3. 部署 Mock Carbon API (Nginx)
				// ==========================================
				By("creating Mock Carbon API ConfigMap")
				nginxCarbonConf := fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  nginx.conf: |
    events {}
    http {
      server {
        listen 80;
        location / {
          default_type application/json;
          return 200 '{"carbonIntensity": 300}';
        }
      }
    }
`, mockCarbonConfig, namespace)

				tmpCarbonCM := filepath.Join(os.TempDir(), "mock_carbon_cm.yaml")
				Expect(os.WriteFile(tmpCarbonCM, []byte(nginxCarbonConf), 0644)).To(Succeed())

				_, _ = utils.Run(exec.Command("kubectl", "delete", "configmap", mockCarbonConfig, "-n", namespace, "--ignore-not-found"))

				cmd = exec.Command("kubectl", "apply", "-f", tmpCarbonCM)
				_, err = utils.Run(cmd)
				Expect(err).To(Succeed())

				By("deploying Mock Carbon Pod & Service")
				_, _ = utils.Run(exec.Command("kubectl", "delete", "pod", mockCarbonName, "-n", namespace, "--ignore-not-found"))

				podCarbonYAML := fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
spec:
  containers:
  - name: nginx
    image: nginx:alpine
    ports:
    - containerPort: 80
    volumeMounts:
    - name: config
      mountPath: /etc/nginx/nginx.conf
      subPath: nginx.conf
  volumes:
  - name: config
    configMap:
      name: %s
`, mockCarbonName, namespace, mockCarbonName, mockCarbonConfig)

				tmpCarbonPod := filepath.Join(os.TempDir(), "mock_carbon_pod.yaml")
				Expect(os.WriteFile(tmpCarbonPod, []byte(podCarbonYAML), 0644)).To(Succeed())

				cmd = exec.Command("kubectl", "apply", "-f", tmpCarbonPod)
				_, err = utils.Run(cmd)
				Expect(err).To(Succeed())

				_, _ = utils.Run(exec.Command("kubectl", "delete", "service", mockCarbonName, "-n", namespace, "--ignore-not-found"))

				cmd = exec.Command("kubectl", "expose", "pod", mockCarbonName, "--port=80", "--target-port=80", "-n", namespace)
				_, err = utils.Run(cmd)
				Expect(err).To(Succeed())

				// ==========================================
				// 4. Patch Controller 使用 Mock URL
				// ==========================================
				By("patching Controller to use Mock Carbon API via Env Var")
				mockCarbonURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:80", mockCarbonName, namespace)

				patchCmd := exec.Command("kubectl", "set", "env", "deployment/sustain-kube-controller-manager",
					fmt.Sprintf("CARBON_INTENSITY_URL=%s", mockCarbonURL),
					"-n", namespace)
				_, err = utils.Run(patchCmd)
				Expect(err).To(Succeed())

				By("waiting for Controller Deployment rollout to finish")
				rolloutCmd := exec.Command("kubectl", "rollout", "status", "deployment/sustain-kube-controller-manager", "-n", namespace)
				_, err = utils.Run(rolloutCmd)
				Expect(err).To(Succeed())

				By("refreshing the controller pod name for logs")
				getPodCmd := exec.Command("kubectl", "get", "pods", "-l", "control-plane=controller-manager",
					"-o", "jsonpath={.items[0].metadata.name}", "-n", namespace)
				newPodName, err := utils.Run(getPodCmd)
				Expect(err).To(Succeed())
				controllerPodName = newPodName // 更新全域變數
				fmt.Printf("Controller restarted. New Pod Name: %s\n", controllerPodName)

				By("waiting for Mock Services to be ready")
				verifyPodRunning := func(podName string) func(Gomega) {
					return func(g Gomega) {
						cmd := exec.Command("kubectl", "get", "pod", podName, "-n", namespace, "-o", "jsonpath={.status.phase}")
						out, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(out).To(Equal("Running"))
					}
				}
				Eventually(verifyPodRunning(mockPromName), 2*time.Minute, 1*time.Second).Should(Succeed())
				Eventually(verifyPodRunning(mockCarbonName), 2*time.Minute, 1*time.Second).Should(Succeed())
			})

			AfterEach(func() {
				// 清理所有資源
				_, _ = utils.Run(exec.Command("kubectl", "delete", "carbonestimator", estimatorName, "-n", namespace, "--ignore-not-found"))
				_, _ = utils.Run(exec.Command("kubectl", "delete", "service", mockPromName, "-n", namespace, "--ignore-not-found"))
				_, _ = utils.Run(exec.Command("kubectl", "delete", "pod", mockPromName, "-n", namespace, "--ignore-not-found"))
				_, _ = utils.Run(exec.Command("kubectl", "delete", "configmap", mockPromConfig, "-n", namespace, "--ignore-not-found"))

				_, _ = utils.Run(exec.Command("kubectl", "delete", "service", mockCarbonName, "-n", namespace, "--ignore-not-found"))
				_, _ = utils.Run(exec.Command("kubectl", "delete", "pod", mockCarbonName, "-n", namespace, "--ignore-not-found"))
				_, _ = utils.Run(exec.Command("kubectl", "delete", "configmap", mockCarbonConfig, "-n", namespace, "--ignore-not-found"))

				_, _ = utils.Run(exec.Command("kubectl", "delete", "secret", secretName, "-n", namespace, "--ignore-not-found"))

				_, _ = utils.Run(exec.Command("kubectl", "set", "env", "deployment/sustain-kube-controller-manager", "CARBON_INTENSITY_URL-", "-n", namespace))

				_, _ = utils.Run(exec.Command("kubectl", "rollout", "status", "deployment/sustain-kube-controller-manager", "-n", namespace))
			})

			It("should retrieve metrics from Mock Prometheus AND Carbon API, then update status", func() {
				By("creating a CarbonEstimator pointing to the Mock Prometheus Service")
				mockPromURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:80", mockPromName, namespace)

				crYAML := fmt.Sprintf(`
apiVersion: sustain-kube.com/v1alpha1
kind: CarbonEstimator
metadata:
  name: %s
  namespace: %s
spec:
  prometheusURL: "%s"
  powerConsumptionCPU: "15.0"
  powerConsumptionMemory: "1.5"
  levelWarning: 5000
  levelCritical: 10000
`, estimatorName, namespace, mockPromURL)

				tmpFile := filepath.Join(os.TempDir(), "sustain_kube_happy_cr.yaml")
				Expect(os.WriteFile(tmpFile, []byte(crYAML), 0644)).To(Succeed())

				cmd := exec.Command("kubectl", "apply", "-f", tmpFile)
				_, err := utils.Run(cmd)
				Expect(err).To(Succeed())

				// 驗證 1: 能耗計算
				By("verifying the Consumption is calculated successfully")
				verifyConsumption := func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "carbonestimator", estimatorName,
						"-n", namespace,
						"-o", "jsonpath={.status.consumption}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(ContainSubstring("1650"), "Expected consumption to be 1650, got %s", output)
				}
				Eventually(verifyConsumption, 2*time.Minute, 1*time.Second).Should(Succeed())

				// 驗證 2: 碳排計算
				By("verifying the Emission is calculated successfully")
				verifyEmission := func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "carbonestimator", estimatorName,
						"-n", namespace,
						"-o", "jsonpath={.status.emission}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(ContainSubstring("495000"), "Expected emission to be 495000, got %s", output)
				}
				Eventually(verifyEmission, 2*time.Minute, 1*time.Second).Should(Succeed())
			})
		})

		Context("When creating a CarbonEstimator resource", func() {
			const (
				estimatorName = "e2e-test-estimator"
				secretName    = "carbon-intensity-secret"
			)

			BeforeEach(func() {
				By("creating the prerequisite Secret for Carbon Intensity API")
				cmd := exec.Command("kubectl", "create", "secret", "generic", secretName,
					"--from-literal=token=dummy-e2e-token",
					"-n", namespace)
				_, err := utils.Run(cmd)
				if err != nil {
					_ = exec.Command("kubectl", "delete", "secret", secretName, "-n", namespace).Run()
					_, _ = utils.Run(cmd)
				}
			})

			AfterEach(func() {
				By("removing the custom resource")
				_, _ = utils.Run(exec.Command("kubectl", "delete", "carbonestimator", estimatorName, "-n", namespace))

				By("removing the secret")
				_, _ = utils.Run(exec.Command("kubectl", "delete", "secret", secretName, "-n", namespace))
			})

			It("should attempt to reconcile and update the resource status", func() {
				By("creating a CarbonEstimator custom resource")
				crYAML := fmt.Sprintf(`
apiVersion: sustain-kube.com/v1alpha1
kind: CarbonEstimator
metadata:
  name: %s
  namespace: %s
spec:
  prometheusURL: "http://non-existent-prometheus:9090"
  powerConsumptionCPU: "10.0"
  powerConsumptionMemory: "20.0"
  levelWarning: 50
  levelCritical: 100
`, estimatorName, namespace)

				tmpFile := filepath.Join(os.TempDir(), "sustain_kube_e2e_cr.yaml")
				Expect(os.WriteFile(tmpFile, []byte(crYAML), 0644)).To(Succeed())

				cmd := exec.Command("kubectl", "apply", "-f", tmpFile)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "Failed to apply CarbonEstimator CR")

				By("verifying the controller reconciles and updates Status to Error")
				verifyReconcileLoop := func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "carbonestimator", estimatorName,
						"-n", namespace,
						"-o", "jsonpath={.status.state}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("Error"), "Controller should update the status to Error due to unreachable endpoints")
				}
				Eventually(verifyReconcileLoop, 1*time.Minute, 1*time.Second).Should(Succeed())

				By("checking the error message in status")
				verifyErrorMessage := func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "carbonestimator", estimatorName,
						"-n", namespace,
						"-o", "jsonpath={.status.errorMessage}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).NotTo(BeEmpty())
					g.Expect(output).To(ContainSubstring("lookup"), "Error message should mention lookup failure for fake url")
				}
				Eventually(verifyErrorMessage, 1*time.Minute, 1*time.Second).Should(Succeed())
			})
		})

	})
})

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() string {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
	return metricsOutput
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
