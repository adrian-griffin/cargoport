package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrian-griffin/cargoport/input"
)

// reads metrics from jsonfile
func ReadJSONMetrics(config *input.ConfigFile) (JobMetrics, EnvMetrics, error) {
	var job JobMetrics
	var env EnvMetrics

	jobPath := filepath.Join(config.MetricsDir, "last_job_metrics.json")
	envPath := filepath.Join(config.MetricsDir, "environment_metrics.json")

	jobFile, err := os.ReadFile(jobPath)
	if err != nil {
		return job, env, fmt.Errorf("reading job metrics: %w", err)
	}
	if err := json.Unmarshal(jobFile, &job); err != nil {
		return job, env, fmt.Errorf("parsing job metrics: %w", err)
	}

	envFile, err := os.ReadFile(envPath)
	if err != nil {
		return job, env, fmt.Errorf("reading env metrics: %w", err)
	}
	if err := json.Unmarshal(envFile, &env); err != nil {
		return job, env, fmt.Errorf("parsing env metrics: %w", err)
	}

	return job, env, nil
}

func writeAtomicJSON(metricsFilePath string, data any) error {
	tmpFilePath := metricsFilePath + ".tmp"
	f, err := os.Create(tmpFilePath)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return os.Rename(tmpFilePath, metricsFilePath)
}

func WriteMetricsFiles(config *input.ConfigFile, jobMetrics JobMetrics, envMetrics EnvMetrics) error {

	metricsDir := config.MetricsDir
	jobPath := filepath.Join(metricsDir, "last_job_metrics.json")
	envPath := filepath.Join(metricsDir, "environment_metrics.json")

	if err := writeAtomicJSON(jobPath, jobMetrics); err != nil {
		return fmt.Errorf("writing job metrics: %w", err)
	}
	if err := writeAtomicJSON(envPath, envMetrics); err != nil {
		return fmt.Errorf("writing env metrics: %w", err)
	}
	return nil
}

// metrics/cache.go
func LoadFromCacheAndExpose(config *input.ConfigFile) error {
	job, env, err := ReadJSONMetrics(config)
	if err != nil {
		return err
	}

	jobSuccess.Set(boolToFloat(job.LastRunSuccess))
	backupSize.Set(float64(job.LastBackupSize))
	jobDuration.Set(job.LastDuration)

	localDirSize.Set(float64(env.LocalDirSize))
	remoteDirSize.Set(float64(env.RemoteDirSize))
	localFileCount.Set(float64(env.LocalFileCount))

	return nil
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
