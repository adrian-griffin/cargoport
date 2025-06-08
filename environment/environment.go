package environment

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/adrian-griffin/cargoport/keytool"
	"github.com/adrian-griffin/cargoport/logger"
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
	SSHKeyName          string `yaml:"ssh_private_key_name"`
	ICMPTest            bool   `yaml:"icmp_test"`
	SSHTest             bool   `yaml:"ssh_test"`
	LogLevel            string `yaml:"log_level"`
	LogFormat           string `yaml:"log_type"`
}

// system-wide config reference path
const ConfigFilePointer = "/etc/cargoport.conf"

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
		return nil, fmt.Errorf("invalid required config: default_cargoport_directory: %v", err)
	}

	// validate that SSH keydir is not empty, is valid, and writeable
	if config.SSHKeyDir == "" {
		return nil, fmt.Errorf("missing required config: ssh_key_directory")
	}
	if err := sysutil.ValidateDirectoryWriteable(config.SSHKeyDir); err != nil {
		return nil, fmt.Errorf("invalid required config: ssh_key_directory: %v", err)
	}

	// validate that SSH key name is not empty
	if config.SSHKeyName == "" {
		return nil, fmt.Errorf("missing required config: ssh_private_key_name")
	}

	// if remote host not empy, validate that remote host is a valid IP address or DNS name
	if config.RemoteHost != "" {
		if err := nethandler.ValidateIP(config.RemoteHost); err != nil {
			return nil, fmt.Errorf("invalid required config: default_remote_host: %v", err)
		}
	}

	// error if empty default_remote_user
	if config.RemoteUser == "" {
		fmt.Errorf("invalid `default_remote_user` in configfile")
	}

	// validate log_level
	// warn if invalid, default to "info"
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
	}

	// walk map, if no keys match valid log levels then warn & set config.LogLevel to `info`
	if !validLogLevels[config.LogLevel] {
		logger.LogxWithFields("warn", "invalid `log_level` supplied, defaulting to `info`", map[string]interface{}{
			"package": "environment",
		})
		config.LogLevel = "info"
	}

	// validate log_type
	// warn if invalid, default to "text"
	validLogFormats := map[string]bool{
		"text": true,
		"json": true,
	}
	// walk map, if no keys match valid log formats then warn & set config.LogFormat to `text`
	if !validLogFormats[config.LogFormat] {
		logger.LogxWithFields("warn", "invalid `log_format` supplied, defaulting to `text`", map[string]interface{}{
			"package": "environment",
		})
	}

	return &config, nil
}

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
	err = sysutil.RunCommand("chmod", "-R", "777", cargoportRemote)
	if err != nil {
		log.Fatalf("ERR: Error setting %s permissions for remotewrite: %v", cargoportRemote, err)
	}

	// initialize logging
	logFilePath := logger.InitLogging(cargoportBase, configFile.LogLevel, configFile.LogFormat)

	return cargoportBase, cargoportLocal, cargoportRemote, logFilePath, cargoportKeys
}

// guided setup tool for initial init
func SetupTool() {

	// validate that current UID=0 during setuptool process
	if os.Geteuid() != 0 {
		log.Fatalf("The -setup command must be run as root (e.g. with sudo).")
	}

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
	time.Sleep(1 * time.Second) // forced slowdowns for readability

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
	fmt.Printf("Keytool storage: %s\n", cargoportKeys)
	fmt.Printf("Log file initialized at: %s\n", logFilePath)

	fmt.Println(" ")
	fmt.Println("------")
	fmt.Println(" ")
	time.Sleep(2 * time.Second)

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
	time.Sleep(1 * time.Second)

	// create ssh key pair
	sshKeyName := "cargoport-id-ed25519"
	if err := keytool.GenerateSSHKeypair(cargoportKeys, sshKeyName); err != nil {
		log.Fatalf("ERROR <keytool>: Failed to generate SSH key: %v", err)
	}

	// save true config at /etc/ reference
	if err := saveTrueConfigReference(configFilePath); err != nil {
		log.Fatalf("ERROR: Failed to save true config reference: %v", err)
	}
	fmt.Println("------")
	fmt.Println(" ")
	time.Sleep(500 * time.Millisecond)

	logger.LogxWithFields("info", fmt.Sprintf("Environment setup completed successfully!"), map[string]interface{}{
		"package": "environment",
		"success": true,
	})
	fmt.Println(" ")
}

// create default config and write to ./config.yml
func createDefaultConfig(configFilePath, rootDir string) error {
	// Template for default config.yml
	defaultConfig := fmt.Sprintf(`# [ LOCAL DEFAULTS ]
## Please only change this default directory using -setup flag
default_cargoport_directory: %s

## Skip all local backups from this machine by default, requires remote flags
skip_local_backups: false

# [ REMOTE TRANSFER DEFAULTS]
default_remote_user: admin
default_remote_host: 10.0.0.1

# If cargoport is also set up on the remote target machine(s), you may want use this!
#   Otherwise use the default ~/ output
default_remote_output_dir: %s/remote
# default_remote_output_dir: ~/

# [ NETWORK SETTINGS ]
# These tests run before every remote transfer
# If you enable SSH tests, you will be prompted for the remote password twice unless you copy the SSH key
icmp_test: true
ssh_test: false

# [ KEYTOOL DEFAULTS ]
ssh_key_directory: %s/keys
ssh_private_key_name: cargoport-id-ed25519

# [ LOGGING ]
# I'd recommend debug or info for most cases
log_level: info       # 'debug', 'info', 'warn', 'error', 'fatal'

# defines .log output type depending on your taste
# json works well if you use jq with it
log_type: text        # 'json' or 'text'
`, rootDir, rootDir, rootDir)

	// Write default config file
	return os.WriteFile(configFilePath, []byte(defaultConfig), 0644)
}

// handles writes between true configfile at /etc/ an configfile reference in declared parent dir
func saveTrueConfigReference(configFilePath string) error {
	return os.WriteFile(ConfigFilePointer, []byte(configFilePath), 0644)
}
