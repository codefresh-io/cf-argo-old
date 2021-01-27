

OUT_DIR="./dist"
BINARY_NAME="cf-argo"

VERSION="v0.0.1"
GIT_COMMIT=$(shell git rev-parse HEAD)

BASE_GIT_URL="https://github.com/noam-codefresh/argocd-production"

ifndef GOPATH
$(error GOPATH is not set, please make sure you set your GOPATH correctly!)
endif

.PHONY: build
build:
	@ OUT_DIR=$(OUT_DIR) \
	BINARY_NAME=$(BINARY_NAME) \
	VERSION=$(VERSION) \
	GIT_COMMIT=$(GIT_COMMIT) \
	BASE_GIT_URL=$(BASE_GIT_URL) \
	./hack/build.sh

$(GOPATH)/bin/golangci-lint:
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b `go env GOPATH`/bin v1.33.2

$(GOPATH)/bin/gocyclo:
	@go get github.com/fzipp/gocyclo/cmd/gocyclo

.PHONY: lint
lint: $(GOPATH)/bin/golangci-lint $(GOPATH)/bin/gocyclo
	@go mod tidy
	# Lint Go files
	@golangci-lint run --fix


.PHONY: clean
clean:
	@rm -rf dist
