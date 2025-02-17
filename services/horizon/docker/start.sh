#!/bin/bash

set -e

# Use the dirname directly, without changing directories
if [[ $BASH_SOURCE = */* ]]; then
    DOCKER_DIR=${BASH_SOURCE%/*}/
else
    DOCKER_DIR=./
fi

echo "Docker dir is $DOCKER_DIR"

NETWORK=${1:-testnet}

case $NETWORK in
  standalone)
    DOCKER_FLAGS="-f ${DOCKER_DIR}docker-compose.yml -f ${DOCKER_DIR}docker-compose.standalone.yml"
    echo "running on standalone network"
    ;;

  pubnet)
    DOCKER_FLAGS="-f ${DOCKER_DIR}docker-compose.yml -f ${DOCKER_DIR}docker-compose.pubnet.yml"
    echo "running on public network"
    ;;

  testnet)
    DOCKER_FLAGS="-f ${DOCKER_DIR}docker-compose.yml"
    echo "running on test network"
    ;;

  *)
    echo  "$1 is not a supported option "
    exit 1
    ;;
esac

docker-compose $DOCKER_FLAGS up --build -d