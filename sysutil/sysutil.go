package sysutil

import (
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
