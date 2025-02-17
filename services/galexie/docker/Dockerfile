FROM golang:1.23-bookworm AS builder

WORKDIR /go/src/github.com/stellar/go

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY . ./

ARG GOFLAGS
RUN go install github.com/stellar/go/services/galexie

FROM ubuntu:22.04
ARG STELLAR_CORE_VERSION
ENV STELLAR_CORE_VERSION=${STELLAR_CORE_VERSION:-*}
ENV STELLAR_CORE_BINARY_PATH /usr/bin/stellar-core

ENV DEBIAN_FRONTEND=noninteractive
# ca-certificates are required to make tls connections
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl wget gnupg apt-utils
RUN wget -qO - https://apt.stellar.org/SDF.asc | APT_KEY_DONT_WARN_ON_DANGEROUS_USAGE=true apt-key add -
RUN echo "deb https://apt.stellar.org focal stable" >/etc/apt/sources.list.d/SDF.list
RUN echo "deb https://apt.stellar.org focal unstable" >/etc/apt/sources.list.d/SDF-unstable.list
RUN apt-get update && apt-get install -y stellar-core=${STELLAR_CORE_VERSION}
RUN apt-get clean

COPY --from=builder /go/bin/galexie /usr/bin/galexie

ENTRYPOINT ["/usr/bin/galexie"]

CMD ["--help"]

