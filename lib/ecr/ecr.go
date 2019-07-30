package ecr

import (
	"fmt"
	"log"

	"github.com/Clever/prune-images/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"gopkg.in/Clever/kayvee-go.v6/logger"
)

var kv = logger.New("prune-images")

// Client to interact with ECR API
type Client struct {
	service *ecr.ECR
	dryrun  bool
}

func NewClient(dryrun bool, region string) *Client {
	awsConfig := aws.NewConfig().WithMaxRetries(10)
	awsConfig.Region = &region
	return &Client{
		service: ecr.New(session.New(), awsConfig),
		dryrun:  dryrun,
	}
}

func (c *Client) generateBatchDeleteInput(repoTags []common.RepoTagDescription) []*ecr.BatchDeleteImageInput {
	var batchDelete []*ecr.BatchDeleteImageInput
	for _, rt := range repoTags {
		imagesToPrune := generateBatchDeleteImageInputRequest(rt.RepoName, rt.Tags)
		if imagesToPrune != nil {
			batchDelete = append(batchDelete, imagesToPrune)
		}
	}

	if c.dryrun {
		fmt.Printf("Request bodies that will be sent to AWS: %+v\n", batchDelete)
	}

	return batchDelete
}

func generateBatchDeleteImageInputRequest(repo string, tags []common.TagDescription) *ecr.BatchDeleteImageInput {
	var imageIdentifiers []*ecr.ImageIdentifier
	var deleteRequest *ecr.BatchDeleteImageInput
	for i := range tags {
		identifier := &ecr.ImageIdentifier{
			ImageTag: &tags[i].Name,
		}
		imageIdentifiers = append(imageIdentifiers, identifier)

		kv.InfoD("ecr-queued-delete", logger.M{"image": identifier.ImageTag})
	}

	if len(imageIdentifiers) > 0 {
		deleteRequest = &ecr.BatchDeleteImageInput{
			ImageIds:       imageIdentifiers,
			RepositoryName: &repo,
		}
	}
	return deleteRequest
}

func (c *Client) deleteImages(inputs []*ecr.BatchDeleteImageInput) {
	for _, input := range inputs {
		_, err := c.service.BatchDeleteImage(input)
		if err != nil {
			log.Printf("failed to delete ECR repository %s: %s", input, err.Error())
		}

		kv.InfoD("erc-batch-delete-images", logger.M{"count": len(input.ImageIds)})
	}
}

// DeleteImages deletes the given images from ECR
func (c *Client) DeleteImages(imagesToDelete []common.RepoTagDescription) {
	// Generate batch delete input. Even if we encounter errors
	// we can still continue with the inputs that were generated
	batchDeleteInput := c.generateBatchDeleteInput(imagesToDelete)

	// Delete images
	if !c.dryrun {
		c.deleteImages(batchDeleteInput)
	}
}
