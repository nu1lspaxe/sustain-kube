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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CarbonEstimatorSpec defines the desired state of CarbonEstimator.
type CarbonEstimatorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	PrometheusURL string `json:"prometheusURL"`
	// +kubebuilder:validation:Minimum=1
	WarningLevel uint `json:"levelWarning"`
	// +kubebuilder:validation:Minimum=1
	CriticalLevel uint `json:"levelCritical"`
	// +kubebuilder:validation:Pattern=`^([1-9]\d*(\.\d)?)$`
	CPUPowerConsumption string `json:"powerConsumptionCPU"`
	// +kubebuilder:validation:Pattern=`^([1-9]\d*(\.\d)?)$`
	MemoryPowerConsumption string `json:"powerConsumptionMemory"`
}

// CarbonEstimatorStatus defines the observed state of CarbonEstimator.
type CarbonEstimatorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	CarbonIntensity string `json:"carbonIntensity,omitempty"` // 碳強度（從 API 拿值，之後再配合comsumption算出emission）
	Consumption     string `json:"consumption,omitempty"`
	Emission        string `json:"emission,omitempty"`
	State           string `json:"state,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// CarbonEstimator is the Schema for the carbonestimators API.
type CarbonEstimator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CarbonEstimatorSpec   `json:"spec,omitempty"`
	Status CarbonEstimatorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CarbonEstimatorList contains a list of CarbonEstimator.
type CarbonEstimatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CarbonEstimator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CarbonEstimator{}, &CarbonEstimatorList{})
}
