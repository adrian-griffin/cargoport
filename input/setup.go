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
func InitEnvironment(configFile ConfigFile) (string, string, string, string, string) {
	// initialize parent cargoport dirs on system
	var err error

	// create /var/cargoport/ directories on local machine
	cargoportBase := strings.TrimSuffix(configFile.DefaultCargoportDir, "/")
	cargoportLocal := filepath.Join(cargoportBase, "/local")
	cargoportRemote := filepath.Join(cargoportBase, "/remote")
	cargoportKeys := filepath.Join(cargoportBase, "/keys")

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

	// create /$CARGOPORT/keys cargoportKeys
	if err = os.MkdirAll(cargoportKeys, 0755); err != nil {
		log.Fatalf("ERR: Error creating directory %s: %v", cargoportKeys, err)
	}

	// set 777 on /var/cargoport/remote for all users to access
	err = util.RunCommand("chmod", "-R", "777", cargoportRemote)
	if err != nil {
		log.Fatalf("ERR: Error setting %s permissions for remotewrite: %v", cargoportRemote, err)
	}

	// initialize logging
	logFilePath := logger.InitLogging(cargoportBase, configFile.LogLevel, configFile.LogFormat, configFile.LogTextColour)

	return cargoportBase, cargoportLocal, cargoportRemote, logFilePath, cargoportKeys
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
	cargoportBase, cargoportLocal, cargoportRemote, logFilePath, cargoportKeys := InitEnvironment(configFile)

	fmt.Printf("Root directory initialized at: %s\n", cargoportBase)
	fmt.Printf("Local backup directory: %s\n", cargoportLocal)
	fmt.Printf("Remote backup directory: %s\n", cargoportRemote)
	fmt.Printf("util storage: %s\n", cargoportKeys)
	fmt.Printf("Log file initialized at: %s\n", logFilePath)

	fmt.Println(" ")
	fmt.Println("------")
	fmt.Println(" ")
	time.Sleep(500 * time.Millisecond)

	// check for existing config.yml
	configFilePath := filepath.Join(cargoportBase, "config.yml")

	// if DNE then prompt to create default config
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		for {
			fmt.Printf("No config.yml found in %s. Would you like to create one? (y/n): ", cargoportBase)
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
	time.Sleep(250 * time.Millisecond)

	// create ssh key pair
	sshKeyName := "cargoport-id-ed25519"
	if err := util.GenerateSSHKeypair(cargoportKeys, sshKeyName); err != nil {
		log.Fatalf("ERROR <util>: Failed to generate SSH key: %v", err)
	}

	// save true config at /etc/ reference
	if err := saveTrueConfigReference(configFilePath); err != nil {
		log.Fatalf("ERROR: Failed to save true config reference: %v", err)
	}
	fmt.Println("------")
	fmt.Println(" ")
	time.Sleep(250 * time.Millisecond)

	logger.LogxWithFields("info", "Environment setup completed successfully!", map[string]interface{}{
		"package": "environment",
		"success": true,
	})
	fmt.Println(" ")
}

// create default config and write to ./config.yml
func createDefaultConfig(configFilePath, rootDir string) error {
	// Template for default config.yml
	defaultConfig := fmt.Sprintf(`# [ LOCAL DEFAULTS ]
## For your convenience, only change the default_cargoport_directory using the -setup flag
default_cargoport_directory: %s
default_output_directory: %s/local

## Skip all local backups from this machine by default, requires remote flags
skip_local_backups: false

# [ REMOTE TRANSFER DEFAULTS]
default_remote_user: admin
default_remote_host: 10.0.0.1

# If cargoport is also set up on the remote target machine(s), you may want to use this!
#   Otherwise use the default ~/ output
#default_remote_output_dir: %s/remote
default_remote_output_dir: ~/

# [ NETWORK SETTINGS ]
# These tests run before every remote transfer
# If you enable SSH tests, you will be prompted for the remote password twice until you copy the SSH key
icmp_test: true
ssh_test: false

# [ SSH KEYTOOL DEFAULTS ]
ssh_key_directory: %s/keys
ssh_private_key_name: cargoport-id-ed25519

# [ LOGGING ]
# I'd recommend debug or info for most cases
log_level: info       # 'debug', 'info', 'warn', 'error', 'fatal'

# defines .log output type depending on taste
# json works well if you use jq with it
log_format: text        # 'json' or 'text'

# if 'text' format, logs will utilize ANSI codes for colouring
# great for readability, but makes casual log grepping harder without using looser matches
log_text_format_colouring: true
`, rootDir, rootDir, rootDir, rootDir)

	// Write default config file
	return os.WriteFile(configFilePath, []byte(defaultConfig), 0644)
}
