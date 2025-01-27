package main

// Cargoport v0.88.20

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	//"archive/tar"
	//"compress/gzip"
)

const (
	Version        = "v0.88.20"
	TrueConfigfile = "/etc/cargoport.conf"
	motd           = "kind words cost nothing"
)

// declare config struct
type ConfigFile struct {
	DefaultCargoportDir string
	SkipLocal           bool
	RemoteUser          string
	RemoteHost          string
	RemoteOutputDir     string
	Version             string
}

//------------------------------------------------------------
//<section>        FUNCTIONS
//------------------------------------------------------------

//<subsection>   INIT-RELATED FUNCTIONS
//------------

// parse config file
func loadConfigFile(path string) (*ConfigFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %v", err)
	}
	defer file.Close()

	config := &ConfigFile{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// skip comments or empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// split keys and values
		parts := strings.SplitN(line, ":", 2)

		// validate that line is interpreted with 2 returned values
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid config line: %s", line)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// map keys to config fields
		switch key {
		case "default_cargoport_directory":
			config.DefaultCargoportDir = value
		case "version":
			config.Version = value
		case "default_remote_user":
			config.RemoteUser = value
		case "default_remote_host":
			config.RemoteHost = value
		case "default_remote_output_dir":
			config.RemoteOutputDir = value
		case "skip_local_backups":
			skipLocal, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("invalid boolean value for skip_local_backups: %s", value)
			}
			config.SkipLocal = skipLocal
		default:
			return nil, fmt.Errorf("unknown yaml key in config.yml: %s", key)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error while attempting to read config file: %v", err)
	}

	return config, nil
}

// inits logging services
func initLogging(cargoportBase string) (logFilePath string) {
	logFilePath = filepath.Join(cargoportBase, "cargoport-main.log")
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("ERROR: Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	return logFilePath
}

// ensures that local default cargoport dir exists; returns `cargoportBase, cargoportLocal, cargoportRemote, nil`
func initCargoportDirs(configFile ConfigFile) (string, string, string, error) {
	var err error

	// Create /var/cargoport/ directories on local machine
	cargoportBase := strings.TrimSuffix(configFile.DefaultCargoportDir, "/")
	cargoportLocal := filepath.Join(cargoportBase, "/local")
	cargoportRemote := filepath.Join(cargoportBase, "/remote")

	// create /var/cargoport/
	if err = os.MkdirAll(cargoportBase, 0755); err != nil {
		log.Fatalf("ERROR <environment>: Error creating directory %s: %v", cargoportLocal, err)
	}
	// create /var/cargoport/local
	if err = os.MkdirAll(cargoportLocal, 0755); err != nil {
		log.Fatalf("ERROR <environment>: Error creating directory %s: %v", cargoportLocal, err)
	}
	// create /var/cargoport/remote
	if err = os.MkdirAll(cargoportRemote, 0755); err != nil {
		log.Fatalf("ERROR <environment>: Error creating directory %s: %v", cargoportRemote, err)
	}

	// set 777 on /var/cargoport/remote for all users to access
	err = runCommand("chmod", "-R", "777", cargoportRemote)
	if err != nil {
		log.Fatalf("ERROR <environtment>: Error setting %s permissions for remotewrite: %v", cargoportRemote, err)
	}

	return cargoportBase, cargoportLocal, cargoportRemote, nil
}

// sets up cargoport parent dirs & logging
func initEnvironment(configFile ConfigFile) (string, string, string, string) {
	// initialize parent cargoport dirs on system
	cargoportBase, cargoportLocal, cargoportRemote, err := initCargoportDirs(configFile)
	if err != nil {
		log.Fatalf("ERROR <environment>: Failed to initialize directories: %v", err)
	}
	// initialize logging
	logFilePath := initLogging(cargoportBase)

	return cargoportBase, cargoportLocal, cargoportRemote, logFilePath
}

// guided setup tool for initial init
func setupTool() {
	fmt.Println("-- Cargoport Setup -----")
	fmt.Println("Welcome to cargoport initial setup . . .")
	fmt.Println(" ")

	// prompt for root directory
	var rootDir string
	fmt.Print("Enter the root directory for Cargoport (default: /var/cargoport/): ")
	fmt.Scanln(&rootDir)
	if rootDir == "" {
		rootDir = "/var/cargoport/"
	}

	// ensure that passed directory name ends in cargoport, otherwise join cargoport onto it
	rootDir = strings.TrimSuffix(rootDir, "/")
	if !strings.HasSuffix(rootDir, "cargoport") {
		rootDir = filepath.Join(rootDir, "cargoport")
	}
	fmt.Printf("Using root directory: %s\n", rootDir)

	// temp configfile for setup & init
	configFile := ConfigFile{
		DefaultCargoportDir: rootDir,
		SkipLocal:           true, // Default behavior
		RemoteUser:          "",
		RemoteHost:          "",
		RemoteOutputDir:     filepath.Join(rootDir, "remote/"),
		Version:             Version,
	}

	// init env and determine directories & logfile
	cargoportBase, cargoportLocal, cargoportRemote, logFilePath := initEnvironment(configFile)

	// print new dir and logfile information
	fmt.Printf("Base directory initialized at: %s\n", cargoportBase)
	fmt.Printf("Local backup directory: %s\n", cargoportLocal)
	fmt.Printf("Remote backup directory: %s\n", cargoportRemote)
	fmt.Printf("Log file initialized at: %s\n", logFilePath)

	// check for existing config.yml
	configFilePath := filepath.Join(cargoportBase, "config.yml")
	// if not exist then prompt to create default config
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		var createConfig string
		fmt.Printf("No config.yml found in %s. Would you like to create one? (y/n): ", cargoportBase)
		fmt.Scanln(&createConfig) // scan for input
		// if yes then invoke createDefaultConfig
		if strings.ToLower(createConfig) == "y" {
			err := createDefaultConfig(configFilePath, rootDir)
			if err != nil {
				log.Fatalf("ERROR: Failed to create config.yml: %v\n", err)
			}
			fmt.Printf("Default config.yml created at %s.\n", configFilePath)
		} else {
			fmt.Println("Skipping config.yml creation.")
		}
	} else {
		fmt.Println("Detected existing config.yml in parent directory")
	}

	if err := saveTrueConfigReference(configFilePath); err != nil {
		log.Fatalf("ERROR: Failed to save true config reference: %v\n", err)
	}

	fmt.Println("Environment setup completed successfully.")
}

// handles writes between true configfile at /etc/ an configfile reference in declared parent dir
func saveTrueConfigReference(configFilePath string) error {
	return os.WriteFile(TrueConfigfile, []byte(configFilePath), 0644)
}

// create default config and write to ./config.yml
func createDefaultConfig(configFilePath, rootDir string) error {
	// Template for default config.yml
	defaultConfig := fmt.Sprintf(`# [ LOCAL CARGOPORT DEFAULTS ]
## Cargoport Root Directory
## Please change default dir using -setup flag
default_cargoport_directory: %s

## Skip all local backups unless otherwise specified (-skip-local=false for local jobs)
skip_local_backups: false

# [ REMOTE CARGOPORT DEFAULTS]
## Default username and host to be used for '-remote-send-defaults' flag
## Default password cannot be set, please use SSH keys instead!
default_remote_user: admin
## Accepts any valid ipv4/6 address or hostname
default_remote_host: 10.0.0.1
#default_remote_output_dir: "/var/cargoport/remote/"
`, rootDir)

	// Write default config file
	return os.WriteFile(configFilePath, []byte(defaultConfig), 0644)
}

// handles user input validation
func validateInput(targetDir, dockerName, remoteUser, remoteHost, remoteOutputDir *string, skipLocal *bool, configFile ConfigFile) error {

	//----------------------------------
	//<section>   CONFIGFILE VALIDATIONS
	//----------------------------------
	// ensure that parentdir is not empty
	if configFile.DefaultCargoportDir == "" {
		return fmt.Errorf("<config> default_cargoport_directory must be defined")
	}

	//----------------------------------
	//<section>   FLAG INPUT VALIDATIONS
	//----------------------------------
	//<subsection>   Validate Backup Target Flags
	///-----------
	// ensure either `targetDir` or `dockerName` is set
	if *targetDir == "" && *dockerName == "" {
		return fmt.Errorf("requires either `-target-dir=<dir>` or `-docker-name=<container>` to proceed")
	}

	// ensure both `targetDir` and `dockerName` are not set simultaneously
	if *targetDir != "" && *dockerName != "" {
		return fmt.Errorf("cannot specify both a target directory and docker container name")
	}

	//<subsection>   Validate Remote Transfer Flags
	///-----------
	// if remote flags are set, ensure `remoteHost` is provided
	if (*remoteUser != "" || *remoteOutputDir != "") && *remoteHost == "" {
		return fmt.Errorf("when remote send input is supplied, a `-remote-host=<host>` is required! Supports IPv4, IPv6, and DNS resolution")
	}

	// validate `remoteHost` a valid IP address or hostname
	if *remoteHost != "" {
		if net.ParseIP(*remoteHost) == nil {
			_, err := net.LookupHost(*remoteHost)
			if err != nil {
				return fmt.Errorf("provided host must be a valid IP(v4/v6) address or queriable hostname: %v", err)
			}
		}
	}

	// ensure `-remote-dir` is not set without `-remote-host` or `-remote-user`
	if *remoteOutputDir != "" && (*remoteHost == "" || *remoteUser == "") {
		return fmt.Errorf("error: `-remote-dir` cannot be used without specifying both `-remote-host` and `-remote-user`")
	}

	//<subsection>   Validate Backup Target Flags
	///-----------
	// if `skipLocal` is true, ensure remote send is configured
	if *skipLocal && (*remoteHost == "" || *remoteUser == "") {
		return fmt.Errorf("when skipping local backup, please ensure that a the necessary remote flags are passed")
	}

	return nil
}

// performs flag/input parsing & handles validations
func interpretflags(
	targetDir, dockerName, localOutputDir *string,
	skipLocal *bool,
	remoteUser, remoteHost, remoteOutputDir *string,
	sendDefaults *bool,
	configFile ConfigFile,
) {
	// validate or override flags with configfile values
	if *sendDefaults {
		if configFile.RemoteUser != "" {
			*remoteUser = configFile.RemoteUser
		}
		if configFile.RemoteHost != "" {
			*remoteHost = configFile.RemoteHost
		}

		if configFile.RemoteUser == "" || configFile.RemoteHost == "" {
			log.Fatalf("ERROR: Default remote host and username must be set in the configuration file to use -remote-send-defaults.")
		}
	}
	// validate inputs
	err := validateInput(targetDir, dockerName, remoteUser, remoteHost, remoteOutputDir, skipLocal, configFile)
	if err != nil {
		log.Fatalf("ERROR <input>: %v", err)
	}
}

// determines configfile path
func getConfigFilePath() (string, error) {
	data, err := os.ReadFile(TrueConfigfile)
	if err != nil {
		return "", fmt.Errorf("failed locate environment")
	}
	configFilePath := strings.TrimSpace(string(data))
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		return "", fmt.Errorf("config file %s not found", configFilePath)
	}
	return configFilePath, nil
}

//<subsection>   DOCKER FUNCTIONS
//------------

// stop docker containers & collect image ids and digests
func handleDockerPreBackup(composeFilePath string) error {
	fmt.Println("-------------------------------------------------------------------------")
	fmt.Println("Handling Docker pre-backup tasks . . .")
	fmt.Println("-------------------------------------------------------------------------")

	// checks whether docker is running
	running, err := checkDockerRunState(composeFilePath)
	if err != nil || !running {
		log.Printf("WARNING <docker>: No active Docker container at target location. Proceeding with backup.")
	}

	// gathers and writes images to disk
	imageVersionFile := filepath.Join(filepath.Dir(composeFilePath), "compose-img-digests.txt")
	if err := writeDockerImages(composeFilePath, imageVersionFile); err != nil {
		return fmt.Errorf("failed to collect Docker images: %v", err)
	}

	// shuts down docker container from composefile
	log.Println("Performing Docker compose jobs . . .")
	if err := runCommand("docker", "compose", "-f", composeFilePath, "down"); err != nil {
		return fmt.Errorf("failed to stop Docker containers: %v", err)
	}
	return nil
}

// handles docker container restart/turn-up commands
func handleDockerPostBackup(composeFilePath string) error {
	log.Println("Restarting Docker compose . . .")
	if err := startDockerContainer(composeFilePath); err != nil {
		return fmt.Errorf("failed to restart Docker containers at : %s", composeFilePath)
	}
	return nil
}

// locates docker compose file based on container name
func findComposeFile(containerName string) (string, error) {
	cmd := exec.Command("docker", "inspect", containerName, "--format", "{{ index .Config.Labels \"com.docker.compose.project.working_dir\" }}")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("ERROR <storage>: Failed to locate docker compose file for container '%s': %v", containerName, err)
		return "", fmt.Errorf("failed to locate docker compose file for container '%s': %v", containerName, err)
	}
	composePath := strings.TrimSpace(string(output))
	return filepath.Join(composePath, "docker-compose.yml"), nil // return filepath to compose
}

// collects docker image information and digests, stores alongside `docker-compose.yml` file
func writeDockerImages(composeFile string, outputFile string) error {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "images", "--quiet")
	output, err := cmd.Output()
	if err != nil {

		return fmt.Errorf("Failed to obtain docker images: %v", err)
	}

	// loops over image ids to gather docker image digests
	imageLines := string(output)
	imageList := strings.Split(imageLines, "\n")
	var imageInfo string

	for _, imageID := range imageList {
		if imageID == "" {
			continue
		}

		// grab image digests based on image id
		cmdInspect := exec.Command("docker", "inspect", "--format", "{{index .RepoDigests 0}}", imageID)
		digestOutput, err := cmdInspect.Output()
		if err != nil {
			return fmt.Errorf("Failed to inspect docker images: %v", err)
		}
		imageInfo += fmt.Sprintf("Image ID: %s  |  Image Digest: %s\n", imageID, digestOutput)
	}

	// cleans up image whitespace formatting and writes to file
	trimmedImageInfo := strings.TrimSpace(imageInfo)
	err = os.WriteFile(outputFile, []byte(trimmedImageInfo), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write docker image version info to file: %v", err)
	}

	fmt.Println("Docker image IDs and digests saved to", outputFile)
	return nil
}

// returns whether or not any docker services are running from target composefile
func checkDockerRunState(composeFile string) (bool, error) {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "ps", "--services", "--filter", "status=running")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("Failed to obtain Docker container status: %v", err)
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
		log.Fatalf("ERROR <docker>: Failed to start Docker container: %v", err)
	}
	log.Printf("Successful startup job on docker compose at %s", composefile)
	return err
}

//<subsection>   OS/SYS FUNCTIONS
//------------

// determines path for new backupfile based on user input
func prepareBackupFilePath(localBackupDir, targetDir, customOutputDir string, skipLocal bool) string {
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

// determines target dir for backup based on input user input
func determineBackupTarget(targetDir, dockerName *string) (string, string, bool) {
	var composeFilePath string
	dockerEnabled := false

	// validates composefile, returns its path and dirpath, and enables dockerMode
	if *dockerName != "" {
		var err error
		composeFilePath, err = findComposeFile(*dockerName)
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

// compresses target directory into output file tarball
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
		log.Printf("Error compressing directory: %s/%s", parentDir, baseDir)
		os.Remove(outputFile) // ensure partial file is cleaned up
		return err
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

// executes command on local system
func runCommand(commandName string, args ...string) error {
	cmd := exec.Command(commandName, args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// defines log & stdout styling and content at start of backups
func logStart(format string, args ...interface{}) {
	log.Println("-------------------------------------------------------------------------")
	log.Printf(format, args...)
	log.Println("-------------------------------------------------------------------------")
	fmt.Println("-------------------------------------------------------------------------")
	fmt.Printf(format, args...)
	fmt.Println("-------------------------------------------------------------------------")
}

// defines log & stdout styling and content at end of backups
func logEnd(format string, args ...interface{}) {

	log.Println("-------------------------------------------------------------------------")
	log.Printf(format, args...)
	log.Println("-------------------------------------------------------------------------")
	fmt.Println("-------------------------------------------------------------------------")
	fmt.Printf(format, args...)
	fmt.Println("-------------------------------------------------------------------------")
}

//<subsection>   REMOTE TRANSFER FUNCTIONS
//------------

// wrapper function for remoteSend
func handleRemoteTransfer(filePath, remoteUser, remoteHost, remoteOutputDir string, skipLocal bool, configFile ConfigFile) {

	// ensure remote host is reachable
	if err := checkRemoteHost(remoteHost, remoteUser); err != nil {
		log.Fatalf("ERROR <remote>: %v", err)
	}

	// proceed with remote transfer
	err := sendToRemote(remoteOutputDir, remoteUser, remoteHost, filepath.Base(filePath), filePath, configFile)
	if err != nil {
		log.Fatalf("ERROR <remote>: %v", err)
	}

	// clean up local tempfile after transfer if skipLocal is enabled
	if skipLocal {
		os.Remove(filePath)
		log.Printf("Cleaning up tempfile at %s\n", filePath)
		fmt.Printf("Tempfile %s removed\n", filePath)
	}
}

// handle remote rsync transfer to another node
func sendToRemote(passedRemotePath, passedRemoteUser, passedRemoteHost, backupFileNameBase, targetFileToTransfer string, configFile ConfigFile) error {

	//<section>  VALIDATIONS
	//---------
	// if remote host or user are empty
	if passedRemoteHost == "" || passedRemoteUser == "" {
		return fmt.Errorf("Both remote user and host must be specified for remote transfer!")
	}

	var remoteFilePath string

	if passedRemotePath != "" {
		// Custom path provided at runtime
		remoteFilePath = strings.TrimSuffix(passedRemotePath, "/")
		remoteFilePath = fmt.Sprintf("%s/%s", remoteFilePath, backupFileNameBase)
	} else {
		// Fallback to configuration-defined path
		remoteFilePath = fmt.Sprintf("%s/remote/%s", strings.TrimSuffix(configFile.DefaultCargoportDir, "/"), backupFileNameBase)
	}

	log.Printf("Copying to remote %s@%s:%s . . .", passedRemoteUser, passedRemoteHost, remoteFilePath)

	rsyncArgs := []string{
		"-avz", "--checksum", "-e", "ssh", targetFileToTransfer,
		fmt.Sprintf("%s@%s:%s", passedRemoteUser, passedRemoteHost, remoteFilePath),
	}

	err := runCommand("rsync", rsyncArgs...)
	if err != nil {
		return fmt.Errorf("Failed to successfully perform rsync with remote server: %v", err)
	}

	log.Printf("Compressed File Successfully Transferred to %s", passedRemoteHost)
	return nil
}

// determine if remote host is a valid target
func checkRemoteHost(remoteHost, remoteUser string) error {
	// Ping the host to check connectivity
	pingCmd := exec.Command("ping", "-c", "1", "-W", "2", remoteHost)
	if err := pingCmd.Run(); err != nil {
		fmt.Printf("ERROR <remote>: Target remote host %s is unreachable!\n", remoteHost)
		return fmt.Errorf("remote host %s is unreachable", remoteHost)
	}

	// Check SSH connectivity
	sshCmd := exec.Command("ssh", fmt.Sprintf("%s@%s", remoteUser, remoteHost), "exit")
	if err := sshCmd.Run(); err != nil {
		fmt.Printf("ERROR <remote>: Failed to estblish SSH connection to remote remote host at %s@%s", remoteUser, remoteHost)
		return fmt.Errorf("failed to establish SSH connection to %s@%s", remoteUser, remoteHost)
	}

	return nil
}

//------------------------------------------------------------
//<section>                MAIN
//------------------------------------------------------------

func main() {

	// define flags
	// special flags
	appVersion := flag.Bool("version", false, "Display app version information")
	initEnvBool := flag.Bool("setup", false, "Run setup utility")

	// primary backup flags
	targetDir := flag.String("target-dir", "", "Target directory to back up (detects if the directory is a Docker environment)")
	dockerName := flag.String("docker-name", "", "Target Docker service name (involves all Docker containers defined in the compose file)")
	localOutputDir := flag.String("output-dir", "", "Custom destination for local output")

	// remote transfer flags
	skipLocal := flag.Bool("skip-local", false, "Skip local backup & only send to remote target")
	remoteUser := flag.String("remote-user", "", "Remote machine username")
	remoteHost := flag.String("remote-host", "", "Remote machine IP(v4/v6) address or hostname")
	remoteOutputDir := flag.String("remote-dir", "", "Remote target directory (file saved as <remote-dir>/<file>.bak.tar.gz)")
	sendDefaultsBool := flag.Bool("remote-send-defaults", false, "Toggles remote send functionality using configfile default creds, overrides remote-user and remote-host flags")

	// custom help message
	flag.Usage = func() {
		fmt.Println("------------------------------------------------------------------------")
		fmt.Printf("cargoport %s  ~  %s\n", Version, motd)
		fmt.Println("-------------------------------------------------------------------------")
		fmt.Println("\n[Options]")
		fmt.Println("\n  [Setup & Info]")
		fmt.Println("     -setup")
		fmt.Println("        Run setup utility to initialize the environment")
		fmt.Println("     -version")
		fmt.Println("        Display app version information")
		fmt.Println("\n  [Local Backup Flags]")
		fmt.Println("     -target-dir <dir>")
		fmt.Println("        Target directory to back up (detects if the directory is a Docker environment)")
		fmt.Println("     -output-dir <dir>")
		fmt.Println("        Custom destination for local output")
		fmt.Println("     -docker-name <name>")
		fmt.Println("        Target Docker service name (involves all Docker containers defined in the compose file)")
		fmt.Println("\n  [Remote Transfer Flags]")
		fmt.Println("     -skip-local")
		fmt.Println("        Skip local backup and only send to the remote target (Note: /tmp/ used for tempfile)")
		fmt.Println("     -remote-user <user>")
		fmt.Println("        Remote machine username")
		fmt.Println("     -remote-host <host>")
		fmt.Println("        Remote machine IP(v4/v6) address or hostname")
		fmt.Println("     -remote-dir <dir>")
		fmt.Println("        Remote target directory (file will save as <remote-dir>/<file>.bak.tar.gz)")
		fmt.Println("     -remote-send-defaults")
		fmt.Println("        Remote transfer backup using default remote values in config.yml")
		fmt.Println("\n[Examples]")
		fmt.Println("  cargoport -setup")
		fmt.Println("  cargoport -target-dir=/path/to/dir -remote-user=admin -remote-host=10.115.0.1")
		fmt.Println("  cargoport -docker-name=container-name -remote-send-defaults -skip-local")
		fmt.Println("\nFor more information, please see the documentation")
	}

	flag.Parse()

	//<section>   Special Flags
	//------------
	// issue version info
	if *appVersion {
		fmt.Printf("cargoport  ~  %s\n", motd)
		fmt.Printf("version: %s", Version)
		os.Exit(0)
	}

	// if setup flag passed
	if *initEnvBool {
		setupTool()
		os.Exit(0)
	}

	//<section>   Begin Environment Checks, Validations, & Map Paths
	//------------

	// determine configfile location
	configFilePath, err := getConfigFilePath()
	if err != nil {
		log.Fatalf("ERROR <config>: %v\nPlease run `cargoport -setup` first!", err)
	}

	// parse config file to set defaults
	configFile, err := loadConfigFile(configFilePath)
	if err != nil {
		log.Fatalf("ERROR <config>: %v", err)
	}

	// interpret already parsed flags
	interpretflags(targetDir, dockerName, localOutputDir, skipLocal, remoteUser, remoteHost, remoteOutputDir, sendDefaultsBool, *configFile)

	// init environment
	_, cargoportLocal, _, _ := initEnvironment(*configFile)

	// determine backup target and prepare execution context
	targetDirectory, composeFilePath, dockerEnabled := determineBackupTarget(targetDir, dockerName)
	backupFilePath := prepareBackupFilePath(cargoportLocal, targetDirectory, *localOutputDir, *skipLocal)

	//<section>   Begin Backups
	//------------
	timeBeginJob := time.Now()
	// log & print job start
	logStart("New Backup Job    |    cargoport %s    |    <%s>\n", Version, filepath.Base(targetDirectory))

	// handle pre-backup docker tasks
	if dockerEnabled {
		if err := handleDockerPreBackup(composeFilePath); err != nil {
			log.Fatalf("ERROR <docker>: Pre-compression Docker tasks failed: %v", err)
		}
	}
	// compress directory to file
	if err := compressDirectory(targetDirectory, backupFilePath); err != nil {
		log.Fatalf("ERROR <storage>: Failed to compress directory: %v", err)
	}

	// handle remote transfer
	if *remoteHost != "" {
		handleRemoteTransfer(backupFilePath, *remoteUser, *remoteHost, *remoteOutputDir, *skipLocal, *configFile)
	}

	//<section>   Post Backup/Restarts
	//------------

	// handle docker post backup
	if dockerEnabled {
		if err := handleDockerPostBackup(composeFilePath); err != nil {
			log.Fatalf("ERROR <docker>: %v", err)
		}
	}

	// job completion banner & time calculation
	jobDuration := time.Since(timeBeginJob)
	executionSeconds := jobDuration.Seconds()
	//                   |        time  5.37s       |
	logEnd("Job Success       |        time  %.2fs       |    <%s>\n", executionSeconds, filepath.Base(targetDirectory))
}
