package main

// Cargoport v0.87.50

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
	//// IMPORTANT: In order to change default cargoport storage directory, the following vars must be edited and the binar rebuilt~!
	// Don't forget the trailing `/` here!

	// Defines the root directory storage directory for cargoport, default is `/opt/cargoport/` and all backups, both local and remote, are stored here
	//DefaultTargetRoot = "/opt/docker/"
	//DefaultBackupRoot = "/opt/docker-backups/"
	DefaultCargoportDir = "/opt/cargoport/"
	Version             = "v0.87.50"
)

// declare config struct
type Config struct {
	DefaultCargoportDir string
	Version             string
	TargetDir           string
	DockerName          string
	RemoteUser          string
	RemoteHost          string
	RemoteSend          bool
	DockerEnabled       bool
	SkipLocal           bool
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
	cmd := exec.Command("docker", "inspect", containerName, "--format", "{{ index .Config.Labels \"com.docker.compose.project.working_dir\" }}")
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

		return fmt.Errorf("failed to get docker images: %v", err)
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
			return fmt.Errorf("failed to inspect docker images: %v", err)
		}
		imageInfo += fmt.Sprintf("Image: %s Digest: %s\n", imageID, digestOutput)
	}

	err = os.WriteFile(outputFile, []byte(imageInfo), 0644)
	if err != nil {
		return fmt.Errorf("failed to write docker image version info to file: %v", err)
	}

	fmt.Println("Docker image information and digests saved to", outputFile)
	return nil
}

func checkDockerRunState(composeFile string) (bool, error) {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "ps", "--services", "--filter", "status=running")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check Docker container status: %v", err)
	}
	runningServices := strings.TrimSpace(string(output))
	if runningServices == "" {
		return false, nil
	}
	return true, nil
}

func main() {

	initLogging() // begins logging for runtime

	var err error

	// Defines config values
	config := Config{
		DefaultCargoportDir: DefaultCargoportDir,
		Version:             Version,
	}

	// Flag definitions
	targetDir := flag.String("target-dir", "", "Target directory to back up (Note: Can safely be run even if dir contains docker data!)")
	dockerName := flag.String("docker-name", "", "Target Docker service name  (Note: All containers apart of the destination compose-file will be restarted!)")
	skipLocal := flag.Bool("skip-local", false, "Skip local backup & only send to remote target (Note: Still requires -remote-send!)")
	appVersion := flag.Bool("version", false, "Display app version information")
	// Remote handling flags
	remoteSend := flag.Bool("remote-send", false, "Enable sending backup file to remote machine. Additional flags needed!")
	remoteUser := flag.String("remote-user", "", "Remote machine username")
	remoteHost := flag.String("remote-host", "", "Remote machine IP address")
	remoteFile := flag.String("remote-file", "", "Remote filepath. Defaults to /home/$USER/$TARGETNAME.bak.tar.gz")

	// New flags for custom paths
	//customDstRoot := flag.String("dst-root", "", "Custom destination root path (overrides default set in Config)")

	flag.Parse()

	// Informational flags processed first
	if *appVersion {
		fmt.Printf("cargoport version: %s", Version)
		os.Exit(0)
	}

	// Flag validations

	// Neither target directory nor target docker service name is supplied
	if *targetDir == "" && *dockerName == "" {
		fmt.Println("ERROR: Must specify either a target directory OR a Docker container name (but not both).")
		os.Exit(1)

		// Both target dir & target docker are supplied
	} else if *targetDir != "" && *dockerName != "" {
		fmt.Println("ERROR: Cannot specify BOTH a target directory AND a Docker container name. Please pick one.")
		os.Exit(1)
	}

	if *skipLocal && !*remoteSend {
		fmt.Println("ERROR: -skip-local requires -remote-send to be set!")
		fmt.Println("Exiting ...")
		os.Exit(1)
	}

	// User actual string value from flag
	sourceDir := *targetDir //

	var dirName string

	// Create /opt/cargoport/ & /opt/cargoport/local/ directories on local machine
	cargoportBase := config.DefaultCargoportDir
	cargoportLocal := filepath.Join(cargoportBase, "local")
	cargoportRemote := filepath.Join(cargoportBase, "remote")

	if err = os.MkdirAll(cargoportLocal, 0755); err != nil {
		log.Fatalf("Error creating directory %s: %v", cargoportLocal, err)
	}

	if err = os.MkdirAll(cargoportRemote, 0755); err != nil {
		log.Fatalf("Error creating directory %s: %v", cargoportRemote, err)
	}

	log.Println("-------------------------------------------------------------------------")
	log.Printf("Beginning Backup Job -- cargoport %s", Version)

	// Check if target dir contains docker compose, if so then handle docker shutdown before backup

	dockerBool := false
	var composeFilePath string

	var imageVersionFile string

	// If the user passed -docker-name
	if *dockerName != "" {
		composeFilePath, err = findComposeFile(*dockerName)
		if err != nil {
			log.Fatalf("Error locating Docker Compose file: %v", err)
		}

		dockerBool = true
		sourceDir = filepath.Dir(composeFilePath)
		dirName = filepath.Base(sourceDir)

	} else if *targetDir != "" {
		sourceDir = *targetDir
		dirName = filepath.Base(strings.TrimSuffix(sourceDir, "/"))

		// Check if docker-compose.yml exists in target directory
		possibleCompose := filepath.Join(sourceDir, "docker-compose.yml")
		if fi, err := os.Stat(possibleCompose); err == nil && !fi.IsDir() {
			composeFilePath = possibleCompose
			dockerBool = true
		}
	}

	if dirName == "" || dirName == "." {
		log.Fatalf("Invalid directory name derived from sourceDir: %s", sourceDir)
	}

	backupFile := filepath.Join(
		cargoportLocal,
		dirName+".bak.tar.gz",
	)

	if dockerBool {

		// This logic should never apply if docker-compose.yml does not exist
		// Buuut extra check anyway:
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
		if dockerBool {
			imageVersionFile = filepath.Join(filepath.Dir(composeFilePath), "compose-img-digests.txt")
		} else {
			imageVersionFile = filepath.Join(sourceDir, "compose-img-digests.txt")
		}
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

	// Create temp backup filepath if skipping local backup
	tempBackupFile := backupFile
	if *skipLocal {
		tempBackupFile = filepath.Join(os.TempDir(), dirName+".bak.tar.gz")
	}

	// Compress target directory
	fmt.Println("-------------------------------------------------------------------------")
	fmt.Println("Compressing container directory . . .")
	fmt.Println("-------------------------------------------------------------------------")
	err = runCommand(
		"tar",
		"-cvzf",
		tempBackupFile,
		"-C",
		filepath.Dir(sourceDir),  // Parent directory
		filepath.Base(sourceDir), // Directory to compress
	)
	if err != nil {
		log.Fatalf("Error compressing directory: %v", err)
	}

	log.Printf("Contents of %s successfully compressed to %s",
		*targetDir,
		tempBackupFile,
	)

	if !*skipLocal {
		fmt.Println("-------------------------------------------------------------------------")
		fmt.Println("Backupfile saved at:", tempBackupFile)
	}

	// Handle remote rsync transfer
	if *remoteSend {
		if *remoteUser == "" || *remoteHost == "" {
			log.Fatalf("Remote user and host must be specified when sending to a remote machine.")
		}

		// Set default remote file path to remote user's homedir if none is specified
		remoteFilePath := *remoteFile
		if remoteFilePath == "" {
			remoteFilePath = fmt.Sprintf("/home/%s/%s.bak.tar.gz", *remoteUser, dirName)
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
	if dockerBool {
		fmt.Println("-------------------------------------------------------------------------")
		fmt.Println("Starting Docker container . . .")
		fmt.Println("-------------------------------------------------------------------------")
		err = runCommand("docker", "compose", "-f", filepath.Join(sourceDir, "docker-compose.yml"), "up", "-d")
		if err != nil {
			log.Fatalf("Error starting Docker container: %v", err)
		}
	}

	log.Printf("Successful backup job on %s", dirName)
	log.Println("-------------------------------------------------------------------------")
}
