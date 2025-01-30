package backup

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrian-griffin/cargoport/docker"
	"github.com/adrian-griffin/cargoport/sysutil"
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
		log.Printf("Error compressing directory: %s/%s", parentDir, baseDir)
		os.Remove(outputFile) // ensure partial file is cleaned up
		return fmt.Errorf("error compressing directory: %v", err)
	}

	// print to cli & log to logfile regarding successful directory compression
	log.Printf("Contents of %s successfully compressed to %s",
		targetDir,
		outputFile,
	)
	fmt.Printf("Contents of %s successfully compressed to %s\n",
		targetDir,
		outputFile,
	)
	return nil
}
