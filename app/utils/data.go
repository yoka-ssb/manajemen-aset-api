package utils

func ExtractMaintenancePeriod(periodId int32) int {
	var period int
	if periodId == 1 {
		period = 1
	} else if periodId == 2 {
		period = 2
	} else if periodId == 3 {
		period = 3
	} else if periodId == 4 {
		period = 6
	}
	return period
}