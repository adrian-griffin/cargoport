package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

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

// compresses target directory into output file tarball usin Go
func GoCompressDirectory(jobctx *job.JobContext, targetDir, outputFile string) error {

	// defining logging fields
	verboseFields := backupLogBaseFields(*jobctx)
	// coreFields := logger.CoreLogFields(context, "backup")

	// compress target directory
	logger.LogxWithFields("debug", fmt.Sprintf("Compressing target directory %s to %s", targetDir, outputFile), verboseFields)

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
	logger.LogxWithFields("debug", fmt.Sprintf("Contents of %s successfully compressed to %s", targetDir, outputFile), verboseFields)

	// basic info output
	logger.LogxWithFields("info", "Successfully compressed target data", map[string]interface{}{
		"package":    "backup",
		"skip_local": jobctx.SkipLocal,
		"target":     jobctx.Target,
		"target_dir": jobctx.TargetDir,
		"job_id":     jobctx.JobID,
		"tag":        jobctx.Tag,
	})
	return nil
}
