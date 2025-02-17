### Contract integration tests use rpc preflight
The contract integration tests depend on soroban rpc for preflight requests, two additional environment variables must be set to enable soroban rpc server to be launced in a separate docker container:
```
HORIZON_INTEGRATION_TESTS_SOROBAN_RPC_DOCKER_IMG=stellar/soroban-rpc
HORIZON_INTEGRATION_TESTS_ENABLE_SOROBAN_RPC=true
```

The `stellar/soroban-rpc` refers to an image built from soroban-tools/cmd/soroban-rpc/docker/Dockerfile and published on public `docker.io` so it is referrable in any build environment. Images are published to `docker.io/stellar/soroban-rpc` on a release basis, if you need more recent build, can build interim images from soroban-tools/cmd/soroban-rpc/docker/Dockerfile, example:

```
docker build --platform linux/amd64 --build-arg STELLAR_CORE_VERSION=19.11.1-1373.875f47e24.focal~soroban -t stellar-soroban-rpc:test -f cmd/soroban-rpc/docker/Dockerfile .
```

`STELLAR_CORE_VERSION` should be set to a debian package version for `stellar-core`.

### Contract test fixture source code

The existing integeration tests refer to .wasm files from the `testdata/` directory location.

#### Any time contract code changes, follow these steps to rebuild the test WASM fixtures:

1. First install latest rust toolchain:
https://www.rust-lang.org/tools/install

2. Update the [`Cargo.toml file`](./Cargo.toml) to have latest git refs to
[`rs-soroban-sdk`](https://github.com/stellar/rs-soroban-sdk) for the `soroban-sdk` and `soroban-auth` dependencies.

3. Compile the contract source code to WASM and copy it to `testdata/`:

```bash
cd ./services/horizon/internal/integration/contracts
cargo update
cargo build --target wasm32-unknown-unknown --release
cp target/wasm32-unknown-unknown/release/*.wasm ../testdata/
```
