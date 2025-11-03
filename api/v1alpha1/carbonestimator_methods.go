package v1alpha1

import (
	"strconv"
	"sustain_kube/internal/utils"
)

func (carbonEstimator *CarbonEstimator) UpdateStatus(consumption, emission float64) {

	carbonEstimator.Status.Consumption = strconv.FormatFloat(consumption, 'f', 2, 64)
	carbonEstimator.Status.Emission = strconv.FormatFloat(consumption*1.1, 'f', 2, 64)

	if consumption > float64(carbonEstimator.Spec.CriticalLevel) {
		carbonEstimator.Status.State = utils.CriticalStatus
	} else if consumption > float64(carbonEstimator.Spec.WarningLevel) {
		carbonEstimator.Status.State = utils.WarningStatus
	} else {
		carbonEstimator.Status.State = utils.NormalStatus
	}

}

// Error sets the status of the CarbonEstimator to Error
func (carbonEstimator *CarbonEstimator) Error(msg string) {

	carbonEstimator.Status.State = utils.ErrorStatus
	carbonEstimator.Status.Consumption = utils.ErrorInt
	carbonEstimator.Status.Emission = utils.ErrorInt
	carbonEstimator.Status.ErrorMessage = msg
}
