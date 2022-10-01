COMMAND?=httplock
BINARIES?=bin/$(COMMAND)
IMAGES?=docker-$(COMMAND)
ARTIFACT_PLATFORMS?=linux-amd64 linux-arm64 linux-ppc64le linux-s390x darwin-amd64 windows-amd64.exe
ARTIFACTS?=$(addprefix artifacts/$(COMMAND)-,$(ARTIFACT_PLATFORMS))
TEST_PLATFORMS?=linux/386,linux/amd64,linux/arm/v6,linux/arm/v7,linux/arm64,linux/ppc64le,linux/s390x
VCS_REF?=$(shell git rev-list -1 HEAD 2>/dev/null)
ifneq ($(shell git status --porcelain 2>/dev/null),)
  VCS_REF := $(VCS_REF)-dirty
endif
VCS_TAG?=$(shell git describe --tags --abbrev=0 2>/dev/null || echo "none")
LD_FLAGS?=-s -w -extldflags -static
GO_BUILD_FLAGS?=-trimpath -ldflags "$(LD_FLAGS)"
DOCKERFILE_EXT?=$(shell if docker build --help | grep -q -- '--progress'; then echo ".buildkit"; fi)
DOCKER_ARGS?=--build-arg "VCS_REF=$(VCS_REF)"
GOPATH?=$(shell go env GOPATH)
PWD:=$(shell pwd)
NPM?=$(shell command -v npm 2>/dev/null)
NPM_CONTAINER?=node:18
ifeq "$(strip $(NPM))" ''
	NPM=docker run --rm \
		-v "$(shell pwd)/:$(shell pwd)/" -w "$(shell pwd)" \
		-u "$(shell id -u):$(shell id -g)" \
		$(NPM_CONTAINER) npm
endif

.PHONY: all fmt vet test vendor binaries ui docker artifacts artifact-pre .FORCE

.FORCE:

all: npm-install ui fmt vet test lint swagger binaries ## Build application binaries with all setup steps

fmt: ## go fmt
	go fmt ./...

vet: ## go vet
	go vet ./...

test: ## Run unit tests
	go test ./...

go-update: ## Update go dependencies
	go get -u
	go mod tidy
	go mod vendor

lint: lint-go lint-md ## Run all linting

lint-go: $(GOPATH)/bin/staticcheck .FORCE ## Run linting for Go
	$(GOPATH)/bin/staticcheck -checks all ./...

lint-md: .FORCE ## Run linting for markdown
	docker run --rm -v "$(PWD):/workdir:ro" ghcr.io/igorshubovych/markdownlint-cli:latest \
	  --ignore vendor --ignore ui/files/node_modules .

swagger: $(GOPATH)/bin/swag .FORCE ## Update swagger docs
	$(GOPATH)/bin/swag fmt --dir ./internal/api
	$(GOPATH)/bin/swag init --dir ./internal/api -g api.go --parseInternal -o ./internal/api/docs

vendor: ## Vendor Go modules
	go mod vendor

embed/version.json: .FORCE
	# docker builds will not have the .dockerignore inside the container
	if [ -f ".dockerignore" -o ! -f "embed/version.json" ]; then \
		echo "{\"VCSRef\": \"$(VCS_REF)\", \"VCSTag\": \"$(VCS_TAG)\"}" >embed/version.json; \
	fi

binaries: vendor embed/version.json $(BINARIES) ## Build Go binaries

bin/httplock: .FORCE
	CGO_ENABLED=0 go build ${GO_BUILD_FLAGS} -o bin/httplock .

npm-install: .FORCE ## Install NPM tooling for UI
	cd ui/files && $(NPM) install --production

npm-update: .FORCE ## Update NPM dependencies
	cd ui/files && $(NPM) update

ui: .FORCE ## Build the UI files
	cd ui/files; $(NPM) run build

docker: $(IMAGES) ## Build the docker image

docker-httplock: embed/version.json .FORCE
	docker build -t httplock/httplock -f Dockerfile$(DOCKERFILE_EXT) $(DOCKER_ARGS) .
	docker build -t httplock/httplock:alpine -f Dockerfile$(DOCKERFILE_EXT) --target release-alpine $(DOCKER_ARGS) .

test-docker: $(addprefix test-docker-,$(COMMAND)) ## Test the docker multi-platform image builds

test-docker-httplock:
	docker buildx build --platform="$(TEST_PLATFORMS)" -f Dockerfile.buildkit .
	docker buildx build --platform="$(TEST_PLATFORMS)" -f Dockerfile.buildkit --target release-alpine .

artifacts: $(ARTIFACTS) ## Generate artifacts

artifact-pre:
	mkdir -p artifacts

artifacts/httplock-%: artifact-pre .FORCE
	platform_ext="$*"; \
	platform="$${platform_ext%.*}"; \
	export GOOS="$${platform%%-*}"; \
	export GOARCH="$${platform#*-}"; \
	echo export GOOS=$${GOOS}; \
	echo export GOARCH=$${GOARCH}; \
	echo go build ${GO_BUILD_FLAGS} -o "$@" .; \
	CGO_ENABLED=0 go build ${GO_BUILD_FLAGS} -o "$@" .

$(GOPATH)/bin/staticcheck: 
	go install "honnef.co/go/tools/cmd/staticcheck@latest"

$(GOPATH)/bin/swag: 
	go install "github.com/swaggo/swag/cmd/swag@latest"

help: # Display help
	@awk -F ':|##' '/^[^\t].+?:.*?##/ { printf "\033[36m%-30s\033[0m %s\n", $$1, $$NF }' $(MAKEFILE_LIST)
