package nethandler

import (
	"fmt"
	"net"

	"github.com/adrian-griffin/cargoport/logger"
	"github.com/adrian-griffin/cargoport/sysutil"
)

// validate string as valid IPv4, IPv6 address, or resolvable DNS name
func ValidateIP(remoteHost string) error {
	// if not a valid v4 or v6 IP, attempt dns lookup
	if net.ParseIP(remoteHost) == nil {
		_, err := net.LookupHost(remoteHost)
		if err != nil {
			return fmt.Errorf("provided host must be a valid IP(v4/v6) address or queriable hostname: %v", err)
		}
	}
	return nil
}

// test ICMP reachability to remote host
func ICMPRemoteHost(remoteHost string) error {
	// check host via icmp
	if err := sysutil.RunCommand("ping", "-c", "1", "-W", "2", remoteHost); err != nil {
		return fmt.Errorf("remote host %s is unreachable via ICMP", remoteHost)
	}

	logger.LogxWithFields("debug", fmt.Sprintf("ICMP connection test against %s successful", remoteHost), map[string]interface{}{
		"package":     "nethandler",
		"remote":      true,
		"success":     true,
		"remote_host": remoteHost,
	})
	return nil
}

// test SSH connectivity to remote host
func SSHTestRemoteHost(remoteHost, remoteUser, sshPrivKeypath string) error {
	// check ssh connectivity rechability using keys
	_, err := sysutil.RunCommandWithOutput("ssh",
		"-i", sshPrivKeypath,
		"-o", "StrictHostKeyChecking=accept-new",
		fmt.Sprintf("%s@%s", remoteUser, remoteHost),
		"whoami")

	if err != nil {
		return fmt.Errorf("failed to connect via SSH to %s@%s: %v", remoteUser, remoteHost, err)
	}

	logger.LogxWithFields("debug", fmt.Sprintf("SSH connection test success, remote user: %s", remoteUser), map[string]interface{}{
		"package":     "nethandler",
		"remote":      true,
		"success":     true,
		"remote_host": remoteHost,
	})
	return nil
}
