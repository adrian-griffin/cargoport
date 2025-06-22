package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus metric declarations.
var jobSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "cargoport_last_run_success",
	Help: "Last job run success status (1=success, 0=failure)",
})

var backupSize = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "cargoport_last_backup_size_bytes",
	Help: "Size of last backup in bytes",
})

var jobDuration = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "cargoport_last_job_duration_seconds",
	Help: "Duration of last job run in seconds",
})

// Register metrics on package initialization.
func init() {
	prometheus.MustRegister(jobSuccess, backupSize, jobDuration)
}

// SetMetrics sets metric values after a cargoport run.
func (m *Metrics) SetMetrics(success bool, size int64, duration float64) {
	if success {
		jobSuccess.Set(1)
		m.LastRunSuccess = 1
	} else {
		jobSuccess.Set(0)
		m.LastRunSuccess = 0
	}
	backupSize.Set(float64(size))
	jobDuration.Set(duration)

	m.LastBackupSize = size
	m.LastDuration = duration
}

// open metrics endpoint for duration
func StartMetricsServer(addr string, duration time.Duration) {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		http.ListenAndServe(addr, nil)
	}()
	time.Sleep(duration)
}
