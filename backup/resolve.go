package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrian-griffin/cargoport/input"
	"github.com/adrian-griffin/cargoport/job"
	"github.com/adrian-griffin/cargoport/logger"
	"github.com/adrian-griffin/cargoport/util"
)

// resolve target dir to back up, returns composefile path & outputfile path
func ResolveTarget(inputctx *input.InputContext, jobctx *job.JobContext) (string, string, error) {
	// determine backup target
	targetPath, composeFilePath, dockerEnabled, err := DetermineBackupTarget(jobctx, inputctx)
	if err != nil {
		logger.LogxWithFields("error", fmt.Sprintf("error determining backup target: %v", err), map[string]interface{}{
			"package": "main",
			"target":  filepath.Base(filepath.Dir(inputctx.TargetDir)),
			"success": false,
			"docker":  true,
		})
		return "", "", err
	}
	jobctx.Docker = dockerEnabled
	jobctx.Target = filepath.Base(targetPath)
	jobctx.TargetDir = targetPath

	// prepare local backupfile & compose
	outputFilePath, err := PrepareBackupFilePath(jobctx, inputctx)
	if err != nil {
		logger.LogxWithFields("error", fmt.Sprintf("error determining output path: %v", err), map[string]interface{}{
			"package":  "main",
			"target":   filepath.Base(targetPath),
			"root_dir": inputctx.DefaultOutputDir,
		})
		return "", "", err
	}

	return composeFilePath, outputFilePath, nil
}

// determines target dir for backup based on input user input
func DetermineBackupTarget(jobctx *job.JobContext, inputcxt *input.InputContext) (string, string, bool, error) {
	var composeFilePath string
	dockerEnabled := false

	// validates composefile, returns its path and dirpath, and enables dockerMode
	if inputcxt.DockerName != "" {
		var err error
		composeFilePath, err = FindComposeFile(inputcxt.DockerName, filepath.Base(inputcxt.TargetDir))
		if err != nil {
			logger.LogxWithFields("error", fmt.Sprintf("Compose file validation failure at %s", inputcxt.TargetDir), map[string]interface{}{
				"package": "backup",
				"target":  filepath.Base(inputcxt.TargetDir),
				"job_id":  jobctx.JobID,
			})
			return "", "", false, fmt.Errorf("failed to retrieve composefile path: %v", err)
		}
		//("<DEBUG>: TARGET DOCKER FOUND")
		return filepath.Dir(composeFilePath), composeFilePath, true, nil
	}
	// validates target dir and returns it, keeps dockerMode disabled
	if inputcxt.TargetDir != "" {
		targetDirectory := strings.TrimSuffix(inputcxt.TargetDir, "/")
		if stat, err := os.Stat(targetDirectory); err != nil || !stat.IsDir() {
			logger.LogxWithFields("error", fmt.Sprintf("Invalid target directory: %s", targetDirectory), map[string]interface{}{
				"package": "backup",
				"target":  filepath.Base(targetDirectory),
				"job_id":  jobctx.JobID,
			})
			return "", "", false, fmt.Errorf("failed to check target directory: %v", err)
		}

		// tries to determine composefile
		possibleComposeFile := filepath.Join(targetDirectory, "docker-compose.yml")
		_, err := os.Stat(possibleComposeFile)
		if err == nil {
			logger.LogxWithFields("debug", fmt.Sprintf("Compose file found in target dir at %s", filepath.Join(targetDirectory, "docker-compose.yml")), map[string]interface{}{
				"package": "backup",
				"target":  filepath.Base(targetDirectory),
				"job_id":  jobctx.JobID,
			})
			return targetDirectory, possibleComposeFile, true, nil
		}

		logger.LogxWithFields("debug", fmt.Sprintf("Compose file not found in target dir at %s", filepath.Join(targetDirectory, "docker-compose.yml")), map[string]interface{}{
			"package": "backup",
			"target":  filepath.Base(targetDirectory),
			"job_id":  jobctx.JobID,
		})
		logger.LogxWithFields("debug", "Treating as regular dir backup and skipping docker jobs", map[string]interface{}{
			"package": "backup",
			"target":  filepath.Base(targetDirectory),
			"job_id":  jobctx.JobID,
		})
		return targetDirectory, "", false, nil
	}

	logger.LogxWithFields("error", "Invalid -target-dir or -docker-name passed", map[string]interface{}{
		"package": "backup",
		"target":  filepath.Base(filepath.Dir(composeFilePath)),
		"job_id":  jobctx.JobID,
	})
	return "", "", dockerEnabled, fmt.Errorf("no valid target directory or Docker service specified")
}

// determines path for new backupfile based on user input
func PrepareBackupFilePath(jobctx *job.JobContext, inputctx *input.InputContext) (string, error) {

	// defining logging fields
	verboseFields := backupLogBaseFields(*jobctx)
	// coreFields := logger.CoreLogFields(context, "backup")

	// sanitize target directory suffix
	targetDir := strings.TrimSuffix(inputctx.TargetDir, "/")
	baseName := filepath.Base(targetDir)

	var tagOutputString = ""

	// if output file tag is not emty, prepend it with a `-`
	if inputctx.Tag != "" {
		tagOutputString = "-" + inputctx.Tag
	}

	// validate that targetDir exists prior to continuing
	if targetDirInfo, targetDirErr := os.Stat(targetDir); targetDirErr != nil || !targetDirInfo.IsDir() {
		return "", fmt.Errorf("target directory %s does not exist or is not a directory: %v", targetDir, targetDirErr)
	}

	// if baseName is empty, use backup name
	if baseName == "" || baseName == "." || baseName == ".." {
		logger.LogxWithFields("warn", fmt.Sprintf("Invalid target directory name '%s', saving backup as 'unnamed-backup.bak.tar.gz'", targetDir), verboseFields)
		baseName = "unnamed-backup"
	}

	backupFileName := baseName + tagOutputString + ".bak.tar.gz"

	// form output filepath using input's defined outputdir & filename
	filePathString := filepath.Join(inputctx.OutputDir, backupFileName)

	// validate that files can be written in target output dir
	if err := util.ValidateDirectoryWriteable(filepath.Dir(filePathString)); err != nil {
		return "", fmt.Errorf("backup file path validation failed: %v", err)
	}

	return filePathString, nil
}
