package remote

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrian-griffin/cargoport/environment"
	"github.com/adrian-griffin/cargoport/keytool"
	"github.com/adrian-griffin/cargoport/nethandler"
	"github.com/adrian-griffin/cargoport/sysutil"
)

// wrapper function for remoteSend
func HandleRemoteTransfer(filePath, remoteUser, remoteHost, remoteOutputDir string, skipLocal bool, configFile environment.ConfigFile) error {

	cargoportKey := filepath.Join(configFile.SSHKeyDir, configFile.SSHKeyName)

	// try to ensure remote host is reachable via icmp
	if err := nethandler.ICMPRemoteHost(remoteHost, remoteUser); err != nil {
		return err //<~ gotta add icmp toggle in config+nethandler
	}

	if err := nethandler.SSHTestRemoteHost(remoteHost, remoteUser, cargoportKey); err != nil {
		return err
	}

	// proceed with remote transfer
	err := sendToRemote(remoteOutputDir, remoteUser, remoteHost, filepath.Base(filePath), filePath, cargoportKey, configFile)
	if err != nil {
		return fmt.Errorf("error performing transfer: %v", err)
	}

	// clean up local tempfile after transfer if skipLocal is enabled
	if skipLocal {
		sysutil.RemoveTempFile(filePath)
	}

	return nil
}

// handle remote rsync transfer to another node
func sendToRemote(passedRemotePath, passedRemoteUser, passedRemoteHost, backupFileNameBase, targetFileToTransfer, cargoportKey string, configFile environment.ConfigFile) error {

	//<section>  VALIDATIONS
	//---------
	// if remote host or user are empty
	if passedRemoteHost == "" || passedRemoteUser == "" {
		return fmt.Errorf("Both remote user and host must be specified for remote transfer!")
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
		remoteFilePath = fmt.Sprintf("%s/remote/%s", strings.TrimSuffix(configFile.DefaultCargoportDir, "/"), backupFileNameBase)
	}

	log.Printf("Transferring to remote %s@%s:%s . . .", passedRemoteUser, passedRemoteHost, remoteFilePath)

	rsyncArgs := []string{
		"-avz",
		"--checksum",
		"-e", fmt.Sprintf("ssh -i %s", cargoportKey),
		targetFileToTransfer,
		fmt.Sprintf("%s@%s:%s", passedRemoteUser, passedRemoteHost, remoteFilePath),
	}

	// run rsync and capture output
	output, err := sysutil.RunCommandWithOutput("rsync", rsyncArgs...)
	if err != nil {
		// check if the error message contains "password:"
		if strings.Contains(output, "password:") {
			fmt.Println("SSH password prompt detected. Attempting to copy SSH key...")
			log.Println("SSH password prompt detected. Attempting to copy SSH key...")

			// Copy SSH key to the remote host
			err = keytool.CopyPublicKey(cargoportKey, passedRemoteUser, passedRemoteHost)
			if err != nil {
				return fmt.Errorf("failed to copy SSH key to remote host: %v", err)
			}

			// Retry rsync after copying the SSH key
			fmt.Println("Retrying rsync with SSH key authentication...")
			log.Println("Retrying rsync with SSH key authentication...")
			err = sysutil.RunCommand("rsync", rsyncArgs...)
			if err != nil {
				return fmt.Errorf("failed to transfer file after SSH key copy: %v", err)
			}
		} else {
			return fmt.Errorf("rsync failed: %v", err)
		}
	}

	log.Printf("Compressed File Successfully Transferred to %s", passedRemoteHost)
	return nil
}
