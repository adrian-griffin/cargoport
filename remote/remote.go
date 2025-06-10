package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrian-griffin/cargoport/environment"
	"github.com/adrian-griffin/cargoport/jobcontext"
	"github.com/adrian-griffin/cargoport/keytool"
	"github.com/adrian-griffin/cargoport/logger"
	"github.com/adrian-griffin/cargoport/nethandler"
	"github.com/adrian-griffin/cargoport/sysutil"
)

// debug level logging output fields for remote package
func remoteLogDebugFields(context *jobcontext.JobContext) map[string]interface{} {
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
func HandleRemoteTransfer(context *jobcontext.JobContext, filePath, remoteUser, remoteHost, remoteOutputDir string, skipLocal bool, configFile environment.ConfigFile) error {

	// <here> need to add logic here to toggle off icmp & ssh tests/validations in configfile
	cargoportKey := filepath.Join(configFile.SSHKeyDir, configFile.SSHKeyName)

	if configFile.ICMPTest {
		// test to ensure remote host is reachable via icmp
		if err := nethandler.ICMPRemoteHost(remoteHost); err != nil {
			return fmt.Errorf("remote host is not responding to ICMP: %v", err)
		}
	}

	if configFile.SSHTest {
		// test ssh connectivity prior to attempting rsync
		if err := nethandler.SSHTestRemoteHost(context, remoteHost, remoteUser, cargoportKey); err != nil {
			return fmt.Errorf("remote host is not responding to SSH: %v", err)
		}
	}

	// proceed with remote transfer
	err := sendToRemote(context, remoteOutputDir, remoteUser, remoteHost, filepath.Base(filePath), filePath, cargoportKey, configFile)
	if err != nil {
		return fmt.Errorf("error performing remote transfer: %v", err)
	}

	// clean up local tempfile after transfer if skipLocal is enabled
	if skipLocal {
		sysutil.RemoveTempFile(context, filePath)
	}

	return nil
}

// handle remote rsync transfer to defined node
func sendToRemote(context *jobcontext.JobContext, passedRemotePath, passedRemoteUser, passedRemoteHost, backupFileNameBase, targetFileToTransfer, cargoportKey string, configFile environment.ConfigFile) error {

	// defining logging fields
	verboseFields := remoteLogDebugFields(context)

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
	if err := keytool.ValidateSSHPrivateKeyPerms(cargoportKey); err != nil {
		return fmt.Errorf("private SSH key integrity check failed, key may have been tampered with, please generate a new keypair")
	}

	// run rsync
	if err := sysutil.RunCommand("rsync", rsyncArgs...); err != nil {
		return fmt.Errorf("rsync failed: %v", err)
	}

	logger.LogxWithFields("info", "Snapshot successfully transferred to remote", map[string]interface{}{
		"package":     "remote",
		"remote":      true,
		"remote_host": passedRemoteHost,
		"remote_user": passedRemoteUser,
		"success":     true,
		"target":      context.Target,
		"job_id":      context.JobID,
	})
	return nil
}
