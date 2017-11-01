package main

import (
	"encoding/json"
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
	output := programOutput{
		Success: true,
	}
	if err != nil {
		output = programOutput{
			Success:      false,
			ErrorMessage: err.Error(),
		}
	}
	b, _ := json.Marshal(output)
	os.Stdout.Write(b)

	if err != nil {
		os.Exit(1)
	}
}
