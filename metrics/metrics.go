package metrics

// declare metrics struct
type Metrics struct {
	LastRunSuccess float64
	LastBackupSize int64
	LastDuration   float64
}

// create new metrics struct and pass pointer
func NewMetrics() *Metrics {
	return &Metrics{}
}
