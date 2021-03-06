COMMAND=httplock
BINARIES=bin/$(COMMAND)
IMAGES=docker-$(COMMAND)
ARTIFACT_PLATFORMS=linux-amd64 linux-arm64 linux-ppc64le linux-s390x darwin-amd64 windows-amd64.exe
ARTIFACTS=$(addprefix artifacts/$(COMMAND)-,$(ARTIFACT_PLATFORMS))
TEST_PLATFORMS=linux/386,linux/amd64,linux/arm/v6,linux/arm/v7,linux/arm64,linux/ppc64le,linux/s390x
VCS_REF=$(shell git rev-list -1 HEAD 2>/dev/null)
ifneq ($(shell git status --porcelain 2>/dev/null),)
  VCS_REF := $(VCS_REF)-dirty
endif
VCS_TAG=$(shell git describe --tags --abbrev=0 2>/dev/null || echo "none")
LD_FLAGS=-s -w -extldflags -static
GO_BUILD_FLAGS=-trimpath -ldflags "$(LD_FLAGS)"
DOCKERFILE_EXT=$(shell if docker build --help | grep -q -- '--progress'; then echo ".buildkit"; fi)
DOCKER_ARGS=--build-arg "VCS_REF=$(VCS_REF)"
GOPATH:=$(shell go env GOPATH)
PWD:=$(shell pwd)

.PHONY: all fmt vet test vendor binaries docker artifacts artifact-pre .FORCE

.FORCE:

all: fmt vet test lint binaries

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

lint: lint-go lint-md

lint-go: $(GOPATH)/bin/staticcheck .FORCE
	$(GOPATH)/bin/staticcheck -checks all ./...

lint-md: .FORCE
	docker run --rm -v "$(PWD):/workdir:ro" ghcr.io/igorshubovych/markdownlint-cli:latest \
	  --ignore vendor .

vendor:
	go mod vendor

embed/version.json: .FORCE
	# docker builds will not have the .dockerignore inside the container
	if [ -f ".dockerignore" -o ! -f "embed/version.json" ]; then \
		echo "{\"VCSRef\": \"$(VCS_REF)\", \"VCSTag\": \"$(VCS_TAG)\"}" >embed/version.json; \
	fi

binaries: vendor embed/version.json $(BINARIES)

bin/httplock: .FORCE
	CGO_ENABLED=0 go build ${GO_BUILD_FLAGS} -o bin/httplock .

docker: $(IMAGES)

docker-httplock: embed/version.json .FORCE
	docker build -t httplock/httplock -f Dockerfile$(DOCKERFILE_EXT) $(DOCKER_ARGS) .
	docker build -t httplock/httplock:alpine -f Dockerfile$(DOCKERFILE_EXT) --target release-alpine $(DOCKER_ARGS) .

test-docker: $(addprefix test-docker-,$(COMMAND))

test-docker-httplock:
	docker buildx build --platform="$(TEST_PLATFORMS)" -f Dockerfile.buildkit .
	docker buildx build --platform="$(TEST_PLATFORMS)" -f Dockerfile.buildkit --target release-alpine .

artifacts: $(ARTIFACTS)

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
