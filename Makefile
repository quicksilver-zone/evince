#!/usr/bin/make -f
DOCKER_BUILDKIT=1
VERSION=$(shell git describe --tags | head -n1)
DOCKER := $(shell which docker)
COMMIT_HASH := $(shell git rev-parse --short=7 HEAD)
DOCKER_TAG := $(COMMIT_HASH)

linker_flags = "-s -X main.GitCommit=${COMMIT_HASH}"
export GO111MODULE = on



build: go.sum
	go build -ldflags=${linker_flags} -o bin/evinced  -a
build-docker: go.sum
	DOCKER_BUILDKIT=1 $(DOCKER) build . -f Dockerfile -t quicksilverzone/evince:$(DOCKER_TAG)

go.sum: go.mod
	echo "Ensure dependencies have not been modified ..." >&2
	go mod verify
	go mod tidy

run:
	go run -a
install:
	go build -ldflags=${linker_flags} -o /go/bin/evinced -a

all: build

###############################################################################
###                                Linting                                  ###
###############################################################################

lint:
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint run --out-format=tab

lint-fix:
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint run --fix --out-format=tab --issues-exit-code=0

.PHONY: lint lint-fix

format:
	@find . -name '*.go' -type f -not -path "*.git*" | xargs go run mvdan.cc/gofumpt -w .
	@find . -name '*.go' -type f -not -path "*.git*" | | xargs go run github.com/client9/misspell/cmd/misspell -w
	@find . -name '*.go' -type f -not -path "*.git*" |  xargs go run golang.org/x/tools/cmd/goimports -w -local github.com/ingenuity-build/evince
.PHONY: format

mdlint:
	@echo "--> Running markdown linter"
	@$(DOCKER) run -v $(PWD):/workdir ghcr.io/igorshubovych/markdownlint-cli:latest "**/*.md"

mdlint-fix:
	@$(DOCKER)  run -v $(PWD):/workdir ghcr.io/igorshubovych/markdownlint-cli:latest "**/*.md" --fix