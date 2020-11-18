package utils

import (
	"time"
)

func StringInSlice(a string, list []string) bool {
    for _, b := range list {
        if b == a {
            return true
        }
    }
    return false
}

func FuzzyTimeToDuration(t string) (time.Duration, error) {
	if t == "daily" {
		return time.ParseDuration("24h")
	} else if t == "weekly" {
		return time.ParseDuration("168h")
	} else if t == "monthly" {
		return time.ParseDuration("720h")
	}

	return time.ParseDuration("24h")
}
