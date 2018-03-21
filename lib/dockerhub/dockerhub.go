package dockerhub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/Clever/prune-images/common"
	"gopkg.in/Clever/kayvee-go.v5/logger"
)

const (
	dockerHubNamespace = "clever"
	retryAttempts      = 3
)

var kv = logger.New("prune-images")

// Client to interact with DockerHub API
type Client struct {
	baseURL  string
	username string
	password string
	token    string
	client   *http.Client
	dryrun   bool
}

// RequestError is the error when an HTTP request fails (response code >= 400)
type RequestError struct {
	Message    string
	StatusCode int
}

type tagResults struct {
	Next    string       `json:"next"`
	Results []tagDetails `json:"results"`
}

type tagDetails struct {
	Name        string `json:"name"`
	LastUpdated string `json:"last_updated"`
}

type repoResults struct {
	Next    string        `json:"next"`
	Results []repoDetails `json:"results"`
}

type repoDetails struct {
	Name        string `json:"name"`
	LastUpdated string `json:"last_updated"`
}

func (e *RequestError) Error() string {
	return fmt.Sprintf("%s. status code: %d", e.Message, e.StatusCode)
}

// NewClient creates a DockerHub client
func NewClient(username string, password string, dryrun bool) *Client {
	return &Client{
		baseURL:  "https://hub.docker.com/v2/",
		token:    "",
		username: username,
		password: password,
		client:   &http.Client{},
		dryrun:   dryrun,
	}
}

// loginParams are used when calling Login
type loginParams struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// loginResponse is the response JSON from a successful call to Login
type loginResponse struct {
	Token string `json:"token"`
}

// Login does a login request to DockerHub, and sets the token on the client
func (c *Client) Login() error {
	// setup request
	b, err := json.Marshal(loginParams{
		Username: c.username,
		Password: c.password,
	})
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(b)
	postReq, err := http.NewRequest("POST", c.baseURL+"users/login/", buf)
	if err != nil {
		return err
	}
	postReq.Header.Add("Content-Type", "application/json")

	// make request
	resp, err := c.client.Do(postReq)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return &RequestError{Message: "error logging into DockerHub", StatusCode: resp.StatusCode}
	}

	// on successful login, get response and set client's token
	var lr loginResponse
	err = json.NewDecoder(resp.Body).Decode(&lr)
	if err != nil {
		return err
	}
	c.token = "JWT " + lr.Token
	return err
}

func (c *Client) GetAllRepos() ([]repoDetails, error) {
	var allDetails []repoDetails
	var result repoResults
	err := c.getResultsFromURL(fmt.Sprintf("%srepositories/%s/?page_size=100", c.baseURL, dockerHubNamespace), &result)
	if err != nil {
		return nil, err
	}

	allDetails = append(allDetails, result.Results...)

	nextURL := result.Next
	for nextURL != "" {
		var currentResult repoResults
		err = c.getResultsFromURL(nextURL, &currentResult)
		if err != nil {
			return nil, err
		}
		allDetails = append(allDetails, currentResult.Results...)
		nextURL = currentResult.Next
	}

	return allDetails, nil
}

func (c *Client) GetAllTags(repo string) ([]tagDetails, error) {
	var allDetails []tagDetails
	var result tagResults
	err := c.getResultsFromURL(fmt.Sprintf("%srepositories/%s/%s/tags/?page_size=100", c.baseURL, dockerHubNamespace, repo), &result)
	if err != nil {
		return nil, err
	}

	allDetails = append(allDetails, result.Results...)

	nextURL := result.Next
	for nextURL != "" {
		var currentResult tagResults
		err = c.getResultsFromURL(nextURL, &currentResult)
		if err != nil {
			return nil, err
		}
		allDetails = append(allDetails, currentResult.Results...)
		nextURL = currentResult.Next
	}

	return allDetails, nil
}

func (c *Client) getResultsFromURL(url string, result interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	err = json.NewDecoder(resp.Body).Decode(result)
	if err != nil {
		return err
	}

	return nil
}

// DeleteImage deletes an image from DockerHub
func (c *Client) DeleteImage(repo, tag string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%srepositories/%s/%s/tags/%s/", c.baseURL, dockerHubNamespace, repo, tag), nil)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", c.token)

	// make request
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return &RequestError{Message: "error deleting repo in DockerHub", StatusCode: resp.StatusCode}
	}
	return nil
}

func (c *Client) PruneAllRepos() ([]common.RepoTagDescription, error) {
	kv.Info("get-all-docker-repos")
	repos, err := c.GetAllRepos()
	if err != nil {
		return nil, err
	}
	var reposWithTags []common.RepoTagDescription
	kv.Info("get-all-docker-tags")
	for _, repo := range repos {
		tags, err := c.getAllTagsWithBackoff(retryAttempts, repo.Name)
		if err != nil {
			log.Printf("failed to get tags from Docker Hub for repo %s: %s", repo, err.Error())
			continue
		}

		var allTags []common.TagDescription
		for _, tag := range tags {
			currentTag := common.TagDescription{
				Name:        tag.Name,
				LastUpdated: tag.LastUpdated,
			}
			allTags = append(allTags, currentTag)
		}
		repoTagDescription := common.RepoTagDescription{
			RepoName: repo.Name,
			Tags:     allTags,
		}
		reposWithTags = append(reposWithTags, repoTagDescription)
	}
	// Keep track of images that were actually deleted
	kv.Info("delete-docker-repos")
	var deletedImages []common.RepoTagDescription
	for _, repo := range reposWithTags {
		if len(repo.Tags) >= common.MinImagesInRepo {
			currentRepo := common.RepoTagDescription{
				RepoName: repo.RepoName,
			}

			var deletedTags []common.TagDescription
			tags := repo.Tags
			// Sort the images from most recent to least recent
			sort.Slice(tags, func(i, j int) bool {
				return tags[i].LastUpdated > tags[j].LastUpdated
			})
			for i := common.MinImagesInRepo; i < len(tags); i++ {
				if !c.dryrun {
					err = c.deleteImageWithBackoff(retryAttempts, repo.RepoName, tags[i].Name)
					if err != nil {
						log.Printf("failed to delete %s:%s from Docker Hub: %s", repo, tags[i].Name, err.Error())
						continue
					}
				}
				deletedTags = append(deletedTags, common.TagDescription{
					Name:        tags[i].Name,
					LastUpdated: tags[i].LastUpdated,
				})
			}
			currentRepo.Tags = deletedTags
			deletedImages = append(deletedImages, currentRepo)
		}
	}

	return deletedImages, nil
}

func (c *Client) getAllTagsWithBackoff(timesToRetry int, repo string) ([]tagDetails, error) {
	sleepDuration := 1 * time.Second
	var tags []tagDetails
	var err error
	for i := 0; i < timesToRetry; i++ {
		if tags, err = c.GetAllTags(repo); err != nil {
			log.Printf("attempt %d/%d: failed to get tags from Docker Hub for repo %s: %s", i+1, timesToRetry, repo, err.Error())
			time.Sleep(sleepDuration)
			sleepDuration *= 2
		} else {
			break
		}
	}
	return tags, err
}

func (c *Client) deleteImageWithBackoff(timesToRetry int, repo, tag string) error {
	sleepDuration := 1 * time.Second
	var err error
	for i := 0; i < timesToRetry; i++ {
		if err = c.DeleteImage(repo, tag); err != nil {
			log.Printf("attempt %d/%d: failed to delete %s:%s from Docker Hub: %s", i+1, timesToRetry, repo, tag, err.Error())
			time.Sleep(sleepDuration)
			sleepDuration *= 2
		} else {
			break
		}
	}
	return err
}
