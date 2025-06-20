package input

import (
	"fmt"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/adrian-griffin/cargoport/util"
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
	LogFormat           string `yaml:"log_format"`
	LogTextColour       bool   `yaml:"log_text_format_colouring"`
}

// system-wide config reference path
const ConfigFilePointer = "/etc/.cargoport-pointerfile.conf"

// determines configfile path based on global pointerfile
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
	if err := util.ValidateDirectoryString(config.DefaultCargoportDir); err != nil {
		return nil, fmt.Errorf("invalid required config: default_cargoport_directory: %v", err)
	}

	// validate that SSH keydir is not empty, is valid, and writeable
	if config.SSHKeyDir == "" {
		return nil, fmt.Errorf("missing required config: ssh_key_directory")
	}
	if err := util.ValidateDirectoryWriteable(config.SSHKeyDir); err != nil {
		return nil, fmt.Errorf("invalid required config: ssh_key_directory: %v", err)
	}

	// validate that SSH key name is not empty
	if config.SSHKeyName == "" {
		return nil, fmt.Errorf("missing required config: ssh_private_key_name")
	}

	// if remote host not empy, validate that remote host is a valid IP address or DNS name
	if config.RemoteHost != "" {
		if err := util.ValidateIP(config.RemoteHost); err != nil {
			return nil, fmt.Errorf("invalid required config: default_remote_host: %v", err)
		}
	}

	// error if empty default_remote_user
	if config.RemoteUser == "" {
		return nil, fmt.Errorf("invalid `default_remote_user` in configfile")
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
		log.Printf("invalid `log_level` supplied, defaulting to `info`")
		config.LogLevel = "info"
	}

	// validate log_format
	// warn if invalid, default to "text"
	validLogFormats := map[string]bool{
		"text": true,
		"json": true,
	}
	// walk map, if no keys match valid log formats then warn & set config.LogFormat to `text`
	if !validLogFormats[config.LogFormat] {
		log.Printf("invalid `log_format` supplied, defaulting to `text`")
		config.LogFormat = "text"
	}

	return &config, nil
}

// handles writes between true configfile at /etc/ an configfile reference in declared parent dir
func saveTrueConfigReference(configFilePath string) error {
	return os.WriteFile(ConfigFilePointer, []byte(configFilePath), 0644)
}
