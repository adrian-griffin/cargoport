package sysutil

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

// executes command on local system
func RunCommand(commandName string, args ...string) error {
	cmd := exec.Command(commandName, args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// remove file from os
func RemoveTempFile(filePath string) error {

	log.Printf("Cleaning up tempfile at %s\n", filePath)

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("error removing tempfile: %v", err)
	} else {
		fmt.Printf("Tempfile %s removed\n", filePath)
	}

	return nil
}
