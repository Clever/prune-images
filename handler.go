package main

import (
	"fmt"

	"github.com/Clever/prune-images/config"
	"github.com/Clever/prune-images/lib/dockerhub"
	"github.com/Clever/prune-images/lib/ecr"
	"gopkg.in/Clever/kayvee-go.v5/logger"
)

var (
	kv              = logger.New("prune-images")
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

	// List all repos
	kv.Info("get-all-docker-repos")
	repos, err := dockerhubClient.GetAllRepos()
	if err != nil {
		return err
	}

	for _, repo := range repos {
		// Look up tags in Docker Hub
		kv.InfoD("prune-docker-repo", logger.M{"repo": repo.Name})
		repoTags, err := dockerhubClient.GetTagsForRepo(repo)
		if err != nil && err != dockerhub.ErrorFailedToGetTags {
			// Some repos may not have tags; otherwise, error
			return err
		}

		// Prune tags in DockerHub
		deletedImages, err := dockerhubClient.PruneRepo(repoTags)
		if err != nil {
			return err
		}

		// Prune ECR with same image tags that were pruned from Docker Hub
		kv.InfoD("prune-ecr-repo", logger.M{"count": len(deletedImages), "repo": repo.Name})
		ecrClient.DeleteImages(deletedImages)
	}

	return nil
}
