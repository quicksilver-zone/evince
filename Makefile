#!/usr/bin/make -f
DOCKER_BUILDKIT=1
VERSION=$(shell git describe --tags | head -n1)
DOCKER_VERSION ?= $(VERSION)
DOCKER := $(shell which docker)


build:
	go build -o bin/evinced main.go

build-docker:
	DOCKER_BUILDKIT=1 $(DOCKER) build . -f Dockerfile -t quicksilverzone/evinced:$(DOCKER_VERSION)

run:
	go run main.go
install:
	go build -o /go/bin/evinced main.go

all: build
