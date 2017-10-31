package ecr

import (
	"fmt"

	"github.com/Clever/prune-images/common"
	"github.com/Clever/prune-images/config"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
)

// Client to interact with ECR API
type Client struct {
	service *ecr.ECR
}

func NewClient() *Client {
	return &Client{
		service: ecr.New(session.New()),
	}
}

func (c *Client) generateBatchDeleteInput(imagesToDelete []common.RepoTagDescription) []*ecr.BatchDeleteImageInput {
	var reposToPrune []*ecr.BatchDeleteImageInput
	for _, repo := range imagesToDelete {
		imagesToPrune := generateBatchDeleteImageInputRequest(repo.RepoName, repo.Tags)
		reposToPrune = append(reposToPrune, imagesToPrune)
	}

	if config.DryRun {
		fmt.Printf("Request bodies that will be sent to AWS: %+v\n", reposToPrune)
	}

	return reposToPrune
}

func generateBatchDeleteImageInputRequest(repo string, tags []common.TagDescription) *ecr.BatchDeleteImageInput {
	var imageIdentifiers []*ecr.ImageIdentifier
	var deleteRequest *ecr.BatchDeleteImageInput
	for i := range tags {
		identifier := &ecr.ImageIdentifier{
			ImageTag: &tags[i].Name,
		}
		imageIdentifiers = append(imageIdentifiers, identifier)
	}

	if len(imageIdentifiers) > 0 {
		deleteRequest = &ecr.BatchDeleteImageInput{
			ImageIds:       imageIdentifiers,
			RepositoryName: &repo,
		}
	}
	return deleteRequest
}

func (c *Client) deleteImages(reposToPrune []*ecr.BatchDeleteImageInput) []error {
	var errorAccumulator []error
	for _, repo := range reposToPrune {
		_, err := c.service.BatchDeleteImage(repo)
		if err != nil {
			errorAccumulator = append(errorAccumulator, fmt.Errorf("failed to delete ECR repository %s: %s", *repo, err.Error()))
		}
	}
	return errorAccumulator
}

// DeleteImages deletes the given images from ECR
func (c *Client) DeleteImages(imagesToDelete []common.RepoTagDescription) []error {
	// Generate batch delete input. Even if we encounter errors
	// we can still continue with the inputs that were generated
	batchDeleteInput := c.generateBatchDeleteInput(imagesToDelete)

	// Delete images
	if !config.DryRun {
		return c.deleteImages(batchDeleteInput)
	}
	return nil
}
