package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrian-griffin/cargoport/docker"
)

// determines target dir for backup based on input user input
func DetermineBackupTarget(targetDir, dockerName *string) (string, string, bool) {
	var composeFilePath string
	dockerEnabled := false

	// validates composefile, returns its path and dirpath, and enables dockerMode
	if *dockerName != "" {
		var err error
		composeFilePath, err = docker.FindComposeFile(*dockerName)
		if err != nil {
			log.Fatalf("ERROR <storage>: %v", err)
		}
		//log.Println("<DEBUG>: TARGET DOCKER FOUND")
		return filepath.Dir(composeFilePath), composeFilePath, true
	}
	// validates target dir and returns it, keeps dockerMode disabled
	if *targetDir != "" {
		targetDirectory := strings.TrimSuffix(*targetDir, "/")
		if stat, err := os.Stat(targetDirectory); err != nil || !stat.IsDir() {
			log.Fatalf("ERROR <storage>: Invalid target directory: %s", targetDirectory)
		}
		// tries to determine composefile
		possibleComposeFile := filepath.Join(targetDirectory, "docker-compose.yml")
		if _, err := os.Stat(possibleComposeFile); err == nil {
			//log.Println("<DEBUG>: DOCKER COMPOSE FILE FOUND IN TARGET DIR")
			return targetDirectory, possibleComposeFile, true
		}

		//log.Println("<DEBUG>: NO DOCKER COMPOSE FILE FOUND, TREATING AS REGULAR DIR")
		return targetDirectory, "", false
	}

	log.Fatalf("ERROR <storage>: No valid target directory or Docker service specified")
	return "", "", dockerEnabled
}

// determines path for new backupfile based on user input
func PrepareBackupFilePath(localBackupDir, targetDir, customOutputDir string, skipLocal bool) string {
	// sanitize target directory
	targetDir = strings.TrimSuffix(targetDir, "/")
	baseName := filepath.Base(targetDir)

	// if baseName is empty, use backup name
	if baseName == "" || baseName == "." || baseName == ".." {
		log.Printf("WARN <storage>: Invalid target directory name '%s', saving backup as 'unnamed-backup.bak.tar.gz'", targetDir)
		baseName = "unnamed-backup"
	}

	// if a custom local output directory is provided
	if customOutputDir != "" {
		return filepath.Join(customOutputDir, baseName+".bak.tar.gz")
	}

	// use os temp dir if skipLocal
	if skipLocal {
		return filepath.Join(os.TempDir(), baseName+".bak.tar.gz")
	}

	// default to the localBackupDir path defined in conf
	return filepath.Join(localBackupDir, baseName+".bak.tar.gz")
}

// compresses target directory into output file tarball
func CompressDirectory(targetDir, outputFile string) error {
	// compress target directory
	fmt.Println("-------------------------------------------------------------------------")
	fmt.Println("Compressing container directory . . .")
	fmt.Println("-------------------------------------------------------------------------")

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
	log.Printf("Contents of %s successfully compressed to %s", targetDir, outputFile)
	fmt.Printf("Contents of %s successfully compressed to %s\n", targetDir, outputFile)
	return nil
}
