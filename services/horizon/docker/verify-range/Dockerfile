FROM ubuntu:22.04

ARG GO_VERSION
ARG STELLAR_CORE_VERSION 
ENV STELLAR_CORE_VERSION=${STELLAR_CORE_VERSION:-*}
# to remove tzdata interactive flow
ENV DEBIAN_FRONTEND=noninteractive

ADD dependencies /
RUN ["chmod", "+x", "/dependencies"]
RUN /dependencies

ADD stellar-core.cfg /
ADD captive-core-pubnet.cfg /
RUN mkdir -p /cc
RUN mkdir -p /data

ADD start /
RUN ["chmod", "+x", "start"]

ENTRYPOINT ["/start"]
