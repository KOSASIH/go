#! /bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

export HORIZON_INTEGRATION_TESTS_ENABLED=true
export HORIZON_INTEGRATION_TESTS_ENABLE_CAPTIVE_CORE=${HORIZON_INTEGRATION_TESTS_ENABLE_CAPTIVE_CORE:-}
export HORIZON_INTEGRATION_TESTS_CAPTIVE_CORE_USE_DB=${HORIZON_INTEGRATION_TESTS_CAPTIVE_CORE_USE_DB:-}
export HORIZON_INTEGRATION_TESTS_CAPTIVE_CORE_BIN=${HORIZON_INTEGRATION_TESTS_CAPTIVE_CORE_BIN:-/usr/bin/stellar-core}
export TRACY_NO_INVARIANT_CHECK=1 # This fails on my dev vm. - Paul

# launch postgres if it's not already.
if [[ "$(docker inspect integration_postgres -f '{{.State.Running}}')" != "true" ]]; then
  docker rm -f integration_postgres || true;
  docker run -d \
    --name integration_postgres \
    --platform linux/amd64 \
    --env POSTGRES_HOST_AUTH_METHOD=trust \
    -p 5432:5432 \
    postgres:12-bullseye
fi

exec go test -timeout 35m github.com/stellar/go/services/horizon/internal/integration/... "$@"
