package sysutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/adrian-griffin/cargoport/logger"
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

	logger.Logx.WithField("package", "systutil").Debugf("Cleaning up tempfile at %s\n", filePath)

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("error removing tempfile: %v", err)
	} else {
		logger.Logx.WithField("package", "sysutil").Debugf("Tempfile %s removed\n", filePath)
	}

	return nil
}

func ValidateDirectoryString(directoryPathString string) error {
	// validate directory exists
	dirInfo, err := os.Stat(directoryPathString)

	// if dir DNE or is not dirtype, return err
	if err != nil || !dirInfo.IsDir() {
		return fmt.Errorf("target path %s does not exist or is not a directory", directoryPathString)
	}

	return nil
}

func ValidateDirectoryWriteable(directoryPathString string) error {
	// attempt to create temp local file
	testFilePath := filepath.Join(directoryPathString, ".cargoport_testwrite.tmp")
	// create & remove file, return error if file creation fails
	testFile, err := os.Create(testFilePath)
	if err != nil {
		return fmt.Errorf("cannot write to destination directory %s: %v", directoryPathString, err)
	}
	testFile.Close()
	os.Remove(testFilePath)

	return nil
}
