package main

import (
	"os"

	"github.com/Clever/prune-images/config"
)

func main() {
	config.Parse()
	err := pruneRepos()
	if err != nil {
		os.Exit(1)
	}
}
