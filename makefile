NAME = csi-driver-rclone

# This is the only place versions are defined, and is plumbed when deploying.
VERSION = v1.0.0

GIT_COMMIT = $(shell git rev-parse HEAD)
BUILD_DATE = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

GO_PKG = github.com/cornfeedhobo/${NAME}/internal/csirclone
GO_LDFLAGS = -X ${GO_PKG}.driverVersion=${VERSION} -X ${GO_PKG}.gitCommit=${GIT_COMMIT} -X ${GO_PKG}.buildDate=${BUILD_DATE}
GO_EXT_LDFLAGS = -s -w -extldflags '-static'

GO_OS ?= darwin linux windows
GO_ARCH ?= amd64 arm arm64
GO_PARALLEL ?= 1

DOCKER_FLAGS ?= --tag=csi-driver-rclone:latest


.PHONY: help
help:
	@echo 'Please choose a valid make target'


.PHONY: unittest
unittest:
	go test -covermode=count -coverprofile=profile.cov ./internal/... -v


.PHONY: build
build: unittest
	go install github.com/authelia/gox@latest
	gox \
		-output="./bin/{{.Dir}}_{{.OS}}_{{.Arch}}" \
		-os="${GO_OS}" \
		-arch="${GO_ARCH}" \
		-ldflags="${GO_LDFLAGS} ${GO_EXT_LDFLAGS}" \
		-parallel=${GO_PARALLEL} \
		-trimpath \
		./cmd/csi-driver-rclone


.PHONY: container-only
container-only:
	docker buildx build \
		--pull \
		--provenance=false \
		--sbom=false \
		--build-arg=BUILD_DATE=${BUILD_DATE},BUILD_VERSION=${VERSION},BUILD_COMMIT=${GIT_COMMIT} \
		$(DOCKER_FLAGS) .

.PHONY: container
container: build container-only


.PHONY: helm-release
helm-release:
	helm package deploy/helm/csi-driver-rclone \
		--destination=deploy/helm/charts/ \
		--app-version=${VERSION} \
		--version=${VERSION}
	helm repo index deploy/helm/charts/
