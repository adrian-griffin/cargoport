package main

// Cargoport v0.87.1

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	//"archive/tar"
	//"compress/gzip"
)

const (
	//// IMPORTANT: In order to change the default docker backup & docker compose container root paths, these variables must be changed manually and the executable rebuilt!
	//// Don't forget the trailing `/`

	// Defines the root directory for all docker compose container directories, as well as the directory to store compressed backups
	// all docker containers must be located at `/opt/docker/container1`,`/opt/docker/container2`, etc., change this root path below:
	DefaultTargetRoot = "/opt/docker/"
	DefaultBackupRoot = "/opt/docker-backups/"
	Version           = "v0.87.1"
)

// declare config struct
type Config struct {
	TargetRootPath string
	BackupRootPath string
	TargetName     string
	RemoteUser     string
	RemoteHost     string
	Version        string
	RemoteSend     bool
	DockerEnabled  bool
	SkipLocal      bool
}

// init logging
func initLogging() {
	logFile, err := os.OpenFile("/var/log/cargoport.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("ERROR: Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

// runCommand function, requires command name (docker, tar, etc); accepts multiple arguments
func runCommand(commandName string, args ...string) error {
	cmd := exec.Command(commandName, args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findComposeFile(containerName string) (string, error) {
	cmd := exec.Command("docker", "inspect", containerName, "--format", "{{ .Config.Labels.com.docker.compose.project.working_dir }}")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ERROR: Failed to locate docker compose file for container %s: %v", containerName, err)
	}
	composePath := strings.TrimSpace(string(output))
	return filepath.Join(composePath, "docker-compose.yml"), nil
}

// collects docker image information and digests, stores alongside `docker-compose.yml` file in newly compressed tarball
func getDockerImages(composeFile string, outputFile string) error {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "images", "--quiet")
	output, err := cmd.Output()
	if err != nil {

		return fmt.Errorf("Failed to get docker images: %v", err)
	}

	// loop over image ids to gather docker image digests
	imageLines := string(output)
	imageList := strings.Split(imageLines, "\n")
	var imageInfo string

	for _, imageID := range imageList {
		if imageID == "" {
			continue
		}

		// actually get image digest
		cmdInspect := exec.Command("docker", "inspect", "--format", "{{index .RepoDigests 0}}", imageID)
		digestOutput, err := cmdInspect.Output()
		if err != nil {
			return fmt.Errorf("Failed to inspect docker images: %v", err)
		}
		imageInfo += fmt.Sprintf("Image: %s Digest: %s\n", imageID, digestOutput)
	}

	err = os.WriteFile(outputFile, []byte(imageInfo), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write docker image version info to file: %v", err)
	}

	fmt.Println("Docker image information and digests saved to", outputFile)
	return nil
}

func checkDockerRunState(composeFile string) (bool, error) {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "ps", "--services", "--filter", "status=running")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("Failed to check Docker container status: %v", err)
	}
	runningServices := strings.TrimSpace(string(output))
	if runningServices == "" {
		return false, nil
	}
	return true, nil
}

func main() {

	// begin logging
	initLogging()

	var err error

	// Defines config values
	config := Config{
		TargetRootPath: DefaultTargetRoot,
		BackupRootPath: DefaultBackupRoot,
		Version:        Version,
	}

	// Flag definitions
	targetName := flag.String("target", "", "Name of target directory, not including root path")
	remoteSend := flag.Bool("remote-send", false, "Enable sending backup file to remote machine. Additional flags needed!")
	remoteUser := flag.String("remote-user", "", "Remote machine username")
	remoteHost := flag.String("remote-host", "", "Remote machine IP address")
	remoteFile := flag.String("remote-file", "", "Remote filepath. Defaults to /home/$USER/$TARGETNAME.bak.tar.gz")
	dockerBool := flag.Bool("docker", true, "Docker target? Default: true")
	skipLocal := flag.Bool("skip-local", false, "Skip local backups, only send to remote target (Still requires -remote-send)")
	appVersion := flag.Bool("version", false, "Display app version information")

	// New flags for custom paths
	customSrcRoot := flag.String("src-root", "", "Custom source root path (overrides default set in Config)")
	customDstRoot := flag.String("dst-root", "", "Custom destination root path (overrides default set in Config)")

	flag.Parse()

	// Informational flags processed first
	if *appVersion {
		fmt.Printf("cargoport version: %s", Version)
		os.Exit(0)
	}

	log.Println("-------------------------------------------------------------------------")
	log.Printf("Beginning Backup Job -- cargoport %s", Version)

	// Apply override paths if provided

	if *customSrcRoot != "" {
		config.TargetRootPath = *customSrcRoot
	}

	if *customDstRoot != "" {
		config.BackupRootPath = *customDstRoot
	}

	// Flag validations

	if *targetName == "" {
		fmt.Println("Target must be specified!")
		fmt.Println("Exiting..")
		os.Exit(1)
	}

	if *skipLocal && !*remoteSend {
		fmt.Println("Error: -skip-local requires -remote-send to be set")
		fmt.Println("Exiting ...")
		os.Exit(1)
	}

	sourceDir := filepath.Join(config.TargetRootPath, *targetName)
	backupFile := filepath.Join(config.BackupRootPath, *targetName+".bak.tar.gz")
	imageVersionFile := filepath.Join(sourceDir, "docker-image-versions.txt")

	// Handle docker operations

	if *dockerBool {
		composeFilePath := filepath.Join(sourceDir, "docker-compose.yml")

		// Check that docker-compose.yml file exists
		if _, err := os.Stat(composeFilePath); os.IsNotExist(err) {
			log.Fatalf("docker-compose.yml not found at %s", composeFilePath)
		}
		// Ensure Docker container is running
		running, err := checkDockerRunState(composeFilePath)
		if err != nil {
			log.Fatalf("Error checking Docker container status: %v", err)
		}
		if !running {
			log.Fatalf("FATAL ERROR: Docker container is not running or not locateable, exiting!")
		}

		// Get Docker image information & store it in the working dir
		fmt.Println("-------------------------------------------------------------------------")
		fmt.Println("Getting Docker image versions . . .")
		err = getDockerImages(composeFilePath, imageVersionFile)
		if err != nil {
			log.Fatalf("Error retrieving Docker image versions: %v", err)
		}

		// Stop docker container
		fmt.Println("-------------------------------------------------------------------------")
		fmt.Println("Stopping Docker container . . .")
		fmt.Println("Issuing docker compose down on ", composeFilePath)
		fmt.Println("-------------------------------------------------------------------------")
		err = runCommand("docker", "compose", "-f", composeFilePath, "down")
		if err != nil {
			log.Fatalf("Error stopping Docker container: %v", err)
		}
	}

	// Create temp backup file if skipping local backup
	tempBackupFile := backupFile
	if *skipLocal {
		tempBackupFile = filepath.Join(os.TempDir(), *targetName+".bak.tar.gz")
	}

	// Compress target directory
	fmt.Println("-------------------------------------------------------------------------")
	fmt.Println("Compressing container directory . . .")
	fmt.Println("-------------------------------------------------------------------------")
	err = runCommand("tar", "-cvzf", tempBackupFile, "-C", config.TargetRootPath, *targetName)
	if err != nil {
		log.Fatalf("Error compressing directory: %v", err)
	}
	log.Printf("Contents of %s/%s Successfully Compressed to %s", config.TargetRootPath, *targetName, tempBackupFile)

	if !*skipLocal {
		fmt.Println("-------------------------------------------------------------------------")
		fmt.Println("Backupfile saved at:", backupFile)
	}

	// Handle remote rsync transfer
	if *remoteSend {
		if *remoteUser == "" || *remoteHost == "" {
			log.Fatalf("Remote user and host must be specified when sending to a remote machine.")
		}

		// Set default remote file path to remote user's homedir if none is specified
		remoteFilePath := *remoteFile
		if remoteFilePath == "" {
			remoteFilePath = fmt.Sprintf("/home/%s/%s.bak.tar.gz", *remoteUser, *targetName)
		}

		fmt.Println("Copying to remote machine . . .")
		// Checksum forced
		rsyncArgs := []string{
			"-avz", "--checksum", "-e", "ssh", tempBackupFile, fmt.Sprintf("%s@%s:%s", *remoteUser, *remoteHost, remoteFilePath),
		}
		err = runCommand("rsync", rsyncArgs...)
		if err != nil {
			log.Fatalf("Error sending file to remote server: %v", err)
		}

		log.Printf("Compressed File Successfully Transferred to %s:%s", *remoteHost, remoteFilePath)
	}

	// Clean up temp files if used
	if *skipLocal && tempBackupFile != backupFile {
		err = os.Remove(tempBackupFile)
		if err != nil {
			log.Printf("Warning: Failed to remove temporary backup file %s: %v", tempBackupFile, err)
		}
	}

	// Restart docker container
	if *dockerBool {
		fmt.Println("-------------------------------------------------------------------------")
		fmt.Println("Starting Docker container . . .")
		fmt.Println("-------------------------------------------------------------------------")
		err = runCommand("docker", "compose", "-f", filepath.Join(sourceDir, "docker-compose.yml"), "up", "-d")
		if err != nil {
			log.Fatalf("Error starting Docker container: %v", err)
		}
	}

	log.Printf("Successful backup job on %s", *targetName)
	log.Println("-------------------------------------------------------------------------")
}
