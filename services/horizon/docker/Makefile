SUDO := $(shell docker version >/dev/null 2>&1 || echo "sudo")

# https://github.com/opencontainers/image-spec/blob/master/annotations.md
BUILD_DATE := $(shell date -u +%FT%TZ)

TAG ?= stellar/stellar-horizon:$(VERSION)

# build with Core from apt "stable" or "unstable", and horizon from apt "testing"
docker-build:
ifndef VERSION
	$(error VERSION environment variable must be set. For example VERSION=2.4.1-101 )
endif
ifndef STELLAR_CORE_VERSION
	$(SUDO) docker build --pull $(DOCKER_OPTS) \
	--label org.opencontainers.image.created="$(BUILD_DATE)" \
	--build-arg VERSION=$(VERSION) --build-arg ALLOW_CORE_UNSTABLE=$(ALLOW_CORE_UNSTABLE) \
	-t $(TAG) .
else
	$(SUDO) docker build --pull $(DOCKER_OPTS) \
	--label org.opencontainers.image.created="$(BUILD_DATE)" \
	--build-arg VERSION=$(VERSION) --build-arg STELLAR_CORE_VERSION=$(STELLAR_CORE_VERSION) \
        --build-arg ALLOW_CORE_UNSTABLE=$(ALLOW_CORE_UNSTABLE) \
	-t $(TAG) .
endif

# build Core and Horizon from apt "testing"
docker-build-core-testing:
ifndef VERSION
	$(error VERSION environment variable must be set. For example VERSION=2.4.1-101 )
endif
ifndef STELLAR_CORE_VERSION
	$(error STELLAR_CORE_VERSION environment variable must be set. )
else
	$(SUDO) docker build --file ./Dockerfile.core-testing --pull $(DOCKER_OPTS) \
	--label org.opencontainers.image.created="$(BUILD_DATE)" \
	--build-arg VERSION=$(VERSION) --build-arg STELLAR_CORE_VERSION=$(STELLAR_CORE_VERSION) \
	-t $(TAG) .
endif

# build Core and Horizon from apt "stable"
docker-build-core-stable:
ifndef VERSION
	$(error VERSION environment variable must be set. For example VERSION=2.4.1-101 )
endif
ifndef STELLAR_CORE_VERSION
	$(error STELLAR_CORE_VERSION environment variable must be set. )
else
	$(SUDO) docker build --file ./Dockerfile.stable --pull $(DOCKER_OPTS) \
	--label org.opencontainers.image.created="$(BUILD_DATE)" \
	--build-arg VERSION=$(VERSION) --build-arg STELLAR_CORE_VERSION=$(STELLAR_CORE_VERSION) \
	-t $(TAG) .
endif

docker-push:
ifndef TAG
	$(error Must set VERSION or TAG environment variable. For example VERSION=2.4.1-101 )
endif
	$(SUDO) docker push $(TAG)
