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
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sustainkubecomv1alpha1 "sustain_kube/api/v1alpha1"
	"sustain_kube/internal/controller/metrics"
)

// CarbonEstimatorReconciler reconciles a CarbonEstimator object
type CarbonEstimatorReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Metrics metrics.Metrics
}

// +kubebuilder:rbac:groups=sustain-kube.com,resources=carbonestimators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sustain-kube.com,resources=carbonestimators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sustain-kube.com,resources=carbonestimators/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CarbonEstimator object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/reconcile
func (r *CarbonEstimatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	var carbonEstimator sustainkubecomv1alpha1.CarbonEstimator
	if err := r.Get(ctx, req.NamespacedName, &carbonEstimator); err != nil {
		if errors.IsNotFound(err) {
			log.Log.Info("CarbonEstimator resource not found")

			r.Metrics.PowerConsumption.Delete(prometheus.Labels{
				"name":      req.Name,
				"namespace": req.Namespace,
			})

			r.Metrics.CarbonEmission.Delete(prometheus.Labels{
				"name":      req.Name,
				"namespace": req.Namespace,
			})
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := checkPrometheusHealth(carbonEstimator.Spec.PrometheusURL); err != nil {
		log.Log.Error(err, "Prometheus is not healthy")
		return ctrl.Result{}, err
	}

	consumption, err := calculateConsumption(
		carbonEstimator.Spec.PrometheusURL,
		carbonEstimator.Spec.CPUPowerConsumption,
		carbonEstimator.Spec.MemoryPowerConsumption,
	)

	r.Metrics.PowerConsumption.With(prometheus.Labels{
		"name":      req.Name,
		"namespace": req.Namespace,
	}).Set(consumption)

	r.Metrics.CarbonEmission.With(prometheus.Labels{
		"name":      req.Name,
		"namespace": req.Namespace,
	}).Set(consumption)

	carbonEstimator.Status.Consumption = strconv.FormatFloat(consumption, 'f', 2, 64)
	carbonEstimator.Status.Emission = carbonEstimator.Status.Consumption

	if err != nil {
		carbonEstimator.Status.State = "Error"
		return ctrl.Result{}, err
	}

	if consumption > float64(carbonEstimator.Spec.CriticalLevel) {
		carbonEstimator.Status.State = "Critical"
	} else if consumption > float64(carbonEstimator.Spec.WarningLevel) {
		carbonEstimator.Status.State = "Warning"
	} else {
		carbonEstimator.Status.State = "Normal"
	}

	if err := r.Status().Update(ctx, &carbonEstimator); err != nil {
		carbonEstimator.Status.State = "Error"
		return ctrl.Result{}, err
	}

	log.Log.Info("Successfully reconciled CarbonEstimator")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CarbonEstimatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sustainkubecomv1alpha1.CarbonEstimator{}).
		Named("carbonestimator").
		Complete(r)
}
