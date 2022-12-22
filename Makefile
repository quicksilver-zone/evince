#!/usr/bin/make -f
DOCKER_BUILDKIT=1
VERSION=$(shell git describe --tags | head -n1)
DOCKER := $(shell which docker)
COMMIT_HASH := $(shell git rev-parse --short=7 HEAD)
DOCKER_TAG := $(COMMIT_HASH)

linker_flags = "-s -X main.GitCommit=${COMMIT_HASH}"

build:
	go build -ldflags=${linker_flags} -o bin/evinced  -a
build-docker:
	DOCKER_BUILDKIT=1 $(DOCKER) build . -f Dockerfile -t quicksilverzone/evince:$(DOCKER_TAG)

run:
	go run -a
install:
	go build -ldflags=${linker_flags} -o /go/bin/evinced -a

all: build

