package main

// Cargoport

import (
	"flag"
	"fmt"

	// "log"
	"os"
	"path/filepath"
	"time"

	"github.com/adrian-griffin/cargoport/backup"
	"github.com/adrian-griffin/cargoport/docker"
	"github.com/adrian-griffin/cargoport/environment"
	"github.com/adrian-griffin/cargoport/inputhandler"
	"github.com/adrian-griffin/cargoport/jobcontext"
	"github.com/adrian-griffin/cargoport/keytool"
	"github.com/adrian-griffin/cargoport/logger"
	"github.com/adrian-griffin/cargoport/remote"
	"github.com/adrian-griffin/cargoport/sysutil"
)

const Version = "v0.92.44"
const motd = "kind words cost nothing <3"

// debug level logging output fields for main package
func mainLogDebugFields(context *jobcontext.JobContext) map[string]interface{} {
	coreFields := logger.CoreLogFields(context, "main")
	fields := logger.MergeFields(coreFields, map[string]interface{}{
		"remote":         context.Remote,
		"docker":         context.Docker,
		"skip_local":     context.SkipLocal,
		"target_dir":     context.TargetDir,
		"tag":            context.Tag,
		"restart_docker": context.RestartDocker,
		"remote_host":    context.RemoteHost,
		"remote_user":    context.RemoteUser,
	})

	return fields
}

// fatal level logging output fields for main package
func mainLogFatalFields(context *jobcontext.JobContext) map[string]interface{} {
	debugFields := mainLogDebugFields(context)
	fields := logger.MergeFields(debugFields, map[string]interface{}{
		"success": false,
	})

	return fields
}

// main loop
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
	restartDockerBool := flag.Bool("restart-docker", true, "Restart docker container after successful backup. Enabled by default")

	// remote transfer flags
	skipLocal := flag.Bool("skip-local", false, "Skip local backup & only send to remote target")
	remoteUser := flag.String("remote-user", "", "Remote machine username")
	remoteHost := flag.String("remote-host", "", "Remote machine IP(v4/v6) address or hostname")
	remoteOutputDir := flag.String("remote-dir", "", "Remote target directory (file saved as <remote-dir>/<file>.bak.tar.gz)")
	sendDefaultsBool := flag.Bool("remote-send-defaults", false, "Toggles remote send functionality using configfile default creds, overrides remote-user and remote-host flags")

	// ssh key flags
	newSSHKeyBool := flag.Bool("generate-keypair", false, "Generate new SSH key for cargoport")
	copySSHKeyBool := flag.Bool("copy-key", false, "Copy cargoport SSH key to remote host")

	// snapshot versioning and tagging flags
	tagOutputString := flag.String("tag", "", "Append identifying tag to output file name (e.g: service1-<tag>.bak.tar.gz)")

	// custom help messaging
	flag.Usage = func() {
		fmt.Println("------------------------------------------------------------------------")
		fmt.Printf("cargoport %s  ~  %s\n", Version, motd)
		fmt.Println("-------------------------------------------------------------------------")
		fmt.Println("[Options]")
		fmt.Println("  [Setup & Info]")
		fmt.Println("     -setup")
		fmt.Println("        Run setup utility to init the cargoport environment (default is /var/cargoport/)")
		fmt.Println("     -version")
		fmt.Println("        Display app version information")
		fmt.Println("\n  [SSH Key Flags]")
		fmt.Println("     -copy-key")
		fmt.Println("        Copy public key to remote machine, must be passed with remote-host & remote-user")
		fmt.Println("     -generate-keypair")
		fmt.Println("        Generate a new set of SSH keys based on name & location defined in config")
		fmt.Println("\n  [Backup Flags]")
		fmt.Println("     -target-dir <dir>")
		fmt.Println("        Target directory to back up (detects if the directory is a Docker environment)")
		fmt.Println("     -output-dir <dir>")
		fmt.Println("        Custom destination for local output")
		fmt.Println("     -docker-name <name>")
		fmt.Println("        Target Docker service name (involves all Docker containers defined in the compose file)")
		fmt.Println("     -restart-docker <bool>")
		fmt.Println("        Restart docker container after successful backup. Enabled by default")
		fmt.Println("\n  [Remote Transfer Flags]")
		fmt.Println("     -skip-local")
		fmt.Println("        Skip local backup and only send to the remote target (Note: utilized `/tmp`)")
		fmt.Println("     -remote-user <user>")
		fmt.Println("        Remote machine username")
		fmt.Println("     -remote-host <host>")
		fmt.Println("        Remote machine IP(v4/v6) address or hostname")
		fmt.Println("     -remote-dir <dir>")
		fmt.Println("        Remote target directory (file will save as <remote-dir>/<file>.bak.tar.gz)")
		fmt.Println("     -remote-send-defaults")
		fmt.Println("        Remote transfer backup using default remote values in config.yml")
		fmt.Println("     -tag <tag>")
		fmt.Println("        Append identifying tag to output file name (e.g: service1-<tag>.bak.tar.gz)")

		fmt.Println("\n[Examples]")
		fmt.Println("  cargoport -setup")
		fmt.Println("  cargoport -copy-key -remote-host <host> -remote-user <username>")
		fmt.Println("  cargoport -target-dir=/path/to/dir -remote-user=admin -remote-host=<host>")
		fmt.Println("  cargoport -docker-name=container-name -remote-send-defaults -skip-local")
		fmt.Println("  cargoport -docker-name=container-name -tag='pre-pull' -restart-docker=false")

		fmt.Println("\nFor more information, please see the git repo readme")
	}

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
		logger.Logx.Fatal("Failed to read config.yml, please try cargoport -setup first!")
	}
	// parse config file to set defaults
	configFile, err := environment.LoadConfigFile(configFilePath)
	if err != nil {
		logger.Logx.Fatalf("Error parsing config: %v", err)
	}

	// init environment
	_, cargoportLocal, _, _, _ := environment.InitEnvironment(*configFile)

	if *newSSHKeyBool {
		sshKeyDir := configFile.SSHKeyDir
		sshKeyName := configFile.SSHKeyName
		if err := keytool.GenerateSSHKeypair(sshKeyDir, sshKeyName); err != nil {
			logger.Logx.Fatalf("Failed to generate SSH key: %v", err)
		}
		os.Exit(0)
	}

	// validate permissions & integrity on private key
	sshPrivateKeyPath := filepath.Join(configFile.SSHKeyDir, configFile.SSHKeyName)
	if err := keytool.ValidateSSHPrivateKeyPerms(sshPrivateKeyPath); err != nil {
		logger.Logx.Error("Unable to validate keypair, please check configfile or create a new pair")
		logger.Logx.Fatalf("Key validation error: %v", err)
	}

	// if BOTH remote user and remote host are specified during copy command, then proceed
	if *copySSHKeyBool {
		if *remoteHost == "" || *remoteUser == "" {
			logger.Logx.Fatal("Both remote host and user must be specified to copy SSH key")
		}
		// copy public key to remote machine
		sshPrivKeypath := filepath.Join(configFile.SSHKeyDir, configFile.SSHKeyName)
		if err := keytool.CopyPublicKey(sshPrivKeypath, *remoteUser, *remoteHost); err != nil {
			logger.Logx.Errorf("Failed to copy SSH public key: %v", err)
		}
		os.Exit(0)
	}

	// interpret flags & handle config overrides
	err = inputhandler.InterpretFlags(targetDir, dockerName, localOutputDir, skipLocal, remoteUser, remoteHost, remoteOutputDir, sendDefaultsBool, *configFile)
	if err != nil {
		logger.LogxWithFields("fatal", fmt.Sprintf("Failure to parse input: %v", err), map[string]interface{}{
			"package": "main",
			"target":  filepath.Base(*targetDir),
			"success": false,
		})
	}

	// generate job ID & populate context
	jobID := jobcontext.GenerateJobID()

	jobCTX := jobcontext.JobContext{
		Target:                 "",
		Remote:                 (*remoteHost != ""),
		Docker:                 false,
		SkipLocal:              *skipLocal,
		JobID:                  jobID,
		StartTime:              time.Now(), // begin timer now
		TargetDir:              "",
		RootDir:                cargoportLocal,
		Tag:                    *tagOutputString,
		RestartDocker:          *restartDockerBool,
		RemoteHost:             string(*remoteHost),
		RemoteUser:             string(*remoteUser),
		CompressedSizeBytesInt: 0,
		CompressedSizeMBString: "0.0 MB",
	}

	// log & print job start
	logger.LogxWithFields("info", " --------------------------------------------------- ", map[string]interface{}{
		"package": "spacer",
		"job_id":  jobCTX.JobID,
	})

	// determine backup target
	targetPath, composeFilePath, dockerEnabled, err := backup.DetermineBackupTarget(&jobCTX, targetDir, dockerName)
	if err != nil {
		logger.LogxWithFields("fatal", fmt.Sprintf("Failure to determine backup target: %v", err), map[string]interface{}{
			"package": "main",
			"target":  filepath.Base(filepath.Dir(*targetDir)),
			"success": false,
			"docker":  true,
		})
	}
	jobCTX.Docker = dockerEnabled
	jobCTX.Target = filepath.Base(targetPath)
	jobCTX.TargetDir = targetPath

	// prepare local backupfile & compose
	backupFilePath, err := backup.PrepareBackupFilePath(&jobCTX, cargoportLocal, targetPath, *localOutputDir, *tagOutputString, *skipLocal)
	if err != nil {
		logger.LogxWithFields("fatal", fmt.Sprintf("Failure to determine output path: %v", err), map[string]interface{}{
			"package":  "main",
			"target":   filepath.Base(targetPath),
			"root_dir": cargoportLocal,
		})
	}

	/// << BEGIN JOB LOGIC >> (need to create jobhandler package)

	// define job context based on determined information thus far in the job process

	// --------------------
	coreFields := logger.CoreLogFields(&jobCTX, "main")
	verboseFields := mainLogDebugFields(&jobCTX)
	fatalFields := mainLogFatalFields(&jobCTX)

	logger.LogxWithFields("info", "New backup job added", map[string]interface{}{
		"package": "main",
		"target":  jobCTX.Target,
		"remote":  jobCTX.Remote,
		"docker":  jobCTX.Docker,
		"job_id":  jobCTX.JobID,
		//"tag":     jobCTX.Tag,
		"version": Version,
	})

	logger.LogxWithFields("debug", fmt.Sprintf("Beginning backup job via %s", jobCTX.TargetDir), verboseFields)

	// declare target base name for metrics and logging tracking
	targetBaseName := filepath.Base(targetPath)

	// handle pre-backup docker tasks
	if dockerEnabled {
		if err := docker.HandleDockerPreBackup(&jobCTX, composeFilePath, targetBaseName); err != nil {
			logger.LogxWithFields("fatal", fmt.Sprintf("Failure to perform pre-snapshot docker tasks: %v", err), fatalFields)
		}
	}

	// attempt compression of data; if fail && dockerEnabled then attempt to handle docker restart
	if err := backup.ShellCompressDirectory(&jobCTX, targetPath, backupFilePath); err != nil {

		// if docker restart fails, log error
		if dockerEnabled {
			if dockererr := docker.HandleDockerPostBackup(&jobCTX, composeFilePath, *restartDockerBool); dockererr != nil {
				logger.LogxWithFields("error", fmt.Sprintf("Failure to handle docker compose after backup: %v", dockererr), coreFields)
			}
		}

		logger.LogxWithFields("fatal", fmt.Sprintf("Failure to compress target: %v", err), fatalFields)
	}

	// handle remote transfer
	if *remoteHost != "" {
		err := remote.HandleRemoteTransfer(&jobCTX, backupFilePath, *remoteUser, *remoteHost, *remoteOutputDir, *skipLocal, *configFile)
		if err != nil {
			// if remote fail, then remove tempfile when skipLocal enabled
			if *skipLocal {
				sysutil.RemoveTempFile(&jobCTX, backupFilePath)
				logger.LogxWithFields("debug", fmt.Sprintf("Removing local tempfile %s", backupFilePath), verboseFields)

			}

			// if remote fail, then handle post-backup docker jobs
			if dockerEnabled {
				if err := docker.HandleDockerPostBackup(&jobCTX, composeFilePath, *restartDockerBool); err != nil {
					logger.LogxWithFields("fatal", fmt.Sprintf("Failure to reinitialize docker service after failed transfer: %v", err), fatalFields)
				}
			}
			logger.LogxWithFields("error", fmt.Sprintf("Failure to complete remote transfer: %v", err), verboseFields)
		}
	}

	//<section>   Post Backup/Restarts
	//------------

	// handle docker post backup
	if dockerEnabled {
		if err := docker.HandleDockerPostBackup(&jobCTX, composeFilePath, *restartDockerBool); err != nil {
			logger.LogxWithFields("error", fmt.Sprintf("Failure to restart docker service: %v", err), coreFields)
		}
	}

	// job completion banner & time calculation
	jobDuration := time.Since(jobCTX.StartTime)
	executionSeconds := jobDuration.Seconds()

	logger.LogxWithFields("info", fmt.Sprintf("Job success, execution time: %.2fs", executionSeconds), map[string]interface{}{
		"package":  "main",
		"target":   jobCTX.Target,
		"remote":   jobCTX.Remote,
		"docker":   jobCTX.Docker,
		"job_id":   jobCTX.JobID,
		"duration": fmt.Sprintf("%.2fs", executionSeconds),
		"success":  true,
		"size":     jobCTX.CompressedSizeMBString,
	})
	logger.LogxWithFields("info", " --------------------------------------------------- ", map[string]interface{}{
		"package":    "spacer",
		"end_job_id": jobCTX.JobID,
	})
}
