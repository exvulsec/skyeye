package utils

import (
	"fmt"
	"time"
)

func ElapsedTime(startTime time.Time) string {
	elapsedTime := time.Since(startTime)
	unit := "s"
	duration := elapsedTime.Seconds()

	if elapsedTime < 100*time.Millisecond {
		unit = "ms"
		duration = float64(elapsedTime.Nanoseconds()) / float64(time.Millisecond)
	}

	return fmt.Sprintf("%.2f%s", duration, unit)
}
