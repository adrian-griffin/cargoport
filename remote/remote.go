package remote

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/adrian-griffin/cargoport/environment"
	"github.com/adrian-griffin/cargoport/sysutil"
)

// wrapper function for remoteSend
func HandleRemoteTransfer(filePath, remoteUser, remoteHost, remoteOutputDir string, skipLocal bool, configFile environment.ConfigFile) error {

	cargoportKey := filepath.Join(configFile.SSHKeyDir, configFile.SSHKeyName)

	// ensure remote host is reachable
	if err := checkRemoteHost(remoteHost, remoteUser, cargoportKey); err != nil {
		return fmt.Errorf("error validation remote host prior to transfer: %v", err)
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

	if _, err := os.Stat(cargoportKey); err != nil {
		return fmt.Errorf("SSH key not found at %s: %v", cargoportKey, err)
	}

	if passedRemotePath != "" {
		// Custom path provided at runtime
		remoteFilePath = strings.TrimSuffix(passedRemotePath, "/")
		remoteFilePath = fmt.Sprintf("%s/%s", remoteFilePath, backupFileNameBase)
	} else {
		// Fallback to configuration-defined path
		remoteFilePath = fmt.Sprintf("%s/remote/%s", strings.TrimSuffix(configFile.DefaultCargoportDir, "/"), backupFileNameBase)
	}

	log.Printf("Copying to remote %s@%s:%s . . .", passedRemoteUser, passedRemoteHost, remoteFilePath)

	rsyncArgs := []string{
		"-avz",
		"--checksum",
		"-e", fmt.Sprintf("ssh -i %s", cargoportKey),
		targetFileToTransfer,
		fmt.Sprintf("%s@%s:%s", passedRemoteUser, passedRemoteHost, remoteFilePath),
	}

	err := sysutil.RunCommand("rsync", rsyncArgs...)
	if err != nil {
		return fmt.Errorf("Failed to successfully perform rsync with remote server: %v", err)
	}

	log.Printf("Compressed File Successfully Transferred to %s", passedRemoteHost)
	return nil
}

// determine if remote host is a valid target
func checkRemoteHost(remoteHost, remoteUser, sshPrivKeypath string) error {
	// check host via icmp
	pingCmd := exec.Command("ping", "-c", "1", "-W", "2", remoteHost)
	if err := pingCmd.Run(); err != nil {
		fmt.Printf("ERROR <remote>: Target remote host %s is unreachable!\n", remoteHost)
		return fmt.Errorf("remote host %s is unreachable", remoteHost)
	}

	// check ssh connectivity rechability using keys
	cmd := exec.Command("ssh",
		"-i", sshPrivKeypath,
		"-o", "StrictHostKeyChecking=accept-new",
		fmt.Sprintf("%s@%s", remoteUser, remoteHost),
		"whoami",
	)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to connect via SSH key to %s@%s: %v", remoteUser, remoteHost, err)
	}
	fmt.Printf("SSH connection test success; remote user: %s\n", strings.TrimSpace(string(out)))
	return nil
}
