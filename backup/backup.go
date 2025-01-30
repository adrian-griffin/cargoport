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

	basePath := filepath.Dir(targetDir)
	//baseName := filepath.Base(targetDir)
	// E.g., /some/path, and targetDir = /some/path/MyDir => we’ll store
	// files as “MyDir/subfolder/file.txt” in the archive.

	// Walk the directory recursively
	walkPath := targetDir
	err = filepath.Walk(walkPath, func(filePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Build the name to store in the tar header. We want a relative path
		// from the directory’s parent. e.g. “MyDir/...” or “baseDir/...”.
		relPath, err := filepath.Rel(basePath, filePath)
		if err != nil {
			return err
		}

		// If it's a directory, just write a header with no file contents.
		// (Tar archives do need explicit directory entries sometimes.)
		if info.IsDir() {
			// The root directory always isIsDir, so we can return nil to skip
			// writing a header for the top-level if desired. But let's proceed
			// so that it mirrors typical `tar` behavior.
			hdr := &tar.Header{
				Name:     relPath + "/",
				Mode:     int64(info.Mode()),
				Typeflag: tar.TypeDir,
				ModTime:  info.ModTime(),
			}
			return tarWriter.WriteHeader(hdr)
		}

		// Otherwise, it's a file (or symlink).
		// Create a tar header based on file info.
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		// Overwrite the Name field to store the correct relative path in the tar.
		header.Name = relPath

		// Write the header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// If it's a regular file, copy its contents
		if info.Mode().IsRegular() {
			file, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer file.Close()

			// Copy the file data into the tar
			_, err = io.Copy(tarWriter, file)
			if err != nil {
				return err
			}
		}

		// If it's a symlink, the `FileInfoHeader` call above would store the link target in header.Linkname.
		// Tar won't automatically resolve or embed the link's target data, it just records the symlink info.

		return nil
	})
	if err != nil {
		// If any error happened during filepath.Walk or tar writing.
		os.Remove(outputFile) // Clean up partial file
		return fmt.Errorf("failed while building tarball: %v", err)
	}

	// Optionally flush and close writers
	// (defer calls do it, but we can do an explicit flush if we want).
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
