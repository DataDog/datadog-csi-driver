PROJECTNAME := datadog-csi-driver
DOCKER_IMAGE ?= $(PROJECTNAME)
VERSION ?= dev
PLATFORM ?= linux/amd64,linux/arm64
LABELS ?= target=build,env=development
RELEASE_IMAGE_TAG := $(if $(CI_COMMIT_TAG),--tag $(RELEASE_IMAGE),)

UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

KIND_CMD := $(shell which kind || true)
KIND_VERSION := v0.18.0

# Determine the correct binary based on OS and architecture
ifeq ($(UNAME_S),Linux)
	KIND_OS := linux
endif
ifeq ($(UNAME_S),Darwin)
	KIND_OS := darwin
endif

ifeq ($(UNAME_M),x86_64)
	KIND_ARCH := amd64
endif
ifeq ($(UNAME_M),arm64)
	KIND_ARCH := arm64
endif

KIND_URL := https://github.com/kubernetes-sigs/kind/releases/download/$(KIND_VERSION)/kind-$(KIND_OS)-$(KIND_ARCH)

install-kind:
	@if [ -z "$(KIND_CMD)" ]; then \
		echo "Installing kind for $(KIND_OS)-$(KIND_ARCH)..."; \
		curl -Lo kind $(KIND_URL); \
		chmod +x kind; \
		mv kind /usr/local/bin/; \
	else \
		echo "kind is already installed at $(KIND_CMD)."; \
	fi

build:
	docker buildx build \
	  --platform=$(PLATFORM) \
	  $(foreach label,$(LABELS),--label $(label)) \
	  --tag $(DOCKER_IMAGE) \
	  --load .

docker-buildx-ci:
	docker buildx build . \
	  --build-arg LDFLAGS="-X 'main.Version=$(CI_COMMIT_TAG)'" \
	  --platform=linux/arm64,linux/amd64 \
	  --label target=build \
	  --push \
	  --tag ${IMG} ${RELEASE_IMAGE_TAG}

test:
	go test -v -count=1 ./...

e2e: install-kind
	./test/e2e/setup-env.sh
	kubectl apply -f test/e2e/manifests -n default

.PHONY: build
.PHONY: docker-buildx-ci
.PHONY: test
.PHONY: e2e
.PHONY: install-kind
