package input

import (
	"fmt"

	"github.com/adrian-griffin/cargoport/util"
)

// input struct to format final config + input outcomes
type InputContext struct {
	TargetDir       string
	DockerName      string
	OutputDir       string
	RestartDocker   bool
	SkipLocal       bool
	RemoteUser      string
	RemoteHost      string
	RemoteOutputDir string
	SendDefaults    bool
	Tag             string
	CopySSHKey      bool
	GenerateSSHKey  bool

	Config *ConfigFile
}

// finalize merges config defaults and validates all input.
func Finalize(ic *InputContext) error {
	cfg := ic.Config

	// if ssh copykey bool then ensure both remote vars are set (explicit only)
	if ic.CopySSHKey {
		if ic.RemoteHost == "" || ic.RemoteUser == "" {
			return fmt.Errorf("both remote host and user must be specified to copy SSH key")
		}

		// break out of finalizing input to process key copying & to prevent mixing of intent
		return nil
	}

	if ic.GenerateSSHKey {
		return nil
	}

	// apply config defaults
	if ic.SendDefaults {
		if cfg.RemoteUser == "" || cfg.RemoteHost == "" {
			return fmt.Errorf("-remote-send-defaults used, but missing defaults in config")
		}
		ic.RemoteUser = cfg.RemoteUser
		ic.RemoteHost = cfg.RemoteHost
		ic.RemoteOutputDir = cfg.RemoteOutputDir
	}

	// fallback to config default for skipLocal
	if !ic.SkipLocal && cfg.SkipLocal {
		ic.SkipLocal = true
	}

	// fallback remoteOutputDir if still unset
	if ic.RemoteOutputDir == "" && cfg.RemoteOutputDir != "" {
		ic.RemoteOutputDir = cfg.RemoteOutputDir
	}

	// validate target
	if ic.TargetDir == "" && ic.DockerName == "" {
		return fmt.Errorf("must specify either -target-dir or -docker-name")
	}
	if ic.TargetDir != "" && ic.DockerName != "" {
		return fmt.Errorf("cannot specify both -target-dir and -docker-name")
	}

	// validate both remotehost & remoteuser are supplied
	if ic.RemoteHost != "" || ic.RemoteUser != "" {
		if ic.RemoteHost == "" || ic.RemoteUser == "" {
			return fmt.Errorf("both remote-user and remote-host must be specified")
		}
	}

	// validate remote config
	if ic.SkipLocal {
		if ic.RemoteHost == "" || ic.RemoteUser == "" {
			return fmt.Errorf("-skip-local requires remote-user and remote-host")
		}
	}
	if ic.RemoteHost != "" {
		if err := util.ValidateIP(ic.RemoteHost); err != nil {
			return fmt.Errorf("invalid remote-host: %v", err)
		}
		if cfg.ICMPTest {
			if err := util.ICMPRemoteHost(ic.RemoteHost); err != nil {
				return fmt.Errorf("ICMP test failed: %v", err)
			}
		}
	}

	return nil
}
