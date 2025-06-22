package metrics

// declare metrics struct
type Metrics struct {
	LastRunSuccess float64
	LastBackupSize int64
	LastDuration   float64
	LocalDirSize   int64
	RemoteDirSize  int64
	LocalFileCount int
}

// create new metrics struct and pass pointer
func NewMetrics() *Metrics {
	return &Metrics{}
}
