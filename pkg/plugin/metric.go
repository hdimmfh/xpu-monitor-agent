package plugin

import "time"

type Metric struct {
	DeviceID  string
	Name      string
	Value     any
	Unit      string
	Timestamp time.Time
}
