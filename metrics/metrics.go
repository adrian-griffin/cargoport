package metrics

import (
	"github.com/adrian-griffin/cargoport/util"
)

// declare metrics struct
type Metrics struct {
	LastRunSuccess bool
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

// lsat job metrics
type JobMetrics struct {
	LastRunSuccess bool    `json:"last_run_success"`
	LastBackupSize int64   `json:"last_backup_size_bytes"`
	LastDuration   float64 `json:"last_duration_seconds"`
	//LastRunTime    int64   `json:"last_run_unix"`
	//JobCount       int64   `json:"job_count_total"`
}

// environment & general info metrics
type EnvMetrics struct {
	LocalDirSize   int64 `json:"local_dir_size_bytes"`
	RemoteDirSize  int64 `json:"remote_dir_size_bytes"`
	LocalFileCount int   `json:"local_backup_filecount"`
}

// set last job metrics values after run.
func (m *JobMetrics) SetLastJobMetrics(success bool, size int64, duration float64) {
	if success {
		jobSuccess.Set(1)
		m.LastRunSuccess = true
	} else {
		jobSuccess.Set(0)
		m.LastRunSuccess = false
	}
	backupSize.Set(float64(size))
	jobDuration.Set(duration)

	m.LastBackupSize = size
	m.LastDuration = duration
}

func (m *EnvMetrics) SetLocalDirSize(path string) {
	if size, err := util.GetDirectorySize(path); err == nil {
		localDirSize.Set(float64(size))
		m.LocalDirSize = size
	}
}

func (m *EnvMetrics) SetRemoteDirSize(path string) {
	if size, err := util.GetDirectorySize(path); err == nil {
		remoteDirSize.Set(float64(size))
		m.RemoteDirSize = size
	}
}

func (m *EnvMetrics) SetLocalFileCount(path string) {
	if count, err := util.GetTarballCount(path); err == nil {
		localFileCount.Set(float64(count))
		m.LocalFileCount = count
	}
}

// create new metrics struct and pass pointer
func NewJobMetrics() *JobMetrics {
	return &JobMetrics{}
}

// create new metrics struct and pass pointer
func NewEnvMetrics() *EnvMetrics {
	return &EnvMetrics{}
}
