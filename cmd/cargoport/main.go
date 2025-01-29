package main

// Cargoport v0.88.30

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/adrian-griffin/cargoport/backup"
	"github.com/adrian-griffin/cargoport/docker"
	"github.com/adrian-griffin/cargoport/environment"
	"github.com/adrian-griffin/cargoport/inputhandler"
	"github.com/adrian-griffin/cargoport/keytool"
	"github.com/adrian-griffin/cargoport/remote"
)

const Version = "v0.88.30"
const motd = "kind words cost nothing"

func main() {

	//<section>   PARSE FLAGS
	//------------
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

	// ssh key flags
	newSSHKeyBool := flag.Bool("generate-key", false, "Generate new SSH key for cargoport")
	copySSHKeyBool := flag.Bool("copy-key", false, "Copy cargoport SSH key to remote host")

	flag.Parse()

	//<section>   SPECIAL FLAGS
	//------------
	// issue version info
	if *appVersion {
		fmt.Printf("cargoport  ~  %s\n", motd)
		fmt.Printf("version: %s", Version)
		os.Exit(0)
	}

	// if setup flag passed
	if *initEnvBool {
		environment.SetupTool()
		os.Exit(0)
	}

	//<section>   LOAD CONFIG & INIT ENV
	//------------
	// determine configfile location
	configFilePath, err := environment.GetConfigFilePath()
	if err != nil {
		log.Fatalf("ERROR <config>: %v\nPlease run `cargoport -setup` first!", err)
	}
	// parse config file to set defaults
	configFile, err := environment.LoadConfigFile(configFilePath)
	if err != nil {
		log.Fatalf("ERROR <config>: %v", err)
	}

	// init environment
	_, cargoportLocal, _, _, _ := environment.InitEnvironment(*configFile)

	if *newSSHKeyBool {
		sshKeyDir := configFile.SSHKeyDir
		sshKeyName := configFile.SSHKeyName
		if err := keytool.GenerateSSHKeypair(sshKeyDir, sshKeyName); err != nil {
			log.Fatalf("ERROR: Failed to generate SSH key: %v", err)
		}
		os.Exit(0)
	}

	// if both remote user and remote host are specified during copy command, then proceed
	if *copySSHKeyBool {
		if *remoteHost == "" || *remoteUser == "" {
			log.Fatal("Remote user and host must be specified in the config file to copy SSH key")
		}
		//
		sshPrivKeypath := filepath.Join(configFile.SSHKeyDir, configFile.SSHKeyName)
		if err := keytool.CopyPublicKey(sshPrivKeypath, *remoteUser, *remoteHost); err != nil {
			log.Fatalf("ERROR: Failed to copy SSH public key: %v", err)
		}
		os.Exit(0)
	}

	// interpret backup-related flags
	inputhandler.InterpretFlags(targetDir, dockerName, localOutputDir, skipLocal, remoteUser, remoteHost, remoteOutputDir, sendDefaultsBool, *configFile)

	//<section>   Begin Backups
	//------------
	// determine backup target
	targetPath, composeFilePath, dockerEnabled := backup.DetermineBackupTarget(targetDir, dockerName)

	// prepare local backupfile & compose
	backupFilePath := backup.PrepareBackupFilePath(cargoportLocal, targetPath, "", false)

	// begin backup job timer
	timeBeginJob := time.Now()

	// log & print job start
	environment.LogStart("New Backup Job    |    cargoport %s    |    <%s>\n", Version, filepath.Base(targetPath))

	// handle pre-backup docker tasks
	if dockerEnabled {
		if err := docker.HandleDockerPreBackup(composeFilePath); err != nil {
			log.Fatalf("ERROR <docker>: Pre-compression Docker tasks failed: %v", err)
		}
	}

	err = backup.CompressDirectory(targetPath, backupFilePath)
	if err != nil {
		log.Fatalf("Compression failed: %v", err)
	}

	// handle remote transfer
	if *remoteHost != "" {
		remote.HandleRemoteTransfer(backupFilePath, *remoteUser, *remoteHost, *remoteOutputDir, *skipLocal, *configFile)
	}

	//<section>   Post Backup/Restarts
	//------------

	// handle docker post backup
	if dockerEnabled {
		if err := docker.HandleDockerPostBackup(composeFilePath); err != nil {
			log.Fatalf("ERROR <docker>: %v", err)
		}
	}

	// job completion banner & time calculation
	jobDuration := time.Since(timeBeginJob)
	executionSeconds := jobDuration.Seconds()
	//                   |        time  5.37s       |
	environment.LogEnd("Job Success       |        time  %.2fs       |    <%s>\n", executionSeconds, filepath.Base(targetPath))

	// 10) Done
}
