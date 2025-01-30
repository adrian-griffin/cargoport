package sysutil

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
)

// executes command on os
func RunCommand(commandName string, args ...string) error {
	cmd := exec.Command(commandName, args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// executes command on os, capturing output
func RunCommandWithOutput(cmd string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	command := exec.Command(cmd, args...)
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	output := stdout.String() + stderr.String()
	if err != nil {
		return output, fmt.Errorf("%s", output)
	}
	return output, nil
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
