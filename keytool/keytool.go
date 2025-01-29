package keytool

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// create local cargoport keypair
func GenerateSSHKeypair(sshDir, keyName string) error {
	privateKeyPath := filepath.Join(sshDir, keyName)
	publicKeyPath := privateKeyPath + ".pub"

	// if the private key already exists, do not overwrite
	if _, err := os.Stat(privateKeyPath); err == nil {
		log.Printf("SSH Key '%s' already exists. Skipping generation.\n", privateKeyPath)
		fmt.Printf("SSH Key '%s' already exists. Skipping generation.\n", privateKeyPath)
		return nil
	}

	// ensure ssh dir (e.g. /var/cargoport/.ssh) exists
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create SSH directory '%s': %v", sshDir, err)
	}

	// build the ssh-keygen command
	cmd := exec.Command("ssh-keygen",
		"-t", "ed25519",
		"-f", privateKeyPath,
		"-N", "", // no passphrase for cronjobs
		"-C", "cargoport-generated-key",
	)

	// for cleanliness, redirect stdout/stderr if desired
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate SSH key: %v", err)
	}

	// set private key to 600
	if err := os.Chmod(privateKeyPath, 0600); err != nil {
		return fmt.Errorf("failed to chmod private key: %v", err)
	}

	fmt.Printf("SSH key pair generated at: %s (public: %s)\n", privateKeyPath, publicKeyPath)
	log.Printf("SSH key pair generated at: %s (public: %s)\n", privateKeyPath, publicKeyPath)
	return nil
}

func CopyPublicKey(sshPrivKeypath, remoteUser, remoteHost string) error {
	// define pubkey
	sshPubKeyPath := sshPrivKeypath + ".pub"

	//logStart("Copy SSH Key      |    cargoport %s    |    <target: %s>  \n", Version, remoteHost)

	// utilize ssh-copy-id
	cmd := exec.Command("ssh-copy-id", "-i", sshPubKeyPath, fmt.Sprintf("%s@%s", remoteUser, remoteHost))

	// redir sshkeygen stdout to os
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy SSH public key to remote: %v", err)
	}

	log.Printf("Successfully installed local public key into %s@%s:~/.ssh/authorized_keys\n", remoteUser, remoteHost)
	//logEnd("SSH Key Copied    |          Complete        |    <target: %s>  \n", remoteHost)
	return nil
}
