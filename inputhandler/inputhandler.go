package inputhandler

import (
	"fmt"
	"log"
	"net"

	"github.com/adrian-griffin/cargoport/environment"
)

// handles user input validation
func validateInput(targetDir, dockerName, remoteUser, remoteHost, remoteOutputDir *string, skipLocal *bool, configFile environment.ConfigFile) error {

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
func InterpretFlags(
	targetDir, dockerName, localOutputDir *string,
	skipLocal *bool,
	remoteUser, remoteHost, remoteOutputDir *string,
	sendDefaults *bool,
	configFile environment.ConfigFile,
) {
	// validate or override flags with configfile values

	// if send default enabled
	if *sendDefaults {
		// & remote user is not empty
		if configFile.RemoteUser != "" {
			*remoteUser = configFile.RemoteUser
		}
		// & remote host is not empty
		if configFile.RemoteHost != "" {
			*remoteHost = configFile.RemoteHost
		}

		// if either remote user or remote host are empty
		if configFile.RemoteUser == "" || configFile.RemoteHost == "" {
			log.Fatalf("ERROR <config>: Default remote host and username must be set in the configuration file to use -remote-send-defaults.")
		}
	}
	// validate inputs
	err := validateInput(targetDir, dockerName, remoteUser, remoteHost, remoteOutputDir, skipLocal, configFile)
	if err != nil {
		log.Fatalf("ERROR <input>: %v", err)
	}
}
