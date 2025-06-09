package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrian-griffin/cargoport/docker"
	"github.com/adrian-griffin/cargoport/jobcontext"
	"github.com/adrian-griffin/cargoport/logger"
	"github.com/adrian-griffin/cargoport/sysutil"
)

// determines target dir for backup based on input user input
func DetermineBackupTarget(targetDir, dockerName *string) (string, string, bool, error) {
	var composeFilePath string
	dockerEnabled := false

	// validates composefile, returns its path and dirpath, and enables dockerMode
	if *dockerName != "" {
		var err error
		composeFilePath, err = docker.FindComposeFile(*dockerName, filepath.Base(*targetDir))
		if err != nil {
			logger.LogxWithFields("error", fmt.Sprintf("Compose file validation failure at %s", *targetDir), map[string]interface{}{
				"package": "backup",
				"target":  filepath.Base(*targetDir),
				"success": false,
			})
			return "", "", false, fmt.Errorf("failed to retrieve composefile path: %v", err)
		}
		//("<DEBUG>: TARGET DOCKER FOUND")
		return filepath.Dir(composeFilePath), composeFilePath, true, nil
	}
	// validates target dir and returns it, keeps dockerMode disabled
	if *targetDir != "" {
		targetDirectory := strings.TrimSuffix(*targetDir, "/")
		if stat, err := os.Stat(targetDirectory); err != nil || !stat.IsDir() {
			logger.LogxWithFields("error", fmt.Sprintf("Invalid target directory: %s", targetDirectory), map[string]interface{}{
				"package": "backup",
				"target":  filepath.Base(targetDirectory),
				"success": false,
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
			})
			return targetDirectory, possibleComposeFile, true, nil
		}

		logger.LogxWithFields("debug", fmt.Sprintf("Compose file not found in target dir at %s", filepath.Join(targetDirectory, "docker-compose.yml")), map[string]interface{}{
			"package": "backup",
			"target":  filepath.Base(targetDirectory),
		})
		logger.LogxWithFields("debug", "Treating as regular dir backup and skipping docker jobs", map[string]interface{}{
			"package": "backup",
			"target":  filepath.Base(targetDirectory),
		})
		return targetDirectory, "", false, nil
	}

	logger.LogxWithFields("error", "Invalid -target-dir or -docker-name passed", map[string]interface{}{
		"package": "backup",
		"target":  filepath.Base(filepath.Dir(composeFilePath)),
	})
	return "", "", dockerEnabled, fmt.Errorf("no valid target directory or Docker service specified")
}

// determines path for new backupfile based on user input
func PrepareBackupFilePath(localBackupDir, targetDir, customOutputDir, tagOutputString string, skipLocal bool) (string, error) {
	// sanitize target directory suffix
	targetDir = strings.TrimSuffix(targetDir, "/")
	baseName := filepath.Base(targetDir)

	// if output file tag is not emty, prepend it with a `-`
	if tagOutputString != "" {
		tagOutputString = "-" + tagOutputString
	}

	// validate that targetDir exists prior to continuing
	if targetDirInfo, targetDirErr := os.Stat(targetDir); targetDirErr != nil || !targetDirInfo.IsDir() {
		return "", fmt.Errorf("target directory %s does not exist or is not a directory: %v", targetDir, targetDirErr)
	}

	// if baseName is empty, use backup name
	if baseName == "" || baseName == "." || baseName == ".." {
		logger.LogxWithFields("warn", fmt.Sprintf("Invalid target directory name '%s', saving backup as 'unnamed-backup.bak.tar.gz'", targetDir), map[string]interface{}{
			"package":    "backup",
			"target":     filepath.Base(baseName),
			"output_dir": targetDir,
			"tag":        tagOutputString,
		})
		baseName = "unnamed-backup"
	}

	backupFileName := baseName + tagOutputString + ".bak.tar.gz"
	var filePathString string

	switch {
	case customOutputDir != "": // if custom output dir is not empty, make custom dir the target
		filePathString = filepath.Join(customOutputDir, backupFileName)
	case skipLocal: // if skip local enabled, create tempfile in the os's temp dir
		filePathString = filepath.Join(os.TempDir(), backupFileName)
	default: // otherwise use default cargoport local dir
		filePathString = filepath.Join(localBackupDir, backupFileName)
	}

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
	if err := sysutil.ValidateDirectoryString(parentDir); err != nil {
		return fmt.Errorf("target backup path validation failed: %v", err)
	}

	// validate that the parentdir is writeable
	if err := sysutil.ValidateDirectoryWriteable(parentDir); err != nil {
		return fmt.Errorf("target directory is not writeable: %v", err)
	}

	return nil
}

// compresses target directory into output file tarball usin Go
func CompressDirectory(context jobcontext.JobContext, targetDir, outputFile string) error {
	// compress target directory
	logger.LogxWithFields("debug", fmt.Sprintf("Compressing target directory %s to %s", targetDir, outputFile), map[string]interface{}{
		"package": "backup",
		"target":  filepath.Base(targetDir),
	})

	// ensure base dir is valid
	fi, err := os.Stat(targetDir)
	if err != nil {
		return fmt.Errorf("invalid target directory %s: %v", targetDir, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("path %s is not a directory", targetDir)
	}

	// create output file
	out, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create tarball file %s: %v", outputFile, err)
	}
	defer out.Close()

	// wrap outputfile with gzip writer
	gzWriter := gzip.NewWriter(out)
	defer gzWriter.Close()

	// create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// store directory to create relative file names
	basePath := filepath.Dir(targetDir)

	// walk the directory recursively
	walkPath := targetDir
	err = filepath.Walk(walkPath, func(filePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// build relative path to root for directory structure
		relPath, err := filepath.Rel(basePath, filePath)
		if err != nil {
			return err
		}

		// if dir, only write a header with no file contents
		if info.IsDir() {
			hdr := &tar.Header{
				Name:     relPath + "/",
				Mode:     int64(info.Mode()),
				Typeflag: tar.TypeDir,
				ModTime:  info.ModTime(),
			}
			return tarWriter.WriteHeader(hdr)
		}
		// otherwise, it's a file or symlink
		// create a new tar header based on file info
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		// insert relative path for Name field
		header.Name = relPath

		// write the header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// if it's a regular file, copy its contents
		if info.Mode().IsRegular() {
			file, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer file.Close()

			// copy the file data into the tar
			_, err = io.Copy(tarWriter, file)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		// if error during walk or tar writing, clean partial file
		os.Remove(outputFile)
		return fmt.Errorf("failed while building tarball: %v", err)
	}

	// force flush and close writers
	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("tar writer close error: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		return fmt.Errorf("gzip writer close error: %v", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("output file close error: %v", err)
	}

	// print to cli & log to logfile regarding successful directory compression
	logger.LogxWithFields("debug", fmt.Sprintf("Contents of %s successfully compressed to %s", targetDir, outputFile), map[string]interface{}{
		"package": "backup",
		"target":  filepath.Base(targetDir),
	})

	// basic info output
	logger.LogxWithFields("info", "Successfully compressed target data", map[string]interface{}{
		"package":    "backup",
		"skip_local": context.SkipLocal,
		"target":     context.Target,
		"target_dir": context.TargetDir,
		"job_id":     context.JobID,
		"tag":        context.Tag,
	})
	return nil
}

// shells out to cli to compresses target directory into output file tarball
func ShellCompressDirectory(context jobcontext.JobContext, targetDir, outputFile string) error {
	// compress target directory
	logger.LogxWithFields("debug", fmt.Sprintf("Compressing target directory %s to %s", targetDir, outputFile), map[string]interface{}{
		"package": "backup",
		"target":  filepath.Base(targetDir),
	})

	parentDir := filepath.Dir(targetDir)
	baseDir := filepath.Base(targetDir)

	// ensure base dir is valid
	if baseDir == "" || baseDir == "." {
		return fmt.Errorf("invalid directory structure for: %s", targetDir)
	}

	// run tar compression
	err := sysutil.RunCommand(
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
	compressedSizeBytes := fileInfo.Size()
	context.CompressedSizeMB = float64(compressedSizeBytes) / 1024.0 / 1024.0

	// print to cli & log to logfile regarding successful directory compression
	logger.LogxWithFields("debug", fmt.Sprintf("Contents of %s successfully compressed to %s, output filesize %.2f", targetDir, outputFile, float64(context.CompressedSizeMB)/1024.0/1024.0), map[string]interface{}{
		"package":    "backup",
		"skip_local": context.SkipLocal,
		"target":     context.Target,
		"target_dir": context.TargetDir,
		"job_id":     context.JobID,
		"tag":        context.Tag,
		"size":       context.CompressedSizeMB,
		"remote":     context.Remote,
		"docker":     context.Docker,
	})

	// basic info output
	logger.LogxWithFields("info", "Successfully compressed target data", map[string]interface{}{
		"package":    "backup",
		"target":     context.Target,
		"target_dir": context.TargetDir,
		"job_id":     context.JobID,
		"tag":        context.Tag,
		"size":       context.CompressedSizeMB,
	})

	return nil
}
