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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sustainkubecomv1alpha1 "sustain_kube/api/v1alpha1"
	"sustain_kube/internal/controller/metrics"

	corev1 "k8s.io/api/core/v1"
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
			r.Metrics.Delete(req)
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

	if err != nil {
		carbonEstimator.Error()
		return ctrl.Result{}, err
	}

	// 取得碳強度
	// 從 spec.SecretRef 取得 Secret 的 name 與 Namespace
	if carbonEstimator.Spec.SecretRef == nil {
		err := fmt.Errorf("SecretRef is not defined in spec")
		log.Log.Error(err, "Missing SecretRef")
		return ctrl.Result{}, err
	}

	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{
		Name:      carbonEstimator.Spec.SecretRef.Name,
		Namespace: carbonEstimator.Spec.SecretRef.Namespace,
	}, &secret); err != nil {
		log.Log.Error(err, "Unable to fetch secret")
		return ctrl.Result{}, err
	}

	// 取得 Secret 中的 token
	tokenBytes, ok := secret.Data["electricity-maps-token"]
	if !ok {
		err := fmt.Errorf("token not found in secret")
		log.Log.Error(err, "Missing token in secret")
		return ctrl.Result{}, err
	}
	token := string(tokenBytes)

	//用token去抓carbonIntensity
	zone := carbonEstimator.Spec.TimeZone
	carbonIntensity, err := getCarbonIntensity(token, zone)
	if err != nil {
		log.Log.Error(err, "Failed to fetch carbon intensity")
		return ctrl.Result{}, err
	}

	//存入 Status 的 CarbonIntensity
	carbonEstimator.Status.CarbonIntensity = strconv.FormatFloat(carbonIntensity, 'f', 2, 64)

	r.Metrics.Update(
		consumption,
		consumption*carbonIntensity,
		carbonEstimator.Spec.WarningLevel,
		carbonEstimator.Spec.CriticalLevel,
		req)

	carbonEstimator.UpdateStatus(consumption, consumption*carbonIntensity)

	if err := r.Status().Update(ctx, &carbonEstimator); err != nil {
		log.Log.Error(err, "Failed to update CarbonEstimator status")
		carbonEstimator.Error()

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
