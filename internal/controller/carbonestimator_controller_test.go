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

		var fakeProm *httptest.Server // mock Prometheus server

		// NamespacedName for the test resource
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			// mock Prometheus server
			fakeProm = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sustain-kube-system",
				},
			}
			_ = k8sClient.Create(ctx, ns)

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "carbon-intensity-secret",
					Namespace: "sustain-kube-system",
				},
				Data: map[string][]byte{
					"token": []byte("<electricity_maps_api_token>"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

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
			resource := &sustainkubecomv1alpha1.CarbonEstimator{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				By("Cleanup the specific resource instance CarbonEstimator")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			} else if !errors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			if fakeProm != nil {
				fakeProm.Close()
			}

			secret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "carbon-intensity-secret",
				Namespace: "sustain-kube-system",
			}, secret)
			if err == nil {
				By("Cleanup the carbon-intensity-secret")
				Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			} else if !errors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should successfully reconcile the resource", func() {
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
		})
	})
})
