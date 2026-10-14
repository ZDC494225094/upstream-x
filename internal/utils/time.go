package utils

import "time"

func RangeStart(rangeValue string) time.Time {
	now := time.Now()
	switch rangeValue {
	case "3d":
		return now.AddDate(0, 0, -3)
	case "7d":
		return now.AddDate(0, 0, -7)
	case "30d":
		return now.AddDate(0, 0, -30)
	case "90d":
		return now.AddDate(0, 0, -90)
	default:
		return time.Time{}
	}
}

func PtrFloat64(v float64) *float64 {
	return &v
}

func PtrTime(v time.Time) *time.Time {
	return &v
}
