// pkg/plugin/metric.go
package plugin

import "time"

type Metric struct {
	Name      string
	Value     any
	Unit      string
	Timestamp time.Time
}
