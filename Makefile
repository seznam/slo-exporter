#!/usr/bin/make -f 
SRC_DIR	:= $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
TMP_DIR ?= $(SRC_DIR)/tmp
TMP_BIN_DIR ?= $(TMP_DIR)/bin

GORELEASER_VERSION ?= v0.140.1

$(TMP_DIR):
	mkdir -p $(TMP_DIR)

$(TMP_BIN_DIR):
	mkdir -p $(TMP_BIN_DIR)

GORELEASER ?= $(TMP_BIN_DIR)/goreleaser
$(GORELEASER): $(TMP_BIN_DIR)
	@echo "Downloading goreleaser version $(GORELEASER_VERSION) to $(TMP_BIN_DIR) ..."
	@curl -sNL "https://github.com/goreleaser/goreleaser/releases/download/$(GORELEASER_VERSION)/goreleaser_Linux_x86_64.tar.gz" | tar -xzf - -C $(TMP_BIN_DIR)

RELEASE_NOTES ?= $(TMP_DIR)/release_notes
$(RELEASE_NOTES): $(TMP_DIR)
	@echo "Generating release notes to $(RELEASE_NOTES) ..."
	@csplit -q -n1 --suppress-matched -f $(TMP_DIR)/release-notes-part CHANGELOG.md '/## \[\s*v.*\]/' {1}
	@mv $(TMP_DIR)/release-notes-part1 $(RELEASE_NOTES)
	@rm $(TMP_DIR)/release-notes-part*

.PHONY: revive
revive:
	@echo "Downloading linter revive..."
	go get -u github.com/mgechev/revive

.PHONY: all
all: lint test-and-coverage build

.PHONY: lint
lint: revive
	revive -formatter friendly -config .revive.toml $(shell find $(SRC_DIR) -name "*.go" | grep -v "^$(SRC_DIR)/vendor/")

SLO_EXPORTER_BIN ?= slo_exporter
.PHONY: build
build:
	GOOS=$(OS) GOARCH=$(ARCH) CGO_ENABLED=0 go build -o $(SLO_EXPORTER_BIN) -a $(SRC_DIR)/cmd/slo_exporter.go

.PHONY: docker-build
docker: build
	docker build -t slo_exporter .

.PHONY: e2e-test
e2e-test: build
	./test/run_tests.sh

.PHONY: test
test: $(TMP_DIR)
	go test -v --race -coverprofile=$(TMP_DIR)/coverage.out $(shell go list $(SRC_DIR)/... | grep -v /vendor/)

.PHONY: benchmark
benchmark: clean
	./scripts/benchmark.sh

.PHONY: test-and-coverage
test-and-coverage: test
	go tool cover -func $(TMP_DIR)/coverage.out

.PHONY: cross-build
cross-build: $(GORELEASER)
	$(GORELEASER) build --rm-dist

.PHONY: release
release: $(RELEASE_NOTES) $(GORELEASER)
	@echo "Releasing new version do GitHub and DockerHub using goreleaser..."
	$(GORELEASER) release --rm-dist --release-notes $(RELEASE_NOTES)

.PHONY: clean
clean:
	rm -rf dist $(TMP_DIR) $(SLO_EXPORTER_BIN)
	find . -type f -name "*.pos" -prune -exec rm -f {} \;
	find . -type d -name "test_output" -prune -exec rm -rf {} \;
