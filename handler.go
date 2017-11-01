package main

import (
	"fmt"

	"github.com/Clever/prune-images/config"
	"github.com/Clever/prune-images/lib/dockerhub"
	"github.com/Clever/prune-images/lib/ecr"
	"gopkg.in/Clever/kayvee-go.v5/logger"
)

var (
	kv              = logger.New("init-service")
	dockerhubClient *dockerhub.Client
	ecrClient       *ecr.Client
)

func pruneRepos() error {
	dockerhubClient = dockerhub.NewClient(config.DockerHubUsername, config.DockerHubPassword, config.DryRun)
	ecrClient = ecr.NewClient(config.DryRun)

	// Login to DockerHub to get a token
	kv.Info("dockerhub-login")
	err := dockerhubClient.Login()
	if err != nil {
		kv.ErrorD("dockerhub-login", logger.M{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to login to DockerHub")
	}

	// Prune from Docker Hub
	kv.Info("prune-docker-repos")
	deletedImages, encounteredNonFatalError, err := dockerhubClient.PruneAllRepos()
	if encounteredNonFatalError {
		kv.ErrorD("prune-docker-repos", logger.M{
			"error": "encountered one or more non-fatal errors",
		})
	}
	if err != nil {
		return fmt.Errorf("error while pruning repos from Docker Hub: %s", err.Error())
	}

	for _, repo := range deletedImages {
		kv.InfoD("prune-docker-repos", logger.M{
			"repository": repo.RepoName,
			"tags":       repo.Tags,
		})
	}

	// Prune ECR with the images that were pruned from Docker Hub
	kv.Info("prune-ecr-repos")
	encounteredNonFatalError = ecrClient.DeleteImages(deletedImages)
	if encounteredNonFatalError {
		kv.ErrorD("prune-ecr-repos", logger.M{
			"error": "encountered one or more non-fatal errors",
		})
	}

	return nil
}
