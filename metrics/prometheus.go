package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// prom metric declarations
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

// register metrics via prom
func init() {
	prometheus.MustRegister(
		jobSuccess, backupSize, jobDuration, // last job vars
		localDirSize, remoteDirSize, localFileCount, // environment vars
	)
}

// open metrics endpoint for duration
func StartMetricsServer(addr string, duration time.Duration) {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		http.ListenAndServe(addr, nil)
	}()
	time.Sleep(duration)
}
