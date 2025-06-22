package metrics

import (
	"net/http"
	"time"

	"github.com/adrian-griffin/cargoport/util"
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

var localDirSize = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "cargoport_local_backup_dir_bytes",
	Help: "Total size in bytes of the local backup directory",
})

var remoteDirSize = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "cargoport_remote_backup_dir_bytes",
	Help: "Total size in bytes of the remote backup directory",
})

var localFileCount = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "cargoport_local_backup_dir_filecount",
	Help: "Number of files in the local backup directory",
})

// Register metrics on package initialization.
func init() {
	prometheus.MustRegister(jobSuccess, backupSize, jobDuration, localDirSize, remoteDirSize, localFileCount)
}

// SetMetrics sets metric values after a cargoport run.
func (m *Metrics) SetBaseMetrics(success bool, size int64, duration float64) {
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

func (m *Metrics) SetLocalDirSize(path string) {
	if size, err := util.GetDirectorySize(path); err == nil {
		localDirSize.Set(float64(size))
		m.LocalDirSize = size
	}
}

func (m *Metrics) SetRemoteDirSize(path string) {
	if size, err := util.GetDirectorySize(path); err == nil {
		remoteDirSize.Set(float64(size))
		m.RemoteDirSize = size
	}
}

func (m *Metrics) SetLocalFileCount(path string) {
	if count, err := util.GetTarballCount(path); err == nil {
		localFileCount.Set(float64(count))
		m.LocalFileCount = count
	}
}

// open metrics endpoint for duration
func StartMetricsServer(addr string, duration time.Duration) {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		http.ListenAndServe(addr, nil)
	}()
	time.Sleep(duration)
}
