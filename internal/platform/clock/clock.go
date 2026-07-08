package clock

import "time"

func Now() time.Time {
	return time.Now().UTC()
}

func NowString() string {
	return Now().Format(time.RFC3339)
}
