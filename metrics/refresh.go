package metrics

func ApplyPrometheusMetrics(job JobMetrics, env EnvMetrics) {
	if job.LastRunSuccess {
		jobSuccess.Set(1)
	} else {
		jobSuccess.Set(0)
	}
	backupSize.Set(float64(job.LastBackupSize))
	jobDuration.Set(job.LastDuration)

	localDirSize.Set(float64(env.LocalDirSize))
	remoteDirSize.Set(float64(env.RemoteDirSize))
	localFileCount.Set(float64(env.LocalFileCount))
}
