

OUT_DIR="./dist"
BINARY_NAME="cf-argo"

VERSION="v0.0.1"
GIT_COMMIT=$(shell git rev-parse HEAD)

.PHONY: build
build:
	@ BINARY_NAME=$(BINARY_NAME) \
	OUT_DIR=$(OUT_DIR) \
	VERSION=$(VERSION) \
	GIT_COMMIT=$(GIT_COMMIT) ./hack/build.sh

.PHONY: clean
clean:
	@rm -rf dist
