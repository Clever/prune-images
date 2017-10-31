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
	dockerhubClient = dockerhub.NewClient(config.DockerHubUsername, config.DockerHubPassword)
	ecrClient = ecr.NewClient()

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
	deletedImages, errs := dockerhubClient.PruneAllRepos()
	if len(errs) > 0 {
		for _, err := range errs {
			kv.ErrorD("prune-docker-repos", logger.M{
				"error": err.Error(),
			})
		}
		return fmt.Errorf("one or more errors found while pruning repos from Docker Hub")
	}

	for _, repo := range deletedImages {
		kv.InfoD("prune-docker-repos", logger.M{
			"repository": repo.RepoName,
			"tags":       repo.Tags,
		})
	}

	// Prune ECR with the images that were pruned from Docker Hub
	kv.Info("prune-ecr-repos")
	errs = ecrClient.DeleteImages(deletedImages)
	if len(errs) > 0 {
		for _, err := range errs {
			kv.ErrorD("prune-ecr-repos", logger.M{
				"error": err.Error(),
			})
		}
		return fmt.Errorf("one or more errors found while pruning repos from ECR")
	}

	return nil
}
