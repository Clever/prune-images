package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"gopkg.in/Clever/kayvee-go.v6/logger"

	"github.com/Clever/prune-images/common"
	"github.com/Clever/prune-images/config"
)

var (
	kv = logger.New("prune-images")
)

func pruneRepos() error {
	kv.InfoD("pruning-repos", logger.M{"dry-run": config.DryRun, "minImages": common.MinImagesInRepo})

	regions := strings.Split(os.Getenv("REGIONS"), ",")
	if len(regions) == 0 {
		return fmt.Errorf("env variable REGIONS not set")
	}

	// Prune repositories for all regions
	for _, region := range config.Regions {
		kv.DebugD("ecr-prune-region", logger.M{"region": region})

		awsConfig := aws.NewConfig().WithMaxRetries(10)
		awsConfig.Region = &region
		ecrClient := ecr.New(session.New(), awsConfig)

		// Get repositories in region
		repositories, err := ecrClient.DescribeRepositories(&ecr.DescribeRepositoriesInput{})
		if err != nil {
			return fmt.Errorf("failed to get repositories in region %v: %v", region, err.Error())
		}

		// Prune each repositories images
		for _, repo := range repositories.Repositories {
			if err := pruneRepo(ecrClient, repo); err != nil {
				return fmt.Errorf("failed to prune repo %v: %v", *repo.RepositoryName, err.Error())
			}
		}
	}

	return nil
}

func pruneRepo(ecrClient *ecr.ECR, repo *ecr.Repository) error {
	kv.DebugD("ecr-get-repo-images", logger.M{"repo": *repo.RepositoryName})
	images, err := ecrClient.DescribeImages(&ecr.DescribeImagesInput{
		RegistryId:     repo.RegistryId,
		RepositoryName: repo.RepositoryName,
	})
	if err != nil {
		kv.WarnD("failed-to-get-repo-images", logger.M{"error": err.Error()})
		return fmt.Errorf("failed-to-get-repo-images: %v", err.Error())
	}

	kv.DebugD("repo-image-count", logger.M{"repo": *repo.RepositoryName, "count": len(images.ImageDetails)})

	// If the image limit is not reached, skip
	if len(images.ImageDetails) <= common.MinImagesInRepo {
		return nil
	}

	kv.InfoD("ecr-prune-repo", logger.M{"repo": *repo.RepositoryName, "image_count": len(images.ImageDetails)})

	// Sort the images from most recent to least recent
	sort.Slice(images.ImageDetails, func(i, j int) bool {
		return images.ImageDetails[i].ImagePushedAt.Unix() > images.ImageDetails[j].ImagePushedAt.Unix()
	})

	imagesToDelete := []*ecr.ImageIdentifier{}

	for i := common.MinImagesInRepo; i < len(images.ImageDetails); i++ {
		kv.DebugD("image-to-delete", logger.M{"repo": *repo.RepositoryName, "image": *images.ImageDetails[i].ImageDigest})
		imagesToDelete = append(imagesToDelete, &ecr.ImageIdentifier{
			ImageDigest: images.ImageDetails[i].ImageDigest,
			ImageTag:    images.ImageDetails[i].ImageTags[0],
		})
	}

	if !config.DryRun {
		batchDeleteOutput, err := ecrClient.BatchDeleteImage(&ecr.BatchDeleteImageInput{
			ImageIds:       imagesToDelete,
			RegistryId:     repo.RegistryId,
			RepositoryName: repo.RepositoryName,
		})
		if err != nil {
			return fmt.Errorf("failed to delete ECR repository images %s: %s", *repo.RepositoryName, err.Error())
		}
		kv.InfoD("erc-batch-delete-images", logger.M{"count": len(batchDeleteOutput.ImageIds)})
	}
	return nil
}
