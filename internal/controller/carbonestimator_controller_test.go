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

package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlMetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	sustainkubecomv1alpha1 "sustain_kube/api/v1alpha1"
	"sustain_kube/internal/controller/metrics"
)

var _ = Describe("CarbonEstimator Controller", func() {

	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"
		ctx := context.Background()

		var fakeProm *httptest.Server         // mock Prometheus server
		var fakeCarbonServer *httptest.Server // mock Carbon Intensity server

		// NamespacedName for the test resource
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			// 1. Mock Prometheus server
			fakeProm = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Handle Health Check
				if r.URL.Path == "/-/healthy" {
					w.WriteHeader(http.StatusOK)
					return
				}
				// Handle Query
				w.Header().Set("Content-Type", "application/json")
				_, err := fmt.Fprintln(w, `{
					"status": "success",
					"data": {
						"resultType": "vector",
						"result": [
							{
								"metric": {},
								"value": [ 1234567890, "42" ]
							}
						]
					}
				}`)
				Expect(err).NotTo(HaveOccurred())
			}))

			// 2. Mock Carbon Intensity server (Electricity Maps)
			fakeCarbonServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				// Return a fake carbon intensity value
				_, err := fmt.Fprintln(w, `{"carbonIntensity": 300.5}`)
				Expect(err).NotTo(HaveOccurred())
			}))

			carbonIntensityURL = fakeCarbonServer.URL

			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sustain-kube-system",
				},
			}
			// Use Create or Update to avoid "already exists" error in repeated tests
			_ = k8sClient.Create(ctx, ns)

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "carbon-intensity-secret",
					Namespace: "sustain-kube-system",
				},
				Data: map[string][]byte{
					"token": []byte("dummy-token"),
				},
			}
			// Use Create or Update
			_ = k8sClient.Create(ctx, secret)

			By("creating the custom resource for the Kind CarbonEstimator")
			carbonestimator := &sustainkubecomv1alpha1.CarbonEstimator{}
			err := k8sClient.Get(ctx, typeNamespacedName, carbonestimator)
			if err != nil && errors.IsNotFound(err) {
				resource := &sustainkubecomv1alpha1.CarbonEstimator{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: sustainkubecomv1alpha1.CarbonEstimatorSpec{
						PrometheusURL:          fakeProm.URL,
						CPUPowerConsumption:    "10.5",
						MemoryPowerConsumption: "20.0",
						WarningLevel:           1,
						CriticalLevel:          5,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// Cleanup CarbonEstimator
			resource := &sustainkubecomv1alpha1.CarbonEstimator{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				By("Cleanup the specific resource instance CarbonEstimator")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}

			// Cleanup Secret
			secret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "carbon-intensity-secret",
				Namespace: "sustain-kube-system",
			}, secret)
			if err == nil {
				By("Cleanup the carbon-intensity-secret")
				Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			}

			// Close Fake Servers
			if fakeProm != nil {
				fakeProm.Close()
			}
			if fakeCarbonServer != nil {
				fakeCarbonServer.Close()
			}

			// Reset the global variable to avoid side effects
			carbonIntensityURL = ""
		})

		It("should successfully reconcile the resource and update status", func() {
			By("Reconciling the created resource")
			metricsObj := metrics.SetupMetrics("test").MustRegister(ctrlMetrics.Registry)
			controllerReconciler := &CarbonEstimatorReconciler{
				Client:  k8sClient,
				Scheme:  k8sClient.Scheme(),
				Metrics: metricsObj,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking the status of the resource")
			resource := &sustainkubecomv1alpha1.CarbonEstimator{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())

			Expect(resource.Status.State).To(Equal("Critical")) // 1281 > 5 (CriticalLevel)
			Expect(resource.Status.CarbonIntensity).To(Equal("300.50"))
			Expect(resource.Status.Consumption).To(Equal("1281.00")) // (42*10.5) + (42*20.0) = 441 + 840 = 1281
			Expect(resource.Status.Emission).To(Equal("384940.50"))  // 1281 * 300.5 = 384940.5
			Expect(resource.Status.ErrorMessage).To(BeEmpty())
		})

		It("should set error status when Prometheus is unreachable", func() {
			// Close the mock Prometheus server to simulate failure
			fakeProm.Close()
			fakeProm = nil // Set to nil so AfterEach doesn't panic

			metricsObj := metrics.SetupMetrics("test_prom_fail").MustRegister(ctrlMetrics.Registry)
			controllerReconciler := &CarbonEstimatorReconciler{
				Client:  k8sClient,
				Scheme:  k8sClient.Scheme(),
				Metrics: metricsObj,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			// Reconcile returns error, but we also want to check if Status is updated
			Expect(err).To(HaveOccurred())

			By("Checking the status for error")
			resource := &sustainkubecomv1alpha1.CarbonEstimator{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())

			Expect(resource.Status.State).To(Equal("Error"))
			Expect(resource.Status.ErrorMessage).NotTo(BeEmpty())
		})

		It("should set error status when Secret is missing", func() {
			// Delete the secret
			secret := &corev1.Secret{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "carbon-intensity-secret",
				Namespace: "sustain-kube-system",
			}, secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())

			metricsObj := metrics.SetupMetrics("test_secret_fail").MustRegister(ctrlMetrics.Registry)
			controllerReconciler := &CarbonEstimatorReconciler{
				Client:  k8sClient,
				Scheme:  k8sClient.Scheme(),
				Metrics: metricsObj,
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).To(HaveOccurred())

			By("Checking the status for error")
			resource := &sustainkubecomv1alpha1.CarbonEstimator{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())

			Expect(resource.Status.State).To(Equal("Error"))
			Expect(resource.Status.ErrorMessage).NotTo(BeEmpty())
		})

		It("should set error status when Carbon Intensity API fails", func() {
			// Close the mock Carbon server to simulate failure
			fakeCarbonServer.Close()
			fakeCarbonServer = nil

			metricsObj := metrics.SetupMetrics("test_carbon_fail").MustRegister(ctrlMetrics.Registry)
			controllerReconciler := &CarbonEstimatorReconciler{
				Client:  k8sClient,
				Scheme:  k8sClient.Scheme(),
				Metrics: metricsObj,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).To(HaveOccurred())

			By("Checking the status for error")
			resource := &sustainkubecomv1alpha1.CarbonEstimator{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())

			Expect(resource.Status.State).To(Equal("Error"))
			Expect(resource.Status.ErrorMessage).NotTo(BeEmpty())
		})
	})
})
