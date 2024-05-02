package utils

import (
	"fmt"
	"time"
)

func ElapsedTime(startTime time.Time) string {
	elapsedTime := time.Since(startTime)
	var unit string
	var duration float64
	if elapsedTime.Seconds() < 0.1 {
		unit = "ms"
		duration = float64(elapsedTime.Milliseconds())
	} else {
		unit = "s"
		duration = elapsedTime.Seconds()
	}
	return fmt.Sprintf("%.2f%s", duration, unit)
}
