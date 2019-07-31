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

	"github.com/Clever/prune-images/config"
)

var (
	kv = logger.New("prune-images")
)

func pruneRepos() error {
	kv.InfoD("pruning-repos", logger.M{"dry-run": config.DryRun, "minImages": config.MinImagesInRepo})

	regions := strings.Split(os.Getenv("REGIONS"), ",")
	if len(regions) == 0 {
		return fmt.Errorf("env variable REGIONS not set")
	}

	// Prune repositories per region
	for _, region := range config.Regions {
		kv.DebugD("ecr-prune-region", logger.M{"region": region})

		awsConfig := aws.NewConfig().WithMaxRetries(10)
		awsConfig.Region = &region
		ecrClient := ecr.New(session.New(), awsConfig)

		var pruneRepoErr error

		// Func to fetch and prune each repository thru pagination
		pruneRepos := func(output *ecr.DescribeRepositoriesOutput, lastPage bool) bool {
			for _, repo := range output.Repositories {
				if err := pruneRepo(ecrClient, repo); err != nil {
					pruneRepoErr = fmt.Errorf("failed to prune repo %v: %v", *repo.RepositoryName, err.Error())
					return false
				}
			}
			return !lastPage
		}

		// Get and prune repositories in region
		err := ecrClient.DescribeRepositoriesPages(&ecr.DescribeRepositoriesInput{}, pruneRepos)
		if err != nil {
			return fmt.Errorf("failed to get repositories in region %v: %v", region, err.Error())
		}
		if pruneRepoErr != nil {
			return pruneRepoErr
		}
	}

	return nil
}

func pruneRepo(ecrClient *ecr.ECR, repo *ecr.Repository) error {
	kv.DebugD("ecr-get-repo-images", logger.M{"repo": *repo.RepositoryName, "registry": *repo.RegistryId})

	images := []*ecr.ImageDetail{}

	// Get all images in repo thru pagination
	err := ecrClient.DescribeImagesPages(&ecr.DescribeImagesInput{
		RegistryId:     repo.RegistryId,
		RepositoryName: repo.RepositoryName,
	}, func(output *ecr.DescribeImagesOutput, lastPage bool) bool {
		images = append(images, output.ImageDetails...)
		return !lastPage
	})
	if err != nil {
		kv.WarnD("failed-to-get-repo-images", logger.M{"error": err.Error(), "repo": *repo.RepositoryName, "registry": *repo.RegistryId})
		return fmt.Errorf("failed-to-get-repo-images: %v", err.Error())
	}

	// If the image limit is not reached, skip
	if len(images) <= config.MinImagesInRepo {
		kv.InfoD("ecr-skip-prune-repo", logger.M{"repo": *repo.RepositoryName, "registry": *repo.RegistryId, "image_count": len(images)})
		return nil
	}

	kv.InfoD("ecr-prune-repo", logger.M{"repo": *repo.RepositoryName, "registry": *repo.RegistryId, "image_count": len(images)})

	// Sort the images from most recent to least recent
	sort.Slice(images, func(i, j int) bool {
		return images[i].ImagePushedAt.Unix() > images[j].ImagePushedAt.Unix()
	})

	imagesToDelete := []*ecr.ImageIdentifier{}

	for i := config.MinImagesInRepo; i < len(images); i++ {
		image := images[i]
		kv.DebugD("image-to-delete", logger.M{"repo": *repo.RepositoryName, "registry": *repo.RegistryId, "image": *image.ImageDigest})
		imagesToDelete = append(imagesToDelete, &ecr.ImageIdentifier{
			ImageDigest: image.ImageDigest,
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
		kv.InfoD("erc-batch-delete-images", logger.M{"count": len(batchDeleteOutput.ImageIds), "repo": *repo.RepositoryName, "registry": *repo.RegistryId})
	}
	return nil
}
