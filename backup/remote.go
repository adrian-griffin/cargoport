package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrian-griffin/cargoport/input"
	"github.com/adrian-griffin/cargoport/job"
	"github.com/adrian-griffin/cargoport/logger"
	"github.com/adrian-griffin/cargoport/util"
)

// debug level logging output fields for remote package
func remoteLogDebugFields(context *job.JobContext) map[string]interface{} {
	coreFields := logger.CoreLogFields(context, "remote")
	fields := logger.MergeFields(coreFields, map[string]interface{}{
		"remote":      context.Remote,
		"remote_user": context.RemoteUser,
		"remote_host": context.RemoteHost,
		"skip_local":  context.SkipLocal,
		// "remote_filepath":
	})
	return fields
}

// validate remote target dir & space
//func ValidateRemotePath(remoteUser, remoteHost, remoteOutputDir string)

// <here> logic for validating remote target dir existence, permissions, etc.
// as well as logic for confirming enough space on remote for transfer

// wrapper function for all remote-send functions
func HandleRemoteTransfer(jobctx *job.JobContext, filePath string, inputctx *input.InputContext) error {

	// <here> need to add logic here to toggle off icmp & ssh tests/validations in configfile
	cargoportKey := filepath.Join(inputctx.Config.SSHKeyDir, inputctx.Config.SSHKeyName)

	if inputctx.Config.ICMPTest {
		// test to ensure remote host is reachable via icmp
		if err := util.ICMPRemoteHost(inputctx.RemoteHost); err != nil {
			return fmt.Errorf("remote host is not responding to ICMP: %v", err)
		}
	}

	if inputctx.Config.SSHTest {
		// test ssh connectivity prior to attempting rsync
		if err := util.SSHTestRemoteHost(jobctx, inputctx.RemoteHost, inputctx.RemoteUser, cargoportKey); err != nil {
			return fmt.Errorf("remote host is not responding to SSH: %s", inputctx.RemoteHost)
		}
	}

	// proceed with remote transfer
	err := sendToRemote(jobctx, inputctx.RemoteOutputDir, inputctx.RemoteUser, inputctx.RemoteHost, filepath.Base(filePath), filePath, cargoportKey, *inputctx.Config)
	if err != nil {
		return fmt.Errorf("error performing remote transfer: %v", err)
	}

	// clean up local tempfile after transfer if skipLocal is enabled
	if jobctx.SkipLocal {
		util.RemoveTempFile(jobctx, filePath)
	}

	return nil
}

// handle remote rsync transfer to defined node
func sendToRemote(jobctx *job.JobContext, passedRemotePath, passedRemoteUser, passedRemoteHost, backupFileNameBase, targetFileToTransfer, cargoportKey string, configFile input.ConfigFile) error {

	// defining logging fields
	verboseFields := remoteLogDebugFields(jobctx)

	//<section>  VALIDATIONS
	//---------
	// if remote host or user are empty
	if passedRemoteHost == "" || passedRemoteUser == "" {
		return fmt.Errorf("both remote user and host must be specified for remote transfer")
	}

	var remoteFilePath string

	// ensure SSH key exists
	if _, err := os.Stat(cargoportKey); err != nil {
		return fmt.Errorf("SSH key not found at %s: %v", cargoportKey, err)
	}

	// construct remote file path
	// fallback to config if no custom path defined
	if passedRemotePath != "" {
		// custom path provided at runtime
		remoteFilePath = strings.TrimSuffix(passedRemotePath, "/")
		remoteFilePath = fmt.Sprintf("%s/%s", remoteFilePath, backupFileNameBase)
	} else {
		// fallback to configuration-defined path
		remoteFilePath = fmt.Sprintf("~/%s", backupFileNameBase)
	}
	logger.LogxWithFields("debug", fmt.Sprintf("Transferring to remote %s@%s:%s", passedRemoteUser, passedRemoteHost, remoteFilePath), logger.MergeFields(verboseFields, map[string]interface{}{
		"remote_dir": filepath.Dir(remoteFilePath),
	}))

	rsyncArgs := []string{
		"-avz",
		"--checksum",
		"-e", fmt.Sprintf("ssh -i %s -o ConnectTimeout=10 -o ServerAliveInterval=5 -o ServerAliveCountMax=2", cargoportKey),
		targetFileToTransfer,
		fmt.Sprintf("%s@%s:%s", passedRemoteUser, passedRemoteHost, remoteFilePath),
	}

	// validate ssh private key integrity
	if err := util.ValidateSSHPrivateKeyPerms(cargoportKey); err != nil {
		return fmt.Errorf("private SSH key integrity check failed, key may have been tampered with, please generate a new keypair")
	}

	// run rsync
	if err := util.RunCommand("rsync", rsyncArgs...); err != nil {
		return fmt.Errorf("rsync failed: %v", err)
	}

	logger.LogxWithFields("info", "Snapshot successfully transferred to remote", map[string]interface{}{
		"package":     "remote",
		"remote":      true,
		"remote_host": passedRemoteHost,
		"remote_user": passedRemoteUser,
		"success":     true,
		"target":      jobctx.Target,
		"job_id":      jobctx.JobID,
	})
	return nil
}
