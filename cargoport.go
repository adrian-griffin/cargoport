package main

// Cargoport v0.87.57

import (
	"flag"
	"fmt"
	"log"
	"net"
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
	Version             = "v0.87.57"
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

//------------------------------------------------------------
//<section>        FUNCTIONS
//------------------------------------------------------------

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

// requires command name (docker, tar, etc); accepts multiple arguments
func runCommand(commandName string, args ...string) error {
	cmd := exec.Command(commandName, args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// locate docker compose file based on container name
func findComposeFile(containerName string) (string, error) {
	cmd := exec.Command("docker", "inspect", containerName, "--format", "{{ index .Config.Labels \"com.docker.compose.project.working_dir\" }}")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ERROR: Failed to locate docker compose file for container %s: %v", containerName, err)
	}
	composePath := strings.TrimSpace(string(output))
	return filepath.Join(composePath, "docker-compose.yml"), nil // return filepath to compose
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
		imageInfo += fmt.Sprintf("Image ID: %s  |  Image Digest: %s\n", imageID, digestOutput)
	}

	trimmedImageInfo := strings.TrimSpace(imageInfo)
	err = os.WriteFile(outputFile, []byte(trimmedImageInfo), 0644)
	if err != nil {
		return fmt.Errorf("failed to write docker image version info to file: %v", err)
	}

	fmt.Println("Docker image information and digests saved to", outputFile)
	return nil
}

// returns whether or not any docker services are running from target composefile
func checkDockerRunState(composeFile string) (bool, error) {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "ps", "--services", "--filter", "status=running")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("ERROR: failed to check Docker container status: %v", err)
	}
	runningServices := strings.TrimSpace(string(output))
	if runningServices == "" {
		return false, fmt.Errorf("No active services found from composefile (container is likely off)")
	}
	return true, nil
}

// starts docker container from yaml file
func startDockerContainer(composefile string) error {
	// Restart docker container
	fmt.Println("-------------------------------------------------------------------------")
	fmt.Println("Starting Docker container . . .")
	fmt.Println("-------------------------------------------------------------------------")
	err := runCommand("docker", "compose", "-f", composefile, "up", "-d")
	if err != nil {
		fmt.Printf("Error starting Docker container: %v", err)
		log.Fatalf("ERROR: <docker-tasks> Failed to start Docker container: %v", err)
	}
	log.Println("Successful startup job on docker container")
	return err
}

func compressDirectory(targetDir, outputFile string) error {
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
	err := runCommand(
		"tar",
		"-cvzf",
		outputFile,
		"-C",
		parentDir, // Parent directory
		baseDir,   // Directory to compress
	)
	if err != nil {
		log.Printf("Error compressing directory: %v", err)
		return err
	}

	// print to cli & log to logfile regarding successful directory compression
	log.Printf("Contents of %s successfully compressed to %s",
		targetDir,
		outputFile,
	)
	return nil
}

// handle remote rsync transfer to another node
func sendToRemote(passedRemotePath, passedRemoteUser, passedRemoteHost, backupFileNameBase, targetFileToTransfer string) error {

	// if remote host is empty
	if passedRemoteHost == "" {
		fmt.Println("ERROR: Remote host address (ipv4 or ipv6) must be provided for remote transfer!")
	}
	// if remote host is not a valid ip address
	if net.ParseIP(passedRemoteHost) == nil {
		fmt.Println("ERROR: Remote host address must be a valid ipv4 or ipv6 host!")
	}
	// if remote user not specified, cancel out
	if passedRemoteUser == "" {
		fmt.Println("ERROR: Both remote user & host must be specified when sending to a remote machine!")
	}

	// set default remote file path to remote user's homedir if none is specified
	remoteFilePath := passedRemotePath
	if remoteFilePath == "" {
		remoteFilePath = fmt.Sprintf("/home/%s/%s.bak.tar.gz", passedRemoteUser, backupFileNameBase)
		//remoteFilePath = fmt.Sprintf("/opt/cargoport/remote/%s.bak.tar.gz", dirName)
	}

	log.Printf("Copying to remote %s@%s:%s . . .", passedRemoteUser, passedRemoteHost, remoteFilePath)
	// Checksum forced
	rsyncArgs := []string{
		"-avz", "--checksum", "-e", "ssh", targetFileToTransfer, fmt.Sprintf("%s@%s:%s", passedRemoteUser, passedRemoteHost, remoteFilePath),
	}
	err := runCommand("rsync", rsyncArgs...)
	if err != nil {
		log.Fatalf("ERROR: Failed to send file to remote server: %v", err)
	}

	log.Printf("Compressed File Successfully Transferred to %s:%s", passedRemoteHost, remoteFilePath)

	// if successful return nil
	return nil
}

func main() {

	// Var declarations
	var err error
	dockerBool := false
	remoteSendBool := false

	initLogging() // begins logging for runtime

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
	//customDstRoot := flag.String("dst-root", "", "Custom destination root path (overrides default set in Config)")

	// Remote handling flags
	//remoteSend := flag.Bool("remote-send", false, "Enable sending backup file to remote machine. Additional flags needed!")
	remoteUser := flag.String("remote-user", "", "Remote machine username")
	remoteHost := flag.String("remote-host", "", "Remote machine IPv4 or IPv6 addres")
	remoteTargetDir := flag.String("remote-dir", "", "Remote target directory, file will save as <file>.bak.tar.gz. Defaults to remote user's homedir if left blank")

	flag.Parse()

	// Informational flags processed first
	if *appVersion {
		fmt.Printf("cargoport\n")
		fmt.Printf("version: %s  ~  kind words cost nothing", Version)
		os.Exit(0)
	}

	//----------------------------------------------------------------
	//<section>        FLAG VALIDATIONS
	//----------------------------------------------------------------

	// If neither target directory nor target docker service name is supplied
	if *targetDir == "" && *dockerName == "" {
		fmt.Println("ERROR: Must specify a target directory or a Docker container name!")
		os.Exit(1)
		// Both target dir & target docker are supplied
	} else if *targetDir != "" && *dockerName != "" {
		fmt.Println("ERROR: Cannot specify both a target directory and a Docker container name. Please pick one.")
		os.Exit(1)
	}

	//<subsection>  REMOTE SEND VALIDATION LOGIC
	//------------

	// If either remoteUser or remoteTargetDir are passed at runtime but remoteHost is NOT passed, Fatalf
	if (*remoteUser != "" || *remoteTargetDir != "") && *remoteHost == "" {
		fmt.Println("ERROR: Remote host must be specified when passing a remote user or remote target!")
		log.Fatalf("ERROR: Remote host must be specified when passing a remote user or remote target!")
	}

	// Enable remoteSend functionality if `remote-host` passed
	if *remoteHost != "" {
		remoteSendBool = true
	}

	// If skip local backup is enabled, but remoteSendBool is not set to true
	if *skipLocal && !remoteSendBool {
		log.Fatalf("ERROR: Skipping the local backup requires a remote host to be defined!")
	}

	//------------------------------------------------------------
	//<section>        DEFAULT CARGOPORT PATH LOGIC
	//------------------------------------------------------------

	// Use actual string value from flag & trim trailing `/` if exists
	targetDirectory := strings.TrimSuffix(*targetDir, "/")

	// Ensure targetDirectory exists before continuing
	if _, err := os.Stat(targetDirectory); os.IsNotExist(err) {
		log.Fatalf("ERROR: <init> Target directory does not exist: %s", targetDirectory)
	}

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

	//------------------------------------------------------------
	//<section>        DOCKER LOGIC
	//------------------------------------------------------------

	// Check if target dir contains docker compose, if so then handle docker shutdown before backup
	var composeFilePath string
	var imageVersionFile string

	// If the user passed `-docker-name`, find compose file
	// Otherwise, if user passed `-target-dir`, check if targetDir has compose file
	if *dockerName != "" {
		composeFilePath, err = findComposeFile(*dockerName)
		if err != nil {
			log.Fatalf("Error locating Docker Compose file: %v", err)
		}

		dockerBool = true
		targetDirectory = filepath.Dir(composeFilePath)
		if _, err := os.Stat(targetDirectory); os.IsNotExist(err) {
			log.Fatalf("Target directory does not exist: %s", targetDirectory)
		}
		dirName = filepath.Base(targetDirectory)

	} else if *targetDir != "" {
		targetDirectory = strings.TrimSuffix(*targetDir, "/")
		dirName = filepath.Base(strings.TrimSuffix(targetDirectory, "/"))

		// Check if docker-compose.yml exists in target directory
		possibleCompose := filepath.Join(targetDirectory, "docker-compose.yml")
		if fi, err := os.Stat(possibleCompose); err == nil && !fi.IsDir() {
			composeFilePath = possibleCompose
			dockerBool = true
		}
	} // else { extra logic needed for regular dir }
	if dirName == "" || dirName == "." {
		log.Fatalf("Invalid directory name derived from targetDirectory: %s", targetDirectory)
	}

	// Determine local target backupfile path
	targetOutputTarball := filepath.Join(
		cargoportLocal,
		dirName+".bak.tar.gz",
	)

	//
	if dockerBool {

		if _, err := os.Stat(composeFilePath); os.IsNotExist(err) {
			log.Fatalf("docker-compose.yml not found at %s", composeFilePath)
		}
		// Ensure Docker container is running
		running, err := checkDockerRunState(composeFilePath)
		if err != nil {
			log.Printf("WARNING: Unable to fully check Docker containers' statuses: %v", err)
		}
		if !running {
			log.Printf("WARNING: Docker container at target is not running, proceeding with backup & starting container services")
		}

		// Get Docker image information & store it in the working dir
		fmt.Println("-------------------------------------------------------------------------")
		fmt.Println("Getting Docker image versions . . .")
		if dockerBool {
			imageVersionFile = filepath.Join(filepath.Dir(composeFilePath), "compose-img-digests.txt")
		} else {
			imageVersionFile = filepath.Join(targetDirectory, "compose-img-digests.txt")
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
	plannedBackupFile := targetOutputTarball
	if *skipLocal {
		plannedBackupFile = filepath.Join(os.TempDir(), dirName+".bak.tar.gz")
	}

	// compress target directory
	compressDirectory(targetDirectory, plannedBackupFile)

	if !*skipLocal {
		fmt.Println("-------------------------------------------------------------------------")
		fmt.Println("Backupfile saved at:", plannedBackupFile)
	}

	// after compression to either os.tempdir or flag-passed output dir
	// send to remote machine if given flags to do so

	// if remote send is enabled, perform transfer of compressionfile
	if remoteSendBool {
		sendToRemote(*remoteTargetDir, *remoteUser, *remoteHost, dirName, plannedBackupFile)
		remoteSendBool = false // just to be safe tbh
	}

	// Clean up temp files if used
	if *skipLocal && plannedBackupFile != targetOutputTarball {
		err = os.Remove(plannedBackupFile)
		if err != nil {
			log.Printf("Warning: Failed to remove temporary backup file %s: %v", plannedBackupFile, err)
		}
	}
	// Further skipLocal && target logic handling needed !

	// Start docker container if dockermode enabled before shutting down
	if dockerBool {
		startDockerContainer(composeFilePath)
	}

	log.Printf("Successful backup job on %s", dirName)
	log.Println("-------------------------------------------------------------------------")
}
