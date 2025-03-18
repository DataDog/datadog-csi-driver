PROJECTNAME := datadog-csi-driver
DOCKER_IMAGE ?= $(PROJECTNAME)
PLATFORM ?= linux/amd64,linux/arm64
LABELS ?= target=build,env=development

build:
	docker buildx build \
	  --platform=$(PLATFORM) \
	  $(foreach label,$(LABELS),--label $(label)) \
	  --tag $(DOCKER_IMAGE) \
	  .

docker-buildx-ci:
	docker buildx build . --build-arg LDFLAGS="${LDFLAGS}" --platform=linux/arm64,linux/amd64 --label target=build --push --tag ${IMG} ${RELEASE_IMAGE_TAG}

test:
	go test -v ./...

.PHONY: build
.PHONY: docker-buildx-ci
.PHONY: test

