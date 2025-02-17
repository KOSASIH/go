# install Core from apt "stable" or "unstable" pool, and horizon from apt "testing" pool
FROM ubuntu:focal

ARG VERSION
ARG STELLAR_CORE_VERSION
ARG DEBIAN_FRONTEND=noninteractive
ARG ALLOW_CORE_UNSTABLE=no

RUN apt-get update && apt-get install -y wget apt-transport-https gnupg2 && \
    wget -qO /etc/apt/trusted.gpg.d/SDF.asc https://apt.stellar.org/SDF.asc && \
    echo "deb https://apt.stellar.org focal stable" | tee -a /etc/apt/sources.list.d/SDF.list && \
    if [ "${ALLOW_CORE_UNSTABLE}" = "yes" ]; then echo "deb https://apt.stellar.org focal unstable" | tee -a /etc/apt/sources.list.d/SDF.list; fi && \
    cat /etc/apt/sources.list.d/SDF.list && \
    apt-get update && apt-cache madison stellar-core && eval "apt-get install -y stellar-core${STELLAR_CORE_VERSION+=$STELLAR_CORE_VERSION}" && \
    if [ "${ALLOW_CORE_UNSTABLE}" = "yes" ]; then sed -i '/unstable/d' /etc/apt/sources.list.d/SDF.list; fi && \
    cat /etc/apt/sources.list.d/SDF.list && \
    echo "deb https://apt.stellar.org focal testing" | tee -a /etc/apt/sources.list.d/SDF.list && \
    apt-get update && apt-cache madison stellar-horizon && apt-get install -y stellar-horizon=${VERSION} && \
    apt-get clean && rm -rf /var/lib/apt/lists/* /var/log/*.log /var/log/*/*.log

EXPOSE 8000
ENTRYPOINT ["/usr/bin/stellar-horizon"]
