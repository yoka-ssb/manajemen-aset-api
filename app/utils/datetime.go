package utils

import "time"

func CountMonths(from time.Time, to time.Time) int {
	from = from.AddDate(0, 0, -from.Day()+1) // set to first day of month
	to = to.AddDate(0, 0, -to.Day()+1)       // set to first day of month

	months := 0
	for from.Before(to) {
		from = from.AddDate(0, 1, 0)
		months++
	}

	return months
}