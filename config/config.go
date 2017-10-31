package config

import (
	"log"
	"os"
	"strconv"
)

var (
	// DockerHubNamespace is the namespace used when creating a new repo (hub.docker.com/r/<namespace>/<repo>).
	// It must be the namespace for an org, because we add permissions for organization, because the permissions
	// step is for adding organization-style collaborators.
	DockerHubNamespace string = "clever"

	// DockerHubUsername is the username to login to DockerHub.
	// It must be for an account that can create a repo and add permissions.
	DockerHubUsername string

	// DockerHubPassword is the password to login to DockerHub.
	DockerHubPassword string

	// If DryRun is set to true, deleting will not occur
	DryRun bool
)

// Parse reads environment variables and initializes the config
func Parse() {
	DockerHubUsername = requiredEnv("DOCKERHUB_USERNAME")
	DockerHubPassword = requiredEnv("DOCKERHUB_PASSWORD")
	dryRun, err := strconv.ParseBool(requiredEnv("DRY_RUN"))
	if err != nil {
		log.Fatal("Invalid value for DRY_RUN: " + err.Error())
	}

	DryRun = dryRun
	if DryRun {
		log.Println("doing dry run of pruning repos")
	}

}

// requiredEnv tries to find a value in the environment variables. If a value is not
// found the program will panic.
func requiredEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatal("Missing env var: " + key)
	}
	return value
}
