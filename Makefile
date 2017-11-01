include golang.mk
include sfncli.mk
.DEFAULT_GOAL := test

SHELL := /bin/bash
PKG := github.com/Clever/prune-images
PKGS := $(shell go list ./... | grep -v /vendor)
EXECUTABLE = $(shell basename $(PKG))
SFNCLI_VERSION := latest

.PHONY: test $(PKGS) run clean vendor

$(eval $(call golang-version-check,1.8))

test: $(PKGS)

build: ./bin/sfncli
	go build -o bin/$(EXECUTABLE) $(PKG)

run: build
	  bin/sfncli --activityname $(_DEPLOY_ENV)--$(_APP_NAME) \
	  --region us-west-2 \
	  --workername `hostname` \
	  --cmd bin/prune-images

$(PKGS): golang-test-all-deps
	$(call golang-test-all,$@)

$(GOPATH)/bin/glide:
	go get -u github.com/Masterminds/glide

install_deps: $(GOPATH)/bin/glide
	@$(GOPATH)/bin/glide install -v
