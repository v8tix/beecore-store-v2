package utils

import "time"

// NowTimeFormat time.ANSIC:  Tue Nov 10 23:00:00 2009
// time.UnixDate:  Tue Nov 10 23:00:00 UTC 2009
// time.RFC1123:  Tue, 10 Nov 2009 23:00:00 UTC
// time.RFC3339Nano:  2009-11-10T23:00:00Z
// time.RubyDate:  Tue Nov 10 23:00:00 +0000 2009
func NowTimeFormat(format string) string {
	currentTime := time.Now()
	return TimeFormat(currentTime, format)
}

func TimeFormat(ttl time.Time, format string) string {
	switch format {
	case time.ANSIC:
		return ttl.Format(time.ANSIC)
	case time.UnixDate:
		return ttl.Format(time.UnixDate)
	case time.RFC1123:
		return ttl.Format(time.RFC1123)
	case time.RFC3339Nano:
		return ttl.Format(time.RFC3339Nano)
	case time.RubyDate:
		return ttl.Format(time.RubyDate)
	default:
		return ttl.UTC().Format("02 Jan 2006 at 15:04 UTC")
	}
}
