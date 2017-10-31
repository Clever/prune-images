package common

// MinImagesInRepo defines how many images we want to keep in each repository
const MinImagesInRepo = 100

// RepoTagDescription holds a repository name and all the tags under that repo
type RepoTagDescription struct {
	RepoName string
	Tags     []TagDescription
}

type TagDescription struct {
	Name        string
	LastUpdated string
}
