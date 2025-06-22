package runner

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/adrian-griffin/cargoport/backup"
	"github.com/adrian-griffin/cargoport/input"
	"github.com/adrian-griffin/cargoport/job"
	"github.com/adrian-griffin/cargoport/logger"
	"github.com/adrian-griffin/cargoport/meta"
	"github.com/adrian-griffin/cargoport/util"
)

// debug level logging output fields for main package
func jobhandlerLogDebugFields(context *job.JobContext) map[string]interface{} {
	coreFields := logger.CoreLogFields(context, "jobhandler")
	fields := logger.MergeFields(coreFields, map[string]interface{}{
		"remote":         context.Remote,
		"docker":         context.Docker,
		"skip_local":     context.SkipLocal,
		"target_dir":     context.TargetDir,
		"tag":            context.Tag,
		"restart_docker": context.RestartDocker,
		"remote_host":    context.RemoteHost,
		"remote_user":    context.RemoteUser,
	})

	return fields
}

func RunJob(inputctx *input.InputContext) (duration float64, size int64, err error) {
	// generate job ID & populate jobcontext
	jobID := job.GenerateJobID()

	jobCTX := job.JobContext{
		Target:                 "",
		Remote:                 (inputctx.RemoteHost != ""),
		Docker:                 false,
		SkipLocal:              inputctx.SkipLocal,
		JobID:                  jobID,
		StartTime:              time.Now(), // begin timer now
		TargetDir:              "",
		RootDir:                inputctx.DefaultOutputDir,
		Tag:                    inputctx.Tag,
		RestartDocker:          inputctx.RestartDocker,
		RemoteHost:             string(inputctx.RemoteHost),
		RemoteUser:             string(inputctx.RemoteUser),
		CompressedSizeBytesInt: 0,
		CompressedSizeMBString: "0.0 MB",
	}

	// log & print job start
	logger.LogxWithFields("info", " --------------------------------------------------- ", map[string]interface{}{
		"package": "spacer",
		"job_id":  jobCTX.JobID,
	})

	// resolve target dir intended for backup
	composeFilePath, outputFilePath, err := backup.ResolveTarget(inputctx, &jobCTX)
	if err != nil {
		return 0, 0, fmt.Errorf("error determining intended backup target: %v", err)
	}

	// define jobhandler logging
	coreFields := logger.CoreLogFields(&jobCTX, "jobhandler")
	verboseFields := jobhandlerLogDebugFields(&jobCTX)

	logger.LogxWithFields("info", "New backup job added", map[string]interface{}{
		"package": "jobhandler",
		"target":  jobCTX.Target,
		"remote":  jobCTX.Remote,
		"docker":  jobCTX.Docker,
		"job_id":  jobCTX.JobID,
		"tag":     jobCTX.Tag,
		"version": meta.Version,
	})

	// declare target base name for metrics and logging tracking
	targetBaseName := filepath.Base(jobCTX.TargetDir)

	logger.LogxWithFields("debug", fmt.Sprintf("Beginning backup job via %s", jobCTX.TargetDir), verboseFields)

	// handle pre-backup docker tasks
	if jobCTX.Docker {
		if err := backup.HandleDockerPreBackup(&jobCTX, composeFilePath, targetBaseName); err != nil {
			logger.LogxWithFields("error", fmt.Sprintf("error performing pre-snapshot docker tasks: %v", err), coreFields)
			return 0, 0, err
		}
	}

	// attempt compression of data; if fail && dockerEnabled then attempt to handle docker restart
	if err := backup.ShellCompressDirectory(&jobCTX, jobCTX.TargetDir, outputFilePath); err != nil {

		// if docker restart fails, log error
		if jobCTX.Docker {
			if dockererr := backup.HandleDockerPostBackup(&jobCTX, composeFilePath, jobCTX.RestartDocker); dockererr != nil {
				logger.LogxWithFields("error", fmt.Sprintf("error handling docker compose after backup: %v", dockererr), coreFields)
				return 0, 0, err
			}
		}

		logger.LogxWithFields("error", fmt.Sprintf("error compressing target: %v", err), coreFields)
		return 0, 0, err
	}

	// handle remote transfer
	if inputctx.RemoteHost != "" {
		err := backup.HandleRemoteTransfer(&jobCTX, outputFilePath, inputctx)
		if err != nil {
			// if remote fail, then remove tempfile when skipLocal enabled
			if jobCTX.SkipLocal {
				util.RemoveTempFile(&jobCTX, outputFilePath)
				logger.LogxWithFields("debug", fmt.Sprintf("Removing local tempfile %s", outputFilePath), verboseFields)

			}

			// if remote fail, then handle post-backup docker jobs
			if jobCTX.Docker {
				if err := backup.HandleDockerPostBackup(&jobCTX, composeFilePath, jobCTX.RestartDocker); err != nil {
					logger.LogxWithFields("error", fmt.Sprintf("error reinitializing docker service after failed transfer: %v", err), coreFields)
					return 0, 0, err
				}
			}
			logger.LogxWithFields("error", fmt.Sprintf("error completing remote transfer: %v", err), verboseFields)
			return 0, 0, err
		}
	}

	// handle docker post backup
	if jobCTX.Docker {
		if err := backup.HandleDockerPostBackup(&jobCTX, composeFilePath, jobCTX.RestartDocker); err != nil {
			logger.LogxWithFields("error", fmt.Sprintf("error restarting docker service: %v", err), coreFields)
			return 0, 0, err
		}
	}

	// job completion banner & time calculation
	jobDuration := time.Since(jobCTX.StartTime)
	executionSeconds := jobDuration.Seconds()

	logger.LogxWithFields("info", fmt.Sprintf("Job success, execution time: %.2fs", executionSeconds), map[string]interface{}{
		"package":  "jobhandler",
		"target":   jobCTX.Target,
		"remote":   jobCTX.Remote,
		"docker":   jobCTX.Docker,
		"job_id":   jobCTX.JobID,
		"duration": fmt.Sprintf("%.2fs", executionSeconds),
		"success":  true,
		"size":     jobCTX.CompressedSizeMBString,
	})
	logger.LogxWithFields("info", " --------------------------------------------------- ", map[string]interface{}{
		"package":    "spacer",
		"end_job_id": jobCTX.JobID,
	})

	return executionSeconds, jobCTX.CompressedSizeBytesInt, nil

}
