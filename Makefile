#!/usr/bin/make -f 
DOCKER_IMAGE_REPO		?= seznam/slo-exporter
GITHUB_NAMESPACE		?= seznam
GITHUB_PROJECT			?= slo-exporter
SLO_EXPORTER_VERSION	?= test
OS				= linux
ARCH			= amd64
BINARY_PATH		= build/$(OS)-$(ARCH)/slo_exporter
src_dir			:= $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

.PHONY: all
all: lint build test-and-coverage

.PHONY: build
build:
	GOOS=$(OS) GOARCH=$(ARCH) CGO_ENABLED=0 go build -a -ldflags "-X 'main.buildVersion=${SLO_EXPORTER_VERSION}' -X 'main.buildRevision=${CIRCLE_SHA1}' -X 'main.buildBranch=${CIRCLE_BRANCH}' -X 'main.buildTag=${CIRCLE_TAG}' -extldflags '-static'" -o $(BINARY_PATH) $(src_dir)/cmd/slo_exporter.go

.PHONY: docker-build
docker-build:
	docker build -t $(DOCKER_IMAGE_REPO):$(SLO_EXPORTER_VERSION) .
	docker run --rm $(DOCKER_IMAGE_REPO):$(SLO_EXPORTER_VERSION) --help

.PHONY: docker-push
docker-push:
	docker push $(DOCKER_IMAGE_REPO):$(SLO_EXPORTER_VERSION)

.PHONY: github-release
github-release:
	go get github.com/github-release/github-release
	bash ci/github_release.sh

.PHONY: lint
lint:
	go get github.com/mgechev/revive
	revive -formatter friendly -config .revive.toml $(shell find $(src_dir) -name "*.go" | grep -v "^$(src_dir)/vendor/")

.PHONY: e2e-test
e2e-test: build
	./test/run_tests.sh

.PHONY: test
test:
	go test -v --race -coverprofile=coverage.out $(shell go list ./... | grep -v /vendor/)

.PHONY: benchmark
benchmark: clean
	./scripts/benchmark.sh

.PHONY: test-and-coverage
test-and-coverage: test
	go tool cover -func coverage.out

.PHONY: clean
clean:
	rm -rf slo_exporter coverage.out profile
	find . -type f -name "*.pos" -prune -exec rm -f {} \;
	find . -type d -name "test_output" -prune -exec rm -rf {} \;
