SUDO := $(shell docker version >/dev/null 2>&1 || echo "sudo")

ifndef VERSION_STRING
        $(error VERSION_STRING environment variable must be set. For example "2.26.0-d2d01d39759f2f315f4af59e4b95700a4def44eb")
endif

DOCKER_PLATFORM := $(shell docker system info --format '{{.OSType}}/{{.Architecture}}')

binary-build:
	$(SUDO) docker run --platform $(DOCKER_PLATFORM) --rm $(DOCKER_OPTS) -v $(shell pwd)/../../:/go/src/github.com/stellar/go \
		--pull always \
		--env CGO_ENABLED=0 \
		--env GOFLAGS="-ldflags=-X=github.com/stellar/go/support/app.version=$(VERSION_STRING)" \
		golang:1.23-bullseye \
		/bin/bash -c '\
			git config --global --add safe.directory /go/src/github.com/stellar/go && \
			cd /go/src/github.com/stellar/go && \
			go build -o stellar-horizon -trimpath -v ./services/horizon'
