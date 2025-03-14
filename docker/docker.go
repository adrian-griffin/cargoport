package docker

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/adrian-griffin/cargoport/sysutil"
)

// locates docker compose file based on container name
func FindComposeFile(containerName string) (string, error) {
	cmd := exec.Command("docker", "inspect", containerName, "--format", "{{ index .Config.Labels \"com.docker.compose.project.working_dir\" }}")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("ERROR <storage>: Failed to locate docker compose file for container '%s': %v", containerName, err)
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
		return false, fmt.Errorf("Failed to obtain Docker container status: %v", err)
	}
	runningServices := strings.TrimSpace(string(output))
	if runningServices == "" {
		return false, fmt.Errorf("No active services found from composefile (container is likely off)")
	}
	return true, nil
}

// stop docker containers & collect image ids and digests
func HandleDockerPreBackup(composeFilePath string) error {
	fmt.Println("-------------------------------------------------------------------------")
	fmt.Println("Handling Docker pre-backup tasks . . .")
	fmt.Println("-------------------------------------------------------------------------")

	// checks whether docker is running
	running, err := checkDockerRunState(composeFilePath)
	if err != nil || !running {
		log.Printf("WARNING <docker>: No active Docker container at target location. Proceeding with backup.")
		// temporarily partially bring up container to gather image information
		if err := sysutil.RunCommand("docker", "compose", "-f", composeFilePath, "up", "--no-start"); err != nil {
			return fmt.Errorf("failed to partially bring up docker containers containers for image inspection: %v", err)
		}
	}

	// gathers and writes images to disk
	imageVersionFile := filepath.Join(filepath.Dir(composeFilePath), "compose-img-digests.txt")
	if err := writeDockerImages(composeFilePath, imageVersionFile); err != nil {
		return fmt.Errorf("failed to collect Docker images: %v", err)
	}

	// shuts down docker container from composefile
	log.Println("Performing Docker compose jobs . . .")
	if err := sysutil.RunCommand("docker", "compose", "-f", composeFilePath, "down"); err != nil {
		return fmt.Errorf("failed to stop Docker containers: %v", err)
	}
	return nil
}

// collects docker image information and digests, stores alongside `docker-compose.yml` file
func writeDockerImages(composeFile string, outputFile string) error {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "images", "--quiet")
	output, err := cmd.Output()
	if err != nil {

		return fmt.Errorf("Failed to obtain docker images: %v", err)
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
			return fmt.Errorf("Failed to inspect docker images: %v", err)
		}
		imageInfo += fmt.Sprintf("Image ID: %s  |  Image Digest: %s\n", imageID, digestOutput)
	}

	// cleans up image whitespace formatting and writes to file
	trimmedImageInfo := strings.TrimSpace(imageInfo)
	err = os.WriteFile(outputFile, []byte(trimmedImageInfo), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write docker image version info to file: %v", err)
	}

	fmt.Println("Docker image IDs and digests saved to", outputFile)
	return nil
}

// handles docker container restart/turn-up commands
func HandleDockerPostBackup(composeFilePath string, restartDockerBool bool) error {
	if restartDockerBool == false {
		fmt.Println("Docker restart disabled, skipping restart . . .")
		log.Println("Docker restart disabled, skipping restart . . .")
		return nil
	}
	log.Println("Restarting Docker compose . . .")
	if err := startDockerContainer(composeFilePath); err != nil {
		return fmt.Errorf("failed to restart Docker containers at : %s", composeFilePath)
	}
	return nil
}

// starts docker container from yaml file
func startDockerContainer(composefile string) error {
	// Restart docker container
	fmt.Println("-------------------------------------------------------------------------")
	fmt.Println("Starting Docker container . . .")
	fmt.Println("-------------------------------------------------------------------------")
	err := sysutil.RunCommand("docker", "compose", "-f", composefile, "up", "-d")
	if err != nil {
		fmt.Printf("Error starting Docker container: %v", err)
		log.Fatalf("ERROR <docker>: Failed to start Docker container: %v", err)
	}
	log.Printf("Successful startup job on docker compose at %s", composefile)
	return err
}
