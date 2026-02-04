package input

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrian-griffin/cargoport/logger"
	"github.com/adrian-griffin/cargoport/util"
)

// sets up cargoport parent dirs & logging
func InitEnvironment(configFile ConfigFile) (string, string, string, string, string, string, string) {
	// initialize parent cargoport dirs on system
	var err error

	// create /var/cargoport/ directories on local machine
	cargoportBase := strings.TrimSuffix(configFile.DefaultCargoportDir, "/")
	cargoportLocal := filepath.Join(cargoportBase, "/local")
	cargoportRemote := filepath.Join(cargoportBase, "/remote")
	cargoportKeysBase := filepath.Join(cargoportBase, "/keys")
	cargoportSSHKeys := filepath.Join(cargoportKeysBase, "/ssh")
	cargoportAgeKeys := filepath.Join(cargoportKeysBase, "/age")
	cargoportMetrics := filepath.Join(cargoportBase, "/metrics")

	// create /$CARGOPORT/
	if err = os.MkdirAll(cargoportBase, 0755); err != nil {
		log.Fatalf("ERR: Error creating directory %s: %v", cargoportLocal, err)
	}

	// create /$CARGOPORT/local
	if err = os.MkdirAll(cargoportLocal, 0755); err != nil {
		log.Fatalf("ERR: Error creating directory %s: %v", cargoportLocal, err)
	}

	// create /$CARGOPORT/remote
	if err = os.MkdirAll(cargoportRemote, 0755); err != nil {
		log.Fatalf("ERR: Error creating directory %s: %v", cargoportRemote, err)
	}

	// create /$CARGOPORT/keys cargoportSSHKeys
	if err = os.MkdirAll(cargoportSSHKeys, 0755); err != nil {
		log.Fatalf("ERR: Error creating directory %s: %v", cargoportSSHKeys, err)
	}

	// create /$CARGOPORT/keys cargoportSSHKeys
	if err = os.MkdirAll(cargoportMetrics, 0755); err != nil {
		log.Fatalf("ERR: Error creating directory %s: %v", cargoportSSHKeys, err)
	}

	// initialize logging
	logFilePath := logger.InitLogging(cargoportBase, configFile.LogLevel, configFile.LogFormat, configFile.LogTextColour)

	return cargoportBase, cargoportLocal, cargoportRemote, logFilePath, cargoportSSHKeys, cargoportAgeKeys, cargoportMetrics
}

// guided setup tool for initial init
func SetupTool() {

	fmt.Println("|---- Cargoport Setup Wizard -----|")
	fmt.Println("|-~-~-~-~-~-~-~-~-~-~-~-~-~-~-~-~-|")
	fmt.Println("    Thanks for trying this out!")
	fmt.Println("                                 ")

	// prompt for root directory
	var rootDir string
	fmt.Println("Please specify the root directory for Cargoport's data & backup storage")
	fmt.Println("Leave blank for /var/cargoport, which works in most cases")
	fmt.Println(" ")
	fmt.Print("Root directory (default: /var/cargoport): ")
	fmt.Println(" ")
	fmt.Scanln(&rootDir)
	if rootDir == "" {
		rootDir = "/var/cargoport/"
	}
	fmt.Println(" ")
	fmt.Println("------")
	fmt.Println(" ")

	// ensure that passed directory name ends in cargoport, otherwise join cargoport onto it
	rootDir = strings.TrimSuffix(rootDir, "/")
	if !strings.HasSuffix(rootDir, "cargoport") {
		rootDir = filepath.Join(rootDir, "cargoport")
	}
	fmt.Printf("Using root dir: %s\n", rootDir)
	fmt.Println(" ")
	time.Sleep(500 * time.Millisecond) // forced slowdowns for readability

	// walk through temp configfile for setup & init
	configFile := ConfigFile{
		DefaultCargoportDir: rootDir,
		SkipLocal:           true,
		RemoteUser:          "",
		RemoteHost:          "",
		RemoteOutputDir:     filepath.Join(rootDir, "remote/"),
	}

	// init env and determine directories & logfile
	cargoportBase, cargoportLocal, cargoportRemote, logFilePath, cargoportSSHKeys, cargoportAgeKeys, cargoportMetrics := InitEnvironment(configFile)

	fmt.Println(" ")
	fmt.Println("------")
	fmt.Println(" ")
	time.Sleep(500 * time.Millisecond)

	// check for existing config.yml
	configFilePath := filepath.Join(cargoportBase, "config.yml")

	// if DNE then prompt to create default config
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		fmt.Println(" ")
		fmt.Println("Configfile Setup")
		fmt.Println("------")
		fmt.Println(" ")
		for {
			fmt.Printf("No config.yml found in %s. Would you like to create one? (y/n): ", cargoportBase)
			fmt.Printf("(Recommended for new users)")
			fmt.Println(" ")
			var createConfig string
			// scan for input
			fmt.Scanln(&createConfig)

			// switch to catch input choices
			switch strings.ToLower(createConfig) {
			case "y":
				err := createDefaultConfig(configFilePath, rootDir)
				if err != nil {
					log.Fatalf("ERROR: Failed to create config.yml %v", err)
				}
				fmt.Printf("Root directory initialized at: %s\n", cargoportBase)
				fmt.Printf("Local backup output directory: %s\n", cargoportLocal)
				fmt.Printf("Remote inbound storage directory: %s\n", cargoportRemote)
				fmt.Printf("SSH key storage: %s\n", cargoportSSHKeys)
				fmt.Printf("Age key storage: %s\n", cargoportAgeKeys)
				fmt.Printf("Metrics storage: %s\n", cargoportMetrics)
				fmt.Printf("Log file initialized at: %s\n", logFilePath)
				fmt.Printf("\n")
				fmt.Printf("Default config.yml created at %s", configFilePath)
				break

			case "n":
				log.Println("WARN <environment>: Skipping automatic configfile creation, please ensure you manually create a config.yml file!")
				break

			default:
				fmt.Println("Invalid input, please try again")
				continue // loop again to prompt for input
			}
			break
		}
	}
	fmt.Println(" ")

	fmt.Printf("\n")

	fmt.Println(" ")
	fmt.Println("SSH Key Information\n")
	fmt.Println("------\n")
	fmt.Println(" ")

	fmt.Printf("Cargoport creates and handles its own SSH keys for remote-transfers\n")
	fmt.Printf("Password prompt entry works when running cargoport manually, but if you want to utilize cron for backups on a schedule, SSH keys are required\n")
	fmt.Printf(" ")

	// create ssh key pair
	sshKeyName := "cargoport-ssh-id-ed25519"
	if err := util.GenerateSSHKeypair(cargoportSSHKeys, sshKeyName); err != nil {
		log.Fatalf("ERROR <util>: Failed to generate SSH key: %v", err)
	}

	fmt.Printf("\n")

	fmt.Println(" ")
	fmt.Println("Backup Encryption Information - Please Read\n")
	fmt.Println("------\n")
	fmt.Println(" ")

	fmt.Printf("When creating a backup, cargoport can encrypt the resulting compressed archive using age encryption\n")
	fmt.Printf("After encryption, backups can ONLY be restored if you have the correct decryption key (so don't lose it!)\n")
	fmt.Printf("When performing a backup with cargoport, pass the `-e` flag to encrypt.\n")

	fmt.Printf("\n")

	fmt.Printf("If you opt to encrypt any backups, please be sure to locate and SAVE the contents of `cargoport.agefile` (default `/var/cargoport/keys/age/cargoport.agefile`)\n")
	fmt.Printf("Recommended methods include using a password manager to save the contents, or copying this file to another, offline machine or USB\n")
	fmt.Printf("Although not required, it's recommended to remove the `cargoport.agefile` from this machine entirely to minimize blast radius if this machine is ever compromised\n")
	fmt.Printf("\n")
	fmt.Printf("If you never plan to use this feature, it can safely be ignored and no file needs to be removed\n")

	// prompt to create age key
	ageKeyName := "cargoport.agekey"
	if err := util.GenerateAgeKeypair(cargoportAgeKeys, ageKeyName); err != nil {
		log.Fatalf("ERROR <util>: Failed to generate Age encryption key: %v", err)
	}

	// save true config at /etc/ reference
	if err := saveTrueConfigReference(configFilePath); err != nil {
		log.Fatalf("ERROR: Failed to save true config reference: %v", err)
	}
	fmt.Println("------")
	fmt.Println(" ")
	time.Sleep(250 * time.Millisecond)

	fmt.Printf("\n")
	fmt.Printf("Intial setup completed, default config located at %s", configFilePath)
	fmt.Printf("\n")
	logger.LogxWithFields("info", "Environment setup completed successfully!", map[string]interface{}{
		"package": "environment",
		"success": true,
	})
	fmt.Println(" ")
}

// create default config and write to ./config.yml
func createDefaultConfig(configFilePath, rootDir string) error {
	// Template for default config.yml
	defaultConfig := fmt.Sprintf(`# [ LOCAL ]
## For your convenience, only change the default_cargoport_directory using the -setup flag
default_cargoport_directory: %s
default_output_directory: %s/local

## Skip all local backups from this machine by default, requires remote flags
skip_local_backups: false

# [ REMOTE TRANSFER ]
default_remote_user: admin
default_remote_host: 10.0.0.1

# If cargoport is also set up on the remote target machine(s), you may want to use this!
#   Otherwise use the default ~/ output
#default_remote_output_dir: %s/remote
default_remote_output_dir: ~/

# [ NETWORK ]
# These tests run before every remote transfer
# If you enable SSH tests, you will be prompted for the remote password twice until you copy the SSH key
icmp_test: true
ssh_test: false

# [ SSH KEYTOOL ]
ssh_key_directory: %s/keys/ssh
ssh_private_key_name: cargoport-ssh-id-ed25519

# [ LOGGING ]
# I'd recommend debug or info for most cases
log_level: info       # 'debug', 'info', 'warn', 'error', 'fatal'

# defines .log output type depending on taste
# json works well if you use jq with it
log_format: text        # 'json' or 'text'

# if 'text' format, logs will utilize ANSI codes for colouring
# great for readability, but makes casual log grepping harder without using looser matches
log_text_format_colouring: true

# [ METRICS ] 
metrics_dir: %s/metrics
# If per_job_metrics_server is enabled, or cargoport -metrics-daemon is run
# Then the prometheus /metrics endpoint will be available via this address:port
listen_address: 127.0.0.1
listen_port: 9101
# Optionally allow cargoport to run an http metrics endpoint after each job run
# This endpoint is exposed for a defined number of seconds and can be scraped to monitor trends
per_job_metrics_server: false # open http /metrics endpoint after each job attempt
listen_duration: 60 # expose http endpoint for 60s after job
# Interval between metrics exposure reloads when running in headless metrics daemon mode
# Only applies when -metrics-daemon flag is passed
metrics_daemon_reload_interval: 30 
`, rootDir, rootDir, rootDir, rootDir, rootDir)

	// Write default config file
	return os.WriteFile(configFilePath, []byte(defaultConfig), 0644)
}
