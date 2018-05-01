package bodega

import (
	"fmt"
	"time"
)

const (
	oneDayInSeconds int = 86400
	oneHourInSeconds int = 3600
	oneMinuteInSeconds int = 60
)

func inSlice(s string, slice []string) bool {
	for _, elem := range slice {
		if s == elem {
			return true
		}
	}
	return false
}

func convert(timeDelta time.Duration) string {
	seconds := int(timeDelta.Seconds())
	days := seconds / oneDayInSeconds
	seconds = seconds % oneDayInSeconds
	hours := seconds / oneHourInSeconds
	seconds = seconds % oneHourInSeconds
	minutes := seconds / oneMinuteInSeconds
	seconds = seconds - minutes*oneMinuteInSeconds
	if days == 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d %02d:%02d:%02d", days, hours, minutes, seconds)
}
