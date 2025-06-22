package util

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/adrian-griffin/cargoport/job"
	"github.com/adrian-griffin/cargoport/logger"
)

// debug level logging output fields for system package
func systemLogDebugFields(context *job.JobContext) map[string]interface{} {
	coreFields := logger.CoreLogFields(context, "system")
	fields := logger.MergeFields(coreFields, map[string]interface{}{
		"skip_local": context.SkipLocal,
		"target_dir": context.TargetDir,
		"tag":        context.Tag,
	})
	return fields
}

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
func RemoveTempFile(context *job.JobContext, filePath string) error {

	verboseFields := systemLogDebugFields(context)

	logger.LogxWithFields("debug", fmt.Sprintf("Cleaning up tempfile at %s", filePath), verboseFields)

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("error removing tempfile: %v", err)
	} else {
		logger.LogxWithFields("debug", fmt.Sprintf("Tempfile %s removed", filePath), verboseFields)
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
	// validate directory string before proceeding
	if err := ValidateDirectoryString(directoryPathString); err != nil {
		return err
	}

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

func GetDirectorySize(path string) (int64, error) {
	var total int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err // propagate error
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func GetTarballCount(path string) (int, error) {
	count := 0
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".bak.tar.gz") {
			count++
		}
		return nil
	})
	return count, err
}
