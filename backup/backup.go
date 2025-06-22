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

// debug level logging output fields for backup package
func backupLogBaseFields(jobctx job.JobContext) map[string]interface{} {
	coreFields := logger.CoreLogFields(&jobctx, "backup")
	fields := logger.MergeFields(coreFields, map[string]interface{}{
		"skip_local": jobctx.SkipLocal,
		"target_dir": jobctx.TargetDir,
		"tag":        jobctx.Tag,
	})
	return fields
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
	if err := ValidateBackupFilePath(filePathString); err != nil {
		return "", fmt.Errorf("backup file path validation failed: %v", err)
	}

	return filePathString, nil
}

// validates local backup path existence & permissions via test writeability
func ValidateBackupFilePath(backupFilePath string) error {

	//<here> NEEDS LOGIC TO VALIDATE DIRECT DIR PERMS

	// firstly validate parent of determined target dir exists
	parentDir := filepath.Dir(backupFilePath)
	if err := util.ValidateDirectoryString(parentDir); err != nil {
		return fmt.Errorf("target backup path validation failed: %v", err)
	}

	// validate that the parentdir is writeable
	if err := util.ValidateDirectoryWriteable(parentDir); err != nil {
		return fmt.Errorf("target directory is not writeable: %v", err)
	}

	return nil
}

// shells out to cli to compresses target directory into output file tarball
func ShellCompressDirectory(jobctx *job.JobContext, targetDir, outputFile string) error {

	// defining logging fields
	verboseFields := backupLogBaseFields(*jobctx)
	// coreFields := logger.CoreLogFields(context, "backup")

	// compress target directory
	logger.LogxWithFields("debug", fmt.Sprintf("Compressing target directory %s to %s", targetDir, outputFile), verboseFields)

	parentDir := filepath.Dir(targetDir)
	baseDir := filepath.Base(targetDir)

	// ensure base dir is valid
	if baseDir == "" || baseDir == "." {
		return fmt.Errorf("invalid directory structure for: %s", targetDir)
	}

	// run tar compression
	err := util.RunCommand(
		"tar",
		"-cvzf",
		outputFile,
		"-C",
		parentDir, // Parent directory
		baseDir,   // Directory to compress
	)
	if err != nil {
		logger.LogxWithFields("error", fmt.Sprintf("Error compressing directory: %s/%s", parentDir, baseDir), map[string]interface{}{
			"package": "backup",
			"target":  baseDir,
		})
		os.Remove(outputFile) // ensure partial file is cleaned up
		return fmt.Errorf("error compressing directory: %v", err)
	}

	// get output file size and return to job context
	fileInfo, err := os.Stat(outputFile)
	if err != nil {
		return fmt.Errorf("error gathering output file info: %v", err)
	}
	jobctx.CompressedSizeBytesInt = fileInfo.Size()
	jobctx.CompressedSizeMBString = fmt.Sprintf("%.2f MB", float64(jobctx.CompressedSizeBytesInt)/1024.0/1024.0)

	// print to cli & log to logfile regarding successful directory compression
	logger.LogxWithFields("debug", fmt.Sprintf("Contents of %s successfully compressed to %s, output filesize: %s", targetDir, outputFile, jobctx.CompressedSizeMBString), logger.MergeFields(verboseFields, map[string]interface{}{
		"size":       jobctx.CompressedSizeMBString,
		"size_bytes": jobctx.CompressedSizeBytesInt,
	}))

	// basic info output
	logger.LogxWithFields("info", "Successfully compressed target data", map[string]interface{}{
		"package":    "backup",
		"docker":     jobctx.Docker,
		"target":     jobctx.Target,
		"target_dir": jobctx.TargetDir,
		"job_id":     jobctx.JobID,
		"tag":        jobctx.Tag,
		"size":       jobctx.CompressedSizeMBString,
	})

	return nil
}
