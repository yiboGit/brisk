package utils

import (
	"time"
)

// ToFullTimeString YYYY-MM-DD HH:mm:SS
func ToFullTimeString(t *time.Time) string {
	if t == nil {
		return time.Now().Format("2006-01-02 15:04:05")
	}
	return t.Format("2006-01-02 15:04:05")
}
