package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"gopkg.in/Clever/kayvee-go.v6/logger"

	pconfig "github.com/Clever/prune-images/config"
)

var (
	kv = logger.New("prune-images")
)

func pruneRepos() error {
	ctx := logger.NewContext(context.Background(), kv)

	kv.InfoD("pruning-repos", logger.M{"dry-run": pconfig.DryRun, "minImages": pconfig.MinImagesInRepo})

	regions := strings.Split(os.Getenv("REGIONS"), ",")
	if len(regions) == 0 {
		return fmt.Errorf("env variable REGIONS not set")
	}

	// Prune repositories per region
	for _, region := range pconfig.Regions {
		kv.DebugD("ecr-prune-region", logger.M{"region": region})

		cfg, err := config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
			config.WithRetryer(func() aws.Retryer {
				return aws.NopRetryer{}
			}),
		)
		if err != nil {
			return fmt.Errorf("failed to load AWS config for region %v: %v", region, err)
		}

		ecrClient := ecr.NewFromConfig(cfg)
		var pruneRepoErr error

		// Get and prune repositories in region
		paginator := ecr.NewDescribeRepositoriesPaginator(ecrClient, &ecr.DescribeRepositoriesInput{})
		for paginator.HasMorePages() {
			output, err := paginator.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to get repositories in region %v: %v", region, err)
			}

			for _, repo := range output.Repositories {
				if err := pruneRepo(ctx, ecrClient, repo); err != nil {
					pruneRepoErr = fmt.Errorf("failed to prune repo %v: %v", *repo.RepositoryName, err.Error())
					break
				}
			}
			if pruneRepoErr != nil {
				break
			}
		}
		if pruneRepoErr != nil {
			return pruneRepoErr
		}
	}

	return nil
}

func pruneRepo(ctx context.Context, ecrClient *ecr.Client, repo types.Repository) error {
	kv.DebugD("ecr-get-repo-images", logger.M{"repo": *repo.RepositoryName, "registry": *repo.RegistryId})

	images := []types.ImageDetail{}
	weekAgo := time.Now().Add(-1 * 7 * 25 * time.Hour)

	// Get all images in repo thru pagination
	paginator := ecr.NewDescribeImagesPaginator(ecrClient, &ecr.DescribeImagesInput{
		RegistryId:     repo.RegistryId,
		RepositoryName: repo.RepositoryName,
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			kv.WarnD("failed-to-get-repo-images", logger.M{"error": err.Error(), "repo": *repo.RepositoryName, "registry": *repo.RegistryId})
			return fmt.Errorf("failed-to-get-repo-images: %v", err.Error())
		}
		images = append(images, output.ImageDetails...)
	}

	// If the image limit is not reached, skip
	if len(images) <= pconfig.MinImagesInRepo {
		kv.InfoD("ecr-skip-prune-repo", logger.M{"repo": *repo.RepositoryName, "registry": *repo.RegistryId, "image_count": len(images)})
		return nil
	}

	kv.InfoD("ecr-prune-repo", logger.M{"repo": *repo.RepositoryName, "registry": *repo.RegistryId, "image_count": len(images)})

	// Sort the images from most recent to least recent
	sort.Slice(images, func(i, j int) bool {
		return images[i].ImagePushedAt.Unix() > images[j].ImagePushedAt.Unix()
	})

	imagesToDelete := []types.ImageIdentifier{}

	for i := pconfig.MinImagesInRepo; i < len(images); i++ {
		image := images[i]

		// Only remove images added more than 1 week ago.
		if image.ImagePushedAt.Before(weekAgo) {
			kv.DebugD("image-to-delete", logger.M{"repo": *repo.RepositoryName, "registry": *repo.RegistryId, "image": *image.ImageDigest})
			imagesToDelete = append(imagesToDelete, types.ImageIdentifier{
				ImageDigest: image.ImageDigest,
			})
		}
	}

	if !pconfig.DryRun {
		// Delete images in batches of 100, the maximum amount for BatchDeleteImage.
		// https://docs.aws.amazon.com/AmazonECR/latest/APIReference/API_BatchDeleteImage.html
		maxItems := 100

		for len(imagesToDelete) > 0 {
			batchToDelete := make([]types.ImageIdentifier, len(imagesToDelete))
			copy(batchToDelete, imagesToDelete)
			if len(imagesToDelete) >= maxItems {
				batchToDelete = batchToDelete[:maxItems]
				imagesToDelete = imagesToDelete[maxItems:]
			} else {
				imagesToDelete = []types.ImageIdentifier{}
			}

			batchDeleteOutput, err := ecrClient.BatchDeleteImage(ctx, &ecr.BatchDeleteImageInput{
				ImageIds:       batchToDelete,
				RegistryId:     repo.RegistryId,
				RepositoryName: repo.RepositoryName,
			})
			if err != nil {
				return fmt.Errorf("failed to delete ECR repository images %s: %s", *repo.RepositoryName, err.Error())
			}
			kv.InfoD("erc-batch-delete-images", logger.M{"count": len(batchDeleteOutput.ImageIds), "repo": *repo.RepositoryName, "registry": *repo.RegistryId})
		}
	}
	return nil
}
