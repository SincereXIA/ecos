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
	t := m.GetTime()
	return t.Format(layout)
}

func (m *Timestamp) GetTime() time.Time {
	return time.Unix(0, m.Nanos)
}
