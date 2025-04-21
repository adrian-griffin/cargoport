package environment

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/adrian-griffin/cargoport/keytool"
	"github.com/adrian-griffin/cargoport/nethandler"
	"github.com/adrian-griffin/cargoport/sysutil"
)

type ConfigFile struct {
	DefaultCargoportDir string `yaml:"default_cargoport_directory"`
	SkipLocal           bool   `yaml:"skip_local_backups"`
	RemoteUser          string `yaml:"default_remote_user"`
	RemoteHost          string `yaml:"default_remote_host"`
	RemoteOutputDir     string `yaml:"default_remote_output_dir"`
	Version             string `yaml:"version,omitempty"`
	SSHKeyDir           string `yaml:"ssh_key_directory"`
	SSHKeyName          string `yaml:"ssh_key_name"`
}

// system-wide config reference path
const ConfigFilePointer = "/etc/cargoport.conf"

// defines log & stdout styling and content at start of backups
func LogStart(format string, args ...interface{}) {
	log.Println("-------------------------------------------------------------------------")
	log.Printf(format, args...)
	log.Println("-------------------------------------------------------------------------")
	fmt.Println("-------------------------------------------------------------------------")
	fmt.Printf(format, args...)
	fmt.Println("-------------------------------------------------------------------------")
}

// defines log & stdout styling and content at end of backups
func LogEnd(format string, args ...interface{}) {

	log.Println("-------------------------------------------------------------------------")
	log.Printf(format, args...)
	log.Println("-------------------------------------------------------------------------")
	fmt.Println("-------------------------------------------------------------------------")
	fmt.Printf(format, args...)
	fmt.Println("-------------------------------------------------------------------------")
}

// determines true configfile path
func GetConfigFilePath() (string, error) {
	// opens configfile pointer file to reference path to yamlfile
	pointerFileData, err := os.ReadFile(ConfigFilePointer)
	if err != nil {
		return "", fmt.Errorf("could not read %s: %v", ConfigFilePointer, err)
	}
	// strings data from pointerfile and gathers path location
	trueConfigPath := strings.TrimSpace(string(pointerFileData))
	if _, err := os.Stat(trueConfigPath); os.IsNotExist(err) {
		return "", fmt.Errorf("config file in path %s does not exist", trueConfigPath)
	}
	return trueConfigPath, nil
}

// parse config file
func LoadConfigFile(configFilePath string) (*ConfigFile, error) {
	// read config data from config file
	configFileData, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// unmarshal yaml into configfile var
	var config ConfigFile
	if err := yaml.Unmarshal(configFileData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	//> CFG FILE VALIDATIONS
	// validate that default_cargoport_directory is defined & valid
	if config.DefaultCargoportDir == "" {
		return nil, fmt.Errorf("missing required config: default_cargoport_directory")
	}
	if err := sysutil.ValidateDirectoryString(config.DefaultCargoportDir); err != nil {
		return nil, fmt.Errorf("invalid path defined: default_cargoport_directory")
	}

	// validate that SSH keydir is not empty & valid
	if config.SSHKeyDir == "" {
		return nil, fmt.Errorf("missing required config: ssh_key_directory")
	}
	if err := sysutil.ValidateDirectoryString(config.SSHKeyDir); err != nil {
		return nil, fmt.Errorf("invalid path defined: ssh_key_directory")
	}

	// validate that SSH key name is not empty
	if config.SSHKeyName == "" {
		return nil, fmt.Errorf("missing required config: ssh_key_name")
	}

	// validate that SSH privkey exists when cfg loaded
	sshPrivateKeyPath := filepath.Join(config.SSHKeyDir, config.SSHKeyName)
	if _, err := os.Stat(sshPrivateKeyPath); err != nil {
		return nil, fmt.Errorf("defined SSH private key path invalid")
	}

	if err := keytool.ValidateSSHPrivateKeyPerms(sshPrivateKeyPath); err != nil {
		return nil, fmt.Errorf("private SSH key failed integrity check, key may have been tampered with, please generate a new keypair")
	}

	// if remote host not empy, validate that remote host is a valid IP address or DNS name
	if config.RemoteHost != "" {
		if err := nethandler.ValidateIP(config.RemoteHost); err != nil {
			return nil, fmt.Errorf("invalid required config: default_remote_host")
		}
	}

	return &config, nil
}

// sets up cargoport parent dirs & logging
func InitEnvironment(configFile ConfigFile) (string, string, string, string, string) {
	// initialize parent cargoport dirs on system
	var err error

	// Create /var/cargoport/ directories on local machine
	cargoportBase := strings.TrimSuffix(configFile.DefaultCargoportDir, "/")
	cargoportLocal := filepath.Join(cargoportBase, "/local")
	cargoportRemote := filepath.Join(cargoportBase, "/remote")
	cargoportKeys := filepath.Join(cargoportBase, "/keys")

	// create /$CARGOPORT/
	if err = os.MkdirAll(cargoportBase, 0755); err != nil {
		log.Fatalf("ERROR <environment>: Error creating directory %s: %v", cargoportLocal, err)
	}
	// create /$CARGOPORT/local
	if err = os.MkdirAll(cargoportLocal, 0755); err != nil {
		log.Fatalf("ERROR <environment>: Error creating directory %s: %v", cargoportLocal, err)
	}
	// create /$CARGOPORT/remote
	if err = os.MkdirAll(cargoportRemote, 0755); err != nil {
		log.Fatalf("ERROR <environment>: Error creating directory %s: %v", cargoportRemote, err)
	}
	// create /$CARGOPORT/keys cargoportKeys
	if err = os.MkdirAll(cargoportKeys, 0755); err != nil {
		log.Fatalf("ERROR <environment>: Error creating directory %s: %v", cargoportKeys, err)
	}
	// set 777 on /var/cargoport/remote for all users to access
	err = sysutil.RunCommand("chmod", "-R", "777", cargoportRemote)
	if err != nil {
		log.Fatalf("ERROR <environtment>: Error setting %s permissions for remotewrite: %v", cargoportRemote, err)
	}

	// initialize logging
	logFilePath := initLogging(cargoportBase)

	return cargoportBase, cargoportLocal, cargoportRemote, logFilePath, cargoportKeys
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

// guided setup tool for initial init
func SetupTool() {
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

	// walk through temp configfile for setup & init
	configFile := ConfigFile{
		DefaultCargoportDir: rootDir,
		SkipLocal:           true,
		RemoteUser:          "",
		RemoteHost:          "",
		RemoteOutputDir:     filepath.Join(rootDir, "remote/"),
	}

	// detect if setup already exists
	//

	// init env and determine directories & logfile
	cargoportBase, cargoportLocal, cargoportRemote, logFilePath, cargoportKeys := InitEnvironment(configFile)

	// print new dir and logfile information
	fmt.Printf("Base directory initialized at: %s\n", cargoportBase)
	fmt.Printf("Local backup directory: %s\n", cargoportLocal)
	fmt.Printf("Remote backup directory: %s\n", cargoportRemote)
	fmt.Printf("Log file initialized at: %s\n", logFilePath)
	fmt.Printf("Key storage initialized at: %s\n", cargoportKeys)

	// check for existing config.yml
	configFilePath := filepath.Join(cargoportBase, "config.yml")

	// if DNE then prompt to create default config
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

	// create ssh key pair
	sshKeyName := "cargoport_id_ed25519"
	if err := keytool.GenerateSSHKeypair(cargoportKeys, sshKeyName); err != nil {
		log.Fatalf("ERROR <keytool>: Failed to generate SSH key: %v", err)
	}

	// save true config at /etc/ reference
	if err := saveTrueConfigReference(configFilePath); err != nil {
		log.Fatalf("ERROR: Failed to save true config reference: %v\n", err)
	}

	fmt.Println("Environment setup completed successfully.")
}

// create default config and write to ./config.yml
func createDefaultConfig(configFilePath, rootDir string) error {
	// Template for default config.yml
	defaultConfig := fmt.Sprintf(`# [ LOCAL DEFAULTS ]
## Cargoport Root Directory
## Please change default dir using -setup flag
default_cargoport_directory: %s

## Skip all local backups unless otherwise specified (-skip-local=false for local jobs)
skip_local_backups: false

# [ REMOTE TRANSFER DEFAULTS]
default_remote_user: admin
default_remote_host: 10.0.0.1
default_remote_output_dir: %s/remote

# [ KEYTOOL DEFAULTS ]
ssh_key_directory: %s/keys
ssh_key_name: cargoport_id_ed25519
`, rootDir, rootDir, rootDir)

	// Write default config file
	return os.WriteFile(configFilePath, []byte(defaultConfig), 0644)
}

// handles writes between true configfile at /etc/ an configfile reference in declared parent dir
func saveTrueConfigReference(configFilePath string) error {
	return os.WriteFile(ConfigFilePointer, []byte(configFilePath), 0644)
}
