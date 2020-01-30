#!/usr/bin/make -f 
DOCKER_COMPOSE ?= docker-compose
src_dir      := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

all: lint build test-and-coverage

build:
	GOOS=$(OS) CGO_ENABLED=0 go build -a -ldflags "-X gitlab.seznam.net/sklik-devops/slo-exporter/version.buildRevision=${CI_COMMIT_SHA} -X gitlab.seznam.net/sklik-devops/slo-exporter/version.buildRef=${CI_COMMIT_REF_NAME} -X gitlab.seznam.net/sklik-devops/slo-exporter/version.buildAuthor=${GITLAB_USER_LOGIN} -extldflags '-static'" -o slo_exporter $(src_dir)/cmd/slo_exporter.go

lint:
	go get github.com/mgechev/revive
	revive -formatter friendly -config .revive.toml $(shell find $(src_dir) -name "*.go" | grep -v "^$(src_dir)/vendor/")

test:
	go test -v --race -coverprofile=coverage.out $(shell go list ./... | grep -v /vendor/)

test-and-coverage: test
	go tool cover -func coverage.out

compose: build
	$(DOCKER_COMPOSE) up --force-recreate --renew-anon-volumes --abort-on-container-exit --remove-orphans --exit-code-from slo-exporter
	$(DOCKER_COMPOSE) rm --force --stop -v

.PHONY: build lint test test-and-coverage compose
