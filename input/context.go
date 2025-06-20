package input

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/adrian-griffin/cargoport/job"
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

	Config *ConfigFile
}

// Finalize merges config defaults and validates all input.
func Finalize(ic *InputContext) error {
	cfg := ic.Config

	// Apply config defaults
	if ic.SendDefaults {
		if cfg.RemoteUser == "" || cfg.RemoteHost == "" {
			return fmt.Errorf("-remote-send-defaults used, but missing defaults in config")
		}
		ic.RemoteUser = cfg.RemoteUser
		ic.RemoteHost = cfg.RemoteHost
		ic.RemoteOutputDir = cfg.RemoteOutputDir
	}

	// Fallback to config default for skipLocal
	if !ic.SkipLocal && cfg.SkipLocal {
		ic.SkipLocal = true
	}

	// Fallback remoteOutputDir if still unset
	if ic.RemoteOutputDir == "" && cfg.RemoteOutputDir != "" {
		ic.RemoteOutputDir = cfg.RemoteOutputDir
	}

	// Validate target
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

	// Validate remote config
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

// BuildJobContext initializes a new JobContext from InputContext
func (ic *InputContext) BuildJobContext() *job.JobContext {
	return &job.JobContext{
		Target:                 "",
		Remote:                 ic.RemoteHost != "",
		Docker:                 false, // updated later after DetermineBackupTarget
		SkipLocal:              ic.SkipLocal,
		JobID:                  job.GenerateJobID(),
		StartTime:              time.Now(),
		TargetDir:              "", // updated later after DetermineBackupTarget
		RootDir:                filepath.Join(ic.Config.DefaultCargoportDir, "local"),
		Tag:                    ic.Tag,
		RestartDocker:          ic.RestartDocker,
		RemoteHost:             ic.RemoteHost,
		RemoteUser:             ic.RemoteUser,
		CompressedSizeBytesInt: 0,
		CompressedSizeMBString: "0.0 MB",
	}
}
