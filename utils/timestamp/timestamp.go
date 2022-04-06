package timestamp

import "time"

func Now() *Timestamp {
	now := time.Now()
	return &Timestamp{
		Seconds: now.Unix(),
		Nanos:   now.UnixNano(),
	}
}

func (m *Timestamp) Format(layout string) string {
	t := time.Unix(m.Seconds, m.Nanos)
	return t.Format(layout)
}
