package main

import (
	"fmt"
	"os"

	"github.com/Clever/prune-images/config"
)

type programOutput struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message"`
}

func main() {
	config.Parse()
	err := pruneRepos()
	if err != nil {
		output := programOutput{
			Success:      false,
			ErrorMessage: err.Error(),
		}
		fmt.Printf("%+v", output)
		os.Exit(1)
	}
	output := programOutput{
		Success: true,
	}
	fmt.Printf("%+v", output)
}
