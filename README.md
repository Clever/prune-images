# prune-images

Prune-images is a worker that removes old images from Amazon ECR and Docker Hub. It will retain the 100 most recent images on Docker Hub and remove the rest. All of the images that were removed on Docker Hub will also be removed on Amazon ECR.

## Deploying

To run on production:
```
ark start prune-images -e production
```

**Important**: when a job is submitted to a worker that is running on production, the images that qualify for pruning will actually be removed. There is a `DRY_RUN` environment variable that is available which will return all the images that will be deleted, but will not perform the delete.

## Required Environment Variables

`DRY_RUN`: can be either `true` or `false`. Indicateds whether or not the delete will occur for images that qualify for pruning. If `true`, then the worker's output will print the delete calls that will be made to Amazon ECR.
test
