package help

import "time"

func TimestampToDateStr(ts int64) string {
	return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
}

func TimestampToDate(ts int64) time.Time {
	return time.Unix(ts, 0)
}

func DateToTimestamp(t time.Time) int64 {
	return t.Unix()
}
