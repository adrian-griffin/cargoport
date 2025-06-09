package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/adrian-griffin/cargoport/jobcontext"
	"github.com/adrian-griffin/cargoport/logger"
	"github.com/adrian-griffin/cargoport/sysutil"
)

// debug level logging output fields for docker package
func dockerLogBaseFields(context jobcontext.JobContext) map[string]interface{} {
	coreFields := logger.CoreLogFields(&context, "docker")
	fields := logger.MergeFields(coreFields, map[string]interface{}{
		"docker":         context.Docker,
		"restart_docker": context.RestartDocker,
		"target_dir":     context.TargetDir,
		"tag":            context.Tag,
	})
	return fields
}

// locates docker compose file based on container name
func FindComposeFile(containerName, targetBaseName string) (string, error) {
	cmd := exec.Command("docker", "inspect", containerName, "--format", "{{ index .Config.Labels \"com.docker.compose.project.working_dir\" }}")
	output, err := cmd.Output()
	if err != nil {
		logger.LogxWithFields("error", fmt.Sprintf("Failed to locate docker compose file for container '%s': %v", containerName, err), map[string]interface{}{
			"package": "docker",
			"target":  targetBaseName,
		})
		return "", fmt.Errorf("failed to locate docker compose file for container '%s': %v", containerName, err)
	}
	composePath := strings.TrimSpace(string(output))
	return filepath.Join(composePath, "docker-compose.yml"), nil // return filepath to compose
}

// returns whether or not any docker services are running from target composefile
func checkDockerRunState(composeFile string) (bool, error) {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "ps", "--services", "--filter", "status=running")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to obtain Docker container status: %v", err)
	}
	runningServices := strings.TrimSpace(string(output))
	if runningServices == "" {
		return false, fmt.Errorf("no active services found from composefile (container is likely off)")
	}
	return true, nil
}

// stop docker containers & collect image ids and digests
func HandleDockerPreBackup(context *jobcontext.JobContext, composeFilePath, targetBaseName string) error {

	// defining logging fields
	verboseFields := dockerLogBaseFields(*context)
	coreFields := logger.CoreLogFields(context, "docker")

	logger.LogxWithFields("debug", fmt.Sprintf("Handling docker pre-backup tasks"), verboseFields)
	// checks whether docker is running
	running, err := checkDockerRunState(composeFilePath)
	if err != nil || !running {
		logger.LogxWithFields("warn", fmt.Sprintf("No active Docker container at %s. Proceeding with backup.", composeFilePath), coreFields)
		// temporarily partially bring up container to gather image information
		if err := sysutil.RunCommand("docker", "compose", "-f", composeFilePath, "up", "--no-start"); err != nil {
			return fmt.Errorf("failed to partially bring up docker containers containers for image inspection: %v", err)
		}
	}

	// gathers and writes images to disk
	imageVersionFile := filepath.Join(filepath.Dir(composeFilePath), "compose-img-digests.txt")
	if err := writeDockerImages(context, composeFilePath, imageVersionFile); err != nil {
		return fmt.Errorf("failed to collect Docker images: %v", err)
	}

	// shuts down docker container from composefile
	logger.LogxWithFields("debug", fmt.Sprintf("Performing Docker compose down jobs on %s", composeFilePath), verboseFields)
	if err := sysutil.RunCommand("docker", "compose", "-f", composeFilePath, "down"); err != nil {
		return fmt.Errorf("failed to stop Docker containers: %v", err)
	}

	// notify pre-backup docker job status
	logger.LogxWithFields("info", fmt.Sprintf("Pre-backup docker jobs handled successfully"), map[string]interface{}{
		"package": "docker",
		"target":  context.Target,
		"job_id":  context.JobID,
		"remote":  context.Remote,
		"docker":  context.Docker,
		// add # of services as a tag perhaps?
	})
	return nil
}

// collects docker image information and digests, stores alongside `docker-compose.yml` file
func writeDockerImages(context *jobcontext.JobContext, composeFile string, outputFile string) error {

	// defining logging fields
	verboseFields := dockerLogBaseFields(*context)
	// coreFields := logger.CoreLogFields(context, "docker")

	cmd := exec.Command("docker", "compose", "-f", composeFile, "images", "--quiet")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to obtain docker images: %v", err)
	}

	// loops over image ids to gather docker image digests
	imageLines := string(output)
	imageList := strings.Split(imageLines, "\n")
	var imageInfo string

	for _, imageID := range imageList {
		if imageID == "" {
			continue
		}

		// grab image digests based on image id
		cmdInspect := exec.Command("docker", "inspect", "--format", "{{index .RepoDigests 0}}", imageID)
		digestOutput, err := cmdInspect.Output()
		if err != nil {
			return fmt.Errorf("failed to inspect docker images: %v", err)
		}
		imageInfo += fmt.Sprintf("Image ID: %s  |  Image Digest: %s\n", imageID, digestOutput)
	}

	// cleans up image whitespace formatting and writes to file
	trimmedImageInfo := strings.TrimSpace(imageInfo)
	err = os.WriteFile(outputFile, []byte(trimmedImageInfo), 0644)
	if err != nil {
		return fmt.Errorf("failed to write docker image version info to file: %v", err)
	}

	logger.LogxWithFields("debug", fmt.Sprintf("Docker service image IDs and digests saved to %s", outputFile), verboseFields)
	return nil
}

// handles docker container restart/turn-up commands
func HandleDockerPostBackup(context *jobcontext.JobContext, composeFilePath string, restartDockerBool bool) error {

	verboseFields := dockerLogBaseFields(*context)
	// coreFields := logger.CoreLogFields(context, "docker")

	if !restartDockerBool {
		logger.LogxWithFields("info", fmt.Sprintf("Docker service restart disabled, skipping restart"), verboseFields)
		return nil
	}
	logger.LogxWithFields("debug", fmt.Sprintf("Restarting Docker compose services via %s", composeFilePath), verboseFields)
	if err := startDockerContainer(context, composeFilePath); err != nil {
		return fmt.Errorf("failed to restart Docker containers at : %s", composeFilePath)
	}
	return nil
}

// starts docker container from yaml file
func startDockerContainer(context *jobcontext.JobContext, composefile string) error {

	verboseFields := dockerLogBaseFields(*context)
	coreFields := logger.CoreLogFields(context, "docker")

	// restart docker container
	logger.LogxWithFields("debug", fmt.Sprintf("Starting Docker container at %s as headless/daemon", filepath.Dir(composefile)), verboseFields)
	err := sysutil.RunCommand("docker", "compose", "-f", composefile, "up", "-d")
	if err != nil {
		logger.LogxWithFields("error", fmt.Sprintf("Error starting Docker container: %v", err), coreFields)
		return err
	}
	logger.LogxWithFields("debug", fmt.Sprintf("Successful startup job on docker compose at %s", composefile), verboseFields)

	// if no errors, info alert of success
	logger.LogxWithFields("info", "Post-backup docker jobs handled successfully", map[string]interface{}{
		"package":        "docker",
		"target":         context.Target,
		"job_id":         context.JobID,
		"remote":         context.Remote,
		"docker":         context.Docker,
		"restart_docker": context.RestartDocker,
	})

	return err
}
