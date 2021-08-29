COMMAND=httplock
BINARIES=bin/$(COMMAND)
IMAGES=docker-$(COMMAND)
ARTIFACT_PLATFORMS=linux-amd64 linux-arm64 linux-ppc64le linux-s390x darwin-amd64 windows-amd64.exe
ARTIFACTS=$(addprefix artifacts/$(COMMAND)-,$(ARTIFACT_PLATFORMS))
TEST_PLATFORMS=linux/386,linux/amd64,linux/arm/v6,linux/arm/v7,linux/arm64,linux/ppc64le,linux/s390x
VCS_REF=$(shell git rev-list -1 HEAD)
VCS_TAG=$(shell git describe --tags --abbrev=0 2>/dev/null || echo "none")
DOCKERFILE_EXT=$(shell if docker build --help | grep -q -- '--progress'; then echo ".buildkit"; fi)
DOCKER_ARGS=--build-arg "VCS_REF=$(VCS_REF)"

.PHONY: all fmt vet test vendor binaries docker artifacts artifact-pre .FORCE

.FORCE:

all: fmt vet test binaries

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

vendor:
	go mod vendor

binaries: vendor $(BINARIES)

bin/httplock: .FORCE
	CGO_ENABLED=0 go build ${GO_BUILD_FLAGS} -o bin/httplock .

docker: $(IMAGES)

docker-httplock: .FORCE
	docker build -t httplock/httplock -f Dockerfile$(DOCKERFILE_EXT) $(DOCKER_ARGS) .
	docker build -t httplock/httplock:alpine -f Dockerfile$(DOCKERFILE_EXT) --target release-alpine $(DOCKER_ARGS) .

test-docker: $(addprefix test-docker-,$(COMMANDS))

test-docker-httplock:
	docker buildx build --platform="$(TEST_PLATFORMS)" -f Dockerfile.buildkit .
	docker buildx build --platform="$(TEST_PLATFORMS)" -f Dockerfile.buildkit --target release-alpine .

artifacts: $(ARTIFACTS)

artifact-pre:
	mkdir -p artifacts

artifacts/httplock-%: artifact-pre .FORCE
	platform="$*"; \
	export GOOS="$${platform%%-*}"; \
	export GOARCH="$${platform#*-}"; \
	echo export GOOS=$${GOOS}; \
	echo export GOARCH=$${GOARCH}; \
	echo go build ${GO_BUILD_FLAGS} -o "$@" .; \
	CGO_ENABLED=0 go build ${GO_BUILD_FLAGS} -o "$@" .
