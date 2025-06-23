package main

// Cargoport

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/adrian-griffin/cargoport/input"
	"github.com/adrian-griffin/cargoport/job"
	"github.com/adrian-griffin/cargoport/logger"
	"github.com/adrian-griffin/cargoport/meta"
	"github.com/adrian-griffin/cargoport/metrics"
	"github.com/adrian-griffin/cargoport/runner"
	"github.com/adrian-griffin/cargoport/util"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// debug level logging output fields for main package
func mainLogDebugFields(context *job.JobContext) map[string]interface{} {
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
func mainLogFatalFields(context *job.JobContext) map[string]interface{} {
	debugFields := mainLogDebugFields(context)
	fields := logger.MergeFields(debugFields, map[string]interface{}{
		"success": false,
	})

	return fields
}

// main loop
func main() {
	// version & setup flags
	appVersion := flag.Bool("version", false, "Display app version information")
	setupBool := flag.Bool("setup", false, "Run setup utility")

	// core job flags
	targetDir := flag.String("target-dir", "", "Target directory to back up (detects if the directory is a Docker environment)")
	dockerName := flag.String("docker-name", "", "Target Docker service name (involves all Docker containers defined in the compose file)")
	localOutputDir := flag.String("output-dir", "", "Custom destination for local output")
	restartDockerBool := flag.Bool("restart-docker", true, "Restart docker container after successful backup. Enabled by default")
	tagOutputString := flag.String("tag", "", "Append identifying tag to output file name (e.g: service1-<tag>.bak.tar.gz)")

	// remote transfer flags
	skipLocal := flag.Bool("skip-local", false, "Skip local backup & only send to remote target")
	remoteUser := flag.String("remote-user", "", "Remote machine username")
	remoteHost := flag.String("remote-host", "", "Remote machine IP(v4/v6) address or hostname")
	remoteOutputDir := flag.String("remote-dir", "", "Remote target directory (file saved as <remote-dir>/<file>.bak.tar.gz)")
	sendDefaults := flag.Bool("remote-send-defaults", false, "Toggles remote send functionality using configfile default creds, overrides remote-user and remote-host flags")

	// ssh key flags
	newSSHKeyBool := flag.Bool("generate-keypair", false, "Generate new SSH key for cargoport")
	copySSHKeyBool := flag.Bool("copy-key", false, "Copy cargoport SSH key to remote host")

	// metrics flags
	metricsDaemon := flag.Bool("metrics-daemon", false, "Run as a persistent headless metrics http endpoint for monitoring cargoport")

	// custom help messaging
	flag.Usage = func() {
		fmt.Println("------------------------------------------------------------------------")
		fmt.Printf("cargoport %s  ~  %s\n", meta.Version, meta.MOTD)
		fmt.Println("-------------------------------------------------------------------------")
		fmt.Println("[Options]")
		fmt.Println("  [Setup & Info]")
		fmt.Println("     -setup")
		fmt.Println("        Run setup utility to init the cargoport environment (default is /var/cargoport/)")
		fmt.Println("     -version")
		fmt.Println("        Display app version information")
		fmt.Println("\n  [SSH Key Flags]")
		fmt.Println("     -copy-key")
		fmt.Println("        Copy public key to remote machine, must be passed with explicit remote-host & remote-user")
		fmt.Println("     -generate-keypair")
		fmt.Println("        Generate a new set of SSH keys based on name & location defined in config")
		fmt.Println("\n  [Main Backup Flags]")
		fmt.Println("      [Target Selection Flags]")
		fmt.Println("        -target-dir <dir>")
		fmt.Println("           Target directory to back up (detects if the directory is a Docker environment)")
		fmt.Println("        -docker-name <name>")
		fmt.Println("           Target Docker service name (involves all Docker containers defined in the compose file)")
		fmt.Println("\n    [Extra Job Flags]")
		fmt.Println("        -output-dir <dir>")
		fmt.Println("           Custom destination for local output")
		fmt.Println("        -restart-docker <bool>")
		fmt.Println("           Restart docker container after successful backup. Enabled by default")
		fmt.Println("        -tag <tag>")
		fmt.Println("           Append identifying tag to output file name (e.g: service1-<tag>.bak.tar.gz)")
		fmt.Println("\n  [Remote Transfer Flags]")
		fmt.Println("      -skip-local")
		fmt.Println("         Skip local backup and only send to the remote target (Note: utilized `/tmp`)")
		fmt.Println("      -remote-user <user>")
		fmt.Println("         Remote machine username")
		fmt.Println("      -remote-host <host>")
		fmt.Println("         Remote machine IP(v4/v6) address or hostname")
		fmt.Println("      -remote-dir <dir>")
		fmt.Println("         Remote target directory (file will save as <remote-dir>/<file>.bak.tar.gz)")
		fmt.Println("      -remote-send-defaults")
		fmt.Println("         Remote transfer backup using default remote values in config.yml")

		fmt.Println("\n[Examples]")
		fmt.Println("  First time setup")
		fmt.Println("    cargoport -setup")
		fmt.Println("\n  Copy SSH key to remote machine")
		fmt.Println("    cargoport -copy-key -remote-host <host> -remote-user <username>")
		fmt.Println("\n  Perform compressive backup of target directory")
		fmt.Println("    cargoport -target-dir=/path/to/dir -remote-user=admin -remote-host=<host>")
		fmt.Println("\n  Perform compressive backup of target docker container(s) by service name")
		fmt.Println("    cargoport -docker-name=container-name -remote-send-defaults -skip-local")
		fmt.Println("    cargoport -docker-name=container-name -tag='pre-pull' -restart-docker=false")

		fmt.Println("\nFor more information, please check out the git repo readme <3")
	}

	flag.Parse()

	// special flags
	if *appVersion {
		fmt.Printf("cargoport  ~  %s\n", meta.MOTD)
		fmt.Printf("version: %s", meta.Version)
		os.Exit(0)
	}

	// validate that current UID=0/program is running as root
	if os.Geteuid() != 0 {
		fmt.Println("Please run Cargoport with sudo or as the root user")
		fmt.Println("This is required to access Docker volumes, manage SSH keys, and write to system directories")
		fmt.Println("For details and security considerations, see the GitHub README <3")
		os.Exit(0)
	}

	// if setup flag passed
	if *setupBool {
		input.SetupTool()
		os.Exit(0)
	}

	// load configfile
	configFile, err := input.LoadConfigFile()
	if err != nil {
		log.Printf("Error parsing config: %v", err)
		log.Fatalf("Perhaps consider running cargoport -setup again")
	}

	// init logging
	logger.InitLogging(configFile.DefaultCargoportDir, configFile.LogLevel, configFile.LogFormat, configFile.LogTextColour)

	// build input context
	inputCTX := &input.InputContext{
		TargetDir:        *targetDir,
		DockerName:       *dockerName,
		OutputDir:        *localOutputDir,
		RestartDocker:    *restartDockerBool,
		SkipLocal:        *skipLocal,
		RemoteUser:       *remoteUser,
		RemoteHost:       *remoteHost,
		RemoteOutputDir:  *remoteOutputDir,
		SendDefaults:     *sendDefaults,
		Tag:              *tagOutputString,
		CopySSHKey:       *copySSHKeyBool,
		GenerateSSHKey:   *newSSHKeyBool,
		MetricsDaemon:    *metricsDaemon,
		DefaultOutputDir: configFile.DefaultCargoportDir,
		Config:           configFile,
	}
	// interpret flags & handle config overrides
	if err := input.ValidateInputs(inputCTX); err != nil {
		logger.LogxWithFields("fatal", fmt.Sprintf("Failure to parse input: %v", err), map[string]interface{}{
			"package": "main",
			"target":  filepath.Base(*targetDir),
			"success": false,
		})
	}

	// handle ssh key generation
	if inputCTX.GenerateSSHKey {
		sshKeyDir := configFile.SSHKeyDir
		sshKeyName := configFile.SSHKeyName
		if err := util.GenerateSSHKeypair(sshKeyDir, sshKeyName); err != nil {
			logger.Logx.Fatalf("Failure generating SSH key: %v", err)
		}
		os.Exit(0)
	}

	// copy public key to remote machine if passed
	if inputCTX.CopySSHKey {
		sshPrivKeypath := filepath.Join(configFile.SSHKeyDir, configFile.SSHKeyName)
		if err := util.CopyPublicKey(sshPrivKeypath, *remoteUser, *remoteHost); err != nil {
			logger.Logx.Errorf("Failure copying SSH public key: %v", err)
		}
		os.Exit(0)
	}

	// validate permissions & integrity on private key
	sshPrivateKeyPath := filepath.Join(configFile.SSHKeyDir, configFile.SSHKeyName)
	if err := util.ValidateSSHPrivateKeyPerms(sshPrivateKeyPath); err != nil {
		logger.Logx.Error("Failure validating keypair, please check configfile or create a new pair")
		logger.Logx.Fatalf("Key validation error: %v", err)
	}

	if inputCTX.MetricsDaemon {

		// spawn goroutine to reload from cache .json files to push new metrics updates to http interface
		go func() {
			reloadInterval := inputCTX.Config.MetricsDaemonReloadInterval
			if reloadInterval <= 0 {
				reloadInterval = 60
				logger.Logx.Warnf("Invalid or unset reload interval; defaulting to %ds", reloadInterval)
			}
			ticker := time.NewTicker(time.Duration(reloadInterval) * time.Second)

			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					job, env, err := metrics.ReadJSONMetrics(configFile)
					if err != nil {
						logger.Logx.Errorf("Failed to reload metrics from JSON: %v", err)
						continue
					}
					metrics.ApplyPrometheusMetrics(job, env)
					logger.Logx.Debugf("Reloaded metrics from disk")
				}
			}
		}()

		if err := metrics.LoadFromCacheAndExpose(configFile); err != nil {
			logger.Logx.Fatalf("Failed to load metrics from cache: %v", err)
		}

		logger.Logx.Infof("Starting persistent metrics daemon at http://%s:%s/metrics", inputCTX.Config.ListenAddress, inputCTX.Config.ListenSocket)
		logger.Logx.Infof("Reload interval from cache is %d", inputCTX.Config.MetricsDaemonReloadInterval)
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(inputCTX.Config.ListenAddress+":"+inputCTX.Config.ListenSocket, nil)
		return
	}

	// declare empty metrics structs
	jobMetricsData := metrics.NewJobMetrics()
	envMetricsData := metrics.NewEnvMetrics()

	// run backup job
	if jobctx, err := runner.RunJob(inputCTX); err != nil {
		jobMetricsData.SetLastJobMetrics(false, 0, 0)
		logger.Logx.Errorf("Job failed: %v", err)
	} else {
		outputSize := jobctx.CompressedSizeBytesInt
		jobDuration := jobctx.JobDuration
		jobMetricsData.SetLastJobMetrics(true, outputSize, jobDuration)
	}

	// assign env metrics
	envMetricsData.SetLocalDirSize(inputCTX.DefaultOutputDir)
	envMetricsData.SetRemoteDirSize(filepath.Join(inputCTX.RootDir, "/remote"))
	envMetricsData.SetLocalFileCount(inputCTX.DefaultOutputDir)

	// write newly collected metrics to disk
	metrics.WriteMetricsFiles(inputCTX.Config, *jobMetricsData, *envMetricsData)

	// spin up metrics http server if enabled via config
	if inputCTX.Config.PerJobMetricsServer {

		// check if http listener already active via daemon process
		listenAddress := inputCTX.Config.ListenAddress
		listenSocket := inputCTX.Config.ListenSocket
		endpointSocket := fmt.Sprintf("%s:%s", listenAddress, listenSocket)
		ln, err := net.Listen("tcp", endpointSocket)
		if err != nil {
			logger.LogxWithFields("debug", fmt.Sprintf("Metrics endpoint already listening at %s, skipping temp prometheus server", endpointSocket), map[string]interface{}{
				"package": "metrics",
			})
			return
		}
		ln.Close()

		listenDuration := inputCTX.Config.ListenDuration
		logger.LogxWithFields("info", fmt.Sprintf("Starting metrics endpoint on http://%s:%s/metrics for %ds", listenAddress, listenSocket, listenDuration), map[string]interface{}{
			"package": "metrics",
		})
		metrics.StartMetricsServer(fmt.Sprintf("%s:%s", listenAddress, listenSocket), time.Duration(listenDuration)*time.Second)

	}

}
