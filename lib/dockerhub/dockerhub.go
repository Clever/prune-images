package dockerhub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/Clever/prune-images/common"
	"github.com/Clever/prune-images/config"
)

// Client to interact with DockerHub API
type Client struct {
	baseURL  string
	username string
	password string
	token    string
	client   *http.Client
}

// RequestError is the error when an HTTP request fails (response code >= 400)
type RequestError struct {
	Message    string
	StatusCode int
}

type requestResults struct {
	Next    string           `json:"next"`
	Results []requestDetails `json:"results"`
}

type requestDetails struct {
	Name        string `json:"name"`
	LastUpdated string `json:"last_updated"`
}

func (e *RequestError) Error() string {
	return fmt.Sprintf("%s. status code: %d", e.Message, e.StatusCode)
}

// NewClient creates a DockerHub client
func NewClient(username string, password string) *Client {
	return &Client{
		baseURL:  "https://hub.docker.com/v2/",
		token:    "",
		username: username,
		password: password,
		client:   &http.Client{},
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

func (c *Client) GetAllRepos() ([]requestDetails, error) {
	return c.getDetailsFromURL(fmt.Sprintf("%srepositories/%s/?page_size=100", c.baseURL, config.DockerHubNamespace))
}

func (c *Client) GetAllTags(repo string) ([]requestDetails, error) {
	return c.getDetailsFromURL(fmt.Sprintf("%srepositories/%s/%s/tags/?page_size=100", c.baseURL, config.DockerHubNamespace, repo))
}

func (c *Client) getDetailsFromURL(url string) ([]requestDetails, error) {
	var allDetails []requestDetails
	tagResponse, err := c.getResultsFromURL(url)
	if err != nil {
		return nil, err
	}

	allDetails = append(allDetails, tagResponse.Results...)

	for tagResponse.Next != "" {
		tagResponse, err = c.getResultsFromURL(tagResponse.Next)
		if err != nil {
			return nil, err
		}
		allDetails = append(allDetails, tagResponse.Results...)
	}

	return allDetails, nil
}

func (c *Client) getResultsFromURL(url string) (requestResults, error) {
	var result requestResults
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return result, err
	}

	req.Header.Add("Authorization", c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return result, err
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return result, err
	}

	return result, nil
}

// DeleteImage deletes an image from DockerHub
func (c *Client) DeleteImage(repo, tag string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%srepositories/%s/%s/tags/%s/", c.baseURL, config.DockerHubNamespace, repo, tag), nil)
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

func (c *Client) PruneAllRepos() ([]common.RepoTagDescription, []error) {
	var errorAccumulator []error
	repos, err := c.GetAllRepos()
	if err != nil {
		return nil, []error{err}
	}
	var reposWithTags []common.RepoTagDescription
	for _, repo := range repos {
		tags, err := c.GetAllTags(repo.Name)
		if err != nil {
			errorAccumulator = append(errorAccumulator, fmt.Errorf("failed to get tags from Docker Hub for repo %s: %s", repo, err.Error()))
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
				if !config.DryRun {
					err = c.DeleteImage(repo.RepoName, tags[i].Name)
					if err != nil {
						errorAccumulator = append(errorAccumulator, fmt.Errorf("failed to delete %s:%s from Docker Hub: %s", repo, tags[i].Name, err.Error()))
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

	return deletedImages, errorAccumulator
}
