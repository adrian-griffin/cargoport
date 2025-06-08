package inputhandler

import (
	"fmt"
	"path/filepath"

	"github.com/adrian-griffin/cargoport/environment"
	"github.com/adrian-griffin/cargoport/logger"
	"github.com/adrian-griffin/cargoport/nethandler"
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

	//<subsection>   Validate Backup Target Flags
	///-----------
	// if `skipLocal` is true, ensure remote send is configured
	if *skipLocal && (*remoteHost == "" || *remoteUser == "") {
		return fmt.Errorf("when skipping local backup, please ensure that the necessary remote flags are passed")
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
) (err error) {
	// validate or override flags with configfile values

	// determine if job is intended to perform skip local
	skipLocalBool := configFile.SkipLocal || *skipLocal
	if skipLocalBool {
		if *skipLocal == false {
			*skipLocal = false
		} else {
			*skipLocal = true
		}
	}

	// determine if job is intended to involve remote transfer
	remoteTransferBool := *sendDefaults || *remoteHost != "" || *remoteUser != ""
	if remoteTransferBool {

		// > Ensure that both remote host & remote user are passed when one is passed
		// if either remote host or remote user are passed
		if *remoteHost != "" || *remoteUser != "" {
			// if NOT both remote host and remote user are passed, return err
			if *remoteHost == "" || *remoteUser == "" {
				return fmt.Errorf("both `-remote-host` and `-remote-user` must be passed")
			}
		}

		// validate `remoteHost` a valid IP address or hostname & ICMP ping it if enabled in config
		if *remoteHost != "" {
			if err := nethandler.ValidateIP(*remoteHost); err != nil {
				return fmt.Errorf("remote host validation error: %v", err)
			}
			if configFile.ICMPTest {
				if err := nethandler.ICMPRemoteHost(*remoteHost); err != nil {
					return fmt.Errorf("remote host unavailable by ICMP: %v", err)
				}
			}
		}

		// if remote output dir is empty, use configfile defaults
		if *remoteOutputDir == "" {
			*remoteOutputDir = configFile.RemoteOutputDir
		}

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
				logger.Logx.WithField("package", "inputhandler").Fatal("Default remote host and username must be set in config.yml in order to use -remote-send-defaults")
			}

			// icmp check remote host if enabled
			if configFile.ICMPTest {
				if err := nethandler.ICMPRemoteHost(*remoteHost); err != nil {
					return fmt.Errorf("remote host unavailable by ICMP: %v", err)
				}
			}
		}
	} else {
		// set all remote values to empty
		*remoteOutputDir = ""
		*remoteHost = ""
		*remoteUser = ""
		*sendDefaults = false
	}

	// validate inputs
	err = validateInput(targetDir, dockerName, remoteUser, remoteHost, remoteOutputDir, skipLocal, configFile)
	if err != nil {
		logger.LogxWithFields("error", fmt.Sprintf("Input validation errors: %v", err), map[string]interface{}{
			"package": "inputhandler",
			"target":  filepath.Base(*targetDir),
		})
	}

	return nil

}
