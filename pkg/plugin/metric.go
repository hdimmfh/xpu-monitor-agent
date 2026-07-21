package plugin

import "time"

type Metric struct {
	DeviceID  string    `json:"device_id"`
	Name      string    `json:"name"`
	Value     float64   `json:"value"`
	Unit      string    `json:"unit"`
	Timestamp time.Time `json:"timestamp"`
}
