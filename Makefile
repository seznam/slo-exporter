#!/usr/bin/make -f 
DOCKER_COMPOSE ?= docker-compose
src_dir        := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

all: lint build test-and-coverage

build:
	GOOS=$(OS) CGO_ENABLED=0 go build -a -ldflags "-X 'main.buildVersion=${SLO_EXPORTER_VERSION}' -X 'main.buildRevision=${CI_COMMIT_SHA}' -X 'main.buildBranch=${CI_COMMIT_BRANCH}' -X 'main.buildTag=${CI_COMMIT_TAG}' -extldflags '-static'" -o slo_exporter $(src_dir)/cmd/slo_exporter.go

lint:
	go get github.com/mgechev/revive
	revive -formatter friendly -config .revive.toml $(shell find $(src_dir) -name "*.go" | grep -v "^$(src_dir)/vendor/")

e2e-test: build
	./test/run_tests.sh

test:
	go test -v --race -coverprofile=coverage.out $(shell go list ./... | grep -v /vendor/)

test-and-coverage: test
	go tool cover -func coverage.out

compose: build clean-compose
	$(DOCKER_COMPOSE) up --force-recreate --renew-anon-volumes --abort-on-container-exit --remove-orphans --exit-code-from slo-exporter

clean-compose:
	$(DOCKER_COMPOSE) rm --force --stop -v
	docker volume rm slo-exporter_log-volume || true

.PHONY: build lint test test-and-coverage compose clean-compose e2e-test
