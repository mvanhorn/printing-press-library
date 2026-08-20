package source

import "time"

const SafariEpochOffsetSeconds float64 = 978307200

func SafariSecondsToTime(raw float64) time.Time {
	if raw <= 0 {
		return time.Time{}
	}
	unix := raw + SafariEpochOffsetSeconds
	sec := int64(unix)
	ns := int64((unix - float64(sec)) * float64(time.Second))
	if ns < 0 {
		ns = 0
	}
	return time.Unix(sec, ns).UTC()
}
