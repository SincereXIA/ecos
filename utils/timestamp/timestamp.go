package timestamp

import "time"

func Now() *Timestamp {
	now := time.Now()
	return &Timestamp{
		Seconds: int64(now.Second()),
		Nanos:   int32(now.Nanosecond()),
	}
}
