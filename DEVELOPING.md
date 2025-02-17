# Developing

Welcome to the Stellar Go monorepo. These instructions help launch 🚀 you into making and testing code changes to this repository.

For details about what's in this repository and how it is organized read the [README.md](README.md).

If you're aiming to submit a contribution make sure to also read the [contributing guidelines](CONTRIBUTING.md).

If you're making changes to Horizon, look for documentation in its [docs](services/horizon/internal/docs) directory for specific instructions.

## Requirements
To checkout, build, and run most tests these tools are required:
- Git
- [Go](https://golang.org/dl) (this repository is officially supported on the last two releases of Go)

To run some tests these tools are also required:
- PostgreSQL 12+ server running locally, or set [environment variables](https://www.postgresql.org/docs/12/libpq-envars.html) (e.g. `PGHOST`, etc) for alternative host.
- MySQL 10.1+ server running locally.

## Get the code

Check the code out anywhere, using a `GOPATH` is not required.

```
git clone https://github.com/stellar/go
```

## Installing dependencies

Dependencies are managed using [Modules](https://github.com/golang/go/wiki/Modules). Dependencies for the packages you are building will be installed automatically when running any Go command that requires them. If you need to pre-download all dependencies for the repository for offline development, run `go mod download`.

See [Dependency management](#dependency-management) for more details.

## Running tests

```
go test ./...
```

## Running services/tools

```
go run ./services/<service>
```

```
go run ./tools/<tool>
```

## Dependency management

Dependencies are managed using [Modules](https://github.com/golang/go/wiki/Modules) and are tracked in the repository across two files:
- [go.mod](go.mod): Contains a list of direct dependencies, and some indirect dependencies (see [why](https://github.com/golang/go/wiki/Modules#why-does-go-mod-tidy-record-indirect-and-test-dependencies-in-my-gomod)).
- [go.sum](go.sum): Contains hashes for dependencies that are used for verifying downloaded dependencies.

### Adding/Removing dependencies

Add new dependencies by adding the import paths to the code. The next time you execute a Go command the tool will update the `go.mod` and `go.sum` files.

To add a specific version of a dependency use `go get`:

```
go get <importpath>@<version>
```

Before opening a PR make sure to run following command to tidy the module file. It will keep the go.* files tidy:
- `go mod tidy`

### Updating a dependency

Update an existing dependency by using `go get`:

```
go get <importpath>@<version>
```
Before opening a PR make sure to run these commands to tidy the module files:
 ```
 go mod tidy
 ```

Note: `go list -m all` may show that the dependency is still being used. It will be possible that the dependency is still an indirect dependency. If it's important to understand why the dependency is still being used, use `go mod why <importpath>/...` and `go mod graph | grep <importpath>` to understand which modules are importing it.

### Reviewing changes in dependencies

When updating or adding dependencies it's critical that we review what the
changes are in those dependencies that we are introducing into our builds. When
dependencies change the diff for the `go.mod` file may be complex to
understand. In that situation check each new or upgraded dependency,
and check each dependencies code diffs to see what is being imported.
Always treat code being imported as code written that needs review.

```
git checkout master
go list -m -json all > go.list.master
git checkout <branch>
golistcmp go.list.master <(go list -m -json all)
```

[golistcmp]: https://github.com/stellar/golistcmp
