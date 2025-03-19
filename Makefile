PROJECTNAME := datadog-csi-driver
DOCKER_IMAGE ?= $(PROJECTNAME)
VERSION ?= dev
PLATFORM ?= linux/amd64,linux/arm64
LABELS ?= target=build,env=development
RELEASE_IMAGE_TAG := $(if $(CI_COMMIT_TAG),--tag $(RELEASE_IMAGE),)

build:
	docker buildx build \
	  --platform=$(PLATFORM) \
	  $(foreach label,$(LABELS),--label $(label)) \
	  --tag $(DOCKER_IMAGE) \
	  --push \
	  .

docker-buildx-ci:
	docker buildx build . \
	  --build-arg LDFLAGS="-X 'github.com/Datadog/datadog-csi-driver/cmd.version=$(CI_COMMIT_TAG)'" \
	  --platform=linux/arm64,linux/amd64 \
	  --label target=build \
	  --push \
	  --tag ${IMG} ${RELEASE_IMAGE_TAG}

test:
	go test -v ./...

.PHONY: build
.PHONY: docker-buildx-ci
.PHONY: test
.PHONY: release

