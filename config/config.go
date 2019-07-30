package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

var (
	// DryRun is a flag when set to true, deleting will not occur
	DryRun bool

	// MinImagesInRepo defines how many images we want to keep in each repository
	MinImagesInRepo int

	// Regions is the list of aws regions that will get pruned
	Regions []string
)

// Parse reads environment variables and initializes the config
func Parse() {
	dryRun, err := strconv.ParseBool(requiredEnv("DRY_RUN"))
	if err != nil {
		log.Fatal("Invalid value for DRY_RUN: " + err.Error())
	}

	DryRun = dryRun
	if DryRun {
		log.Println("doing dry run of pruning repos")
	}

	MinImagesInRepo, err = strconv.Atoi(requiredEnv("MIN_IMAGES"))
	if err != nil {
		log.Fatal("Invalid value for MIN_IMAGES: " + err.Error())
	}

	regionsList := requiredEnv("REGIONS")
	regions := strings.Split(regionsList, ",")
	if len(regions) == 0 {
		log.Fatal("Invalid env var: REGIONS")
	}

	Regions = regions
}

// requiredEnv tries to find a value in the environment variables. If a value is not
// found the program will panic.
func requiredEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatal("Missing env var: " + key)
	}
	return value
}
