# Changelog

All notable changes to this project will be documented in this file. This project adheres to [Semantic Versioning](http://semver.org/).

## Pending

### Bug Fixes
* Update the boundary check in `BufferedStorageBackend` to queue ledgers up to the end boundary, resolving skipped final batch when the `from` ledger doesn't align with file boundary [5563](https://github.com/stellar/go/pull/5563).

### New Features
* Create new package `ingest/cdp` for new components which will assist towards writing data transformation pipelines as part of [Composable Data Platform](https://stellar.org/blog/developers/composable-data-platform). 
* Add new functional producer, `cdp.ApplyLedgerMetadata`. A new function which enables a private instance of `BufferedStorageBackend` to perfrom the role of a producer operator in streaming pipeline designs.  It will emit pre-computed `LedgerCloseMeta` from a chosen `DataStore`. The stream can use `ApplyLedgerMetadata` as the origin of `LedgerCloseMeta`, providing a callback function which acts as the next operator in the stream, receiving the `LedgerCloseMeta`. [5462](https://github.com/stellar/go/pull/5462).

### Stellar Core Protocol 21 Configuration Update:
* BucketlistDB is now the default database for stellar-core, replacing the experimental option. As a result, the `EXPERIMENTAL_BUCKETLIST_DB` configuration parameter has been deprecated.
* A new mandatory parameter, `DEPRECATED_SQL_LEDGER_STATE`, has been added with a default value of false which equivalent to `EXPERIMENTAL_BUCKETLIST_DB` being set to true.
* The parameter `EXPERIMENTAL_BUCKETLIST_DB_INDEX_PAGE_SIZE_EXPONENT` has been renamed to `BUCKETLIST_DB_INDEX_PAGE_SIZE_EXPONENT`.
* The parameter `EXPERIMENTAL_BUCKETLIST_DB_INDEX_CUTOFF` has been renamed to `BUCKETLIST_DB_INDEX_CUTOFF`.

### New Features
* Support for Soroban and Protocol 20!
* The `LedgerTransactionReader` now has a `Seek(index int)` method to provide reading from arbitrary parts of the ledger [5274](https://github.com/stellar/go/pull/5274).
* `Change` now has a canonical stringification and a set of them is deterministically sortable.
* `NewCompactingChangeReader` will give you a wrapped `ChangeReader` that compacts the changes.
* Let filewatcher use binary hash instead of timestamp to detect core version update [4050](https://github.com/stellar/go/pull/4050).

### Performance Improvements
* The Captive Core backend now reuses bucket files whenever it finds existing ones in the corresponding `--captive-core-storage-path` (introduced in [v2.0](#v2.0.0)) rather than generating a one-time temporary sub-directory ([#3670](https://github.com/stellar/go/pull/3670)). Note that taking advantage of this feature requires [Stellar-Core v17.1.0](https://github.com/stellar/stellar-core/releases/tag/v17.1.0) or later.
* There have been miscallaneous memory and processing speed improvements.

### Bug Fixes
* The Stellar Core runner now parses logs from its underlying subprocess better [#3746](https://github.com/stellar/go/pull/3746).
* Ensures that the underlying Stellar Core is terminated before restarting.
* Backends will now connect with a user agent.
* Better handling of various error and restart scenarios.

### Breaking Changes
* **Captive Core is now the only available backend.**
* The Captive Core configuration should be provided via a TOML file.
* `Change.AccountSignersChanged` has been removed.

## v2.0.0

This release is related to the release of [Horizon v2.3.0](https://github.com/stellar/go/releases/tag/horizon-v2.3.0) and introduces some breaking changes to the `ingest` package for those building their own tools.

### Breaking Changes
- Many APIs now require a `context.Context` parameter, allowing you to interact with the backends and control calls in a more finely-controlled manner. This includes the readers (`ChangeReader` et al.) as well as the backends themselves (`CaptiveStellarCore` et al.).

- **`GetLedger()` always blocks** now, even for an `UnboundedRange`.

- The `CaptiveCoreBackend` now requires an all-inclusive `CaptiveCoreToml` object to configure Captive Core rather than an assortment of individual parameters. This object can be built from a TOML file (see `NewCaptiveCoreTomlFromFile`) or from parameters (see `NewCaptiveCoreToml`) as was done before.

- `LedgerTransaction.Meta` has been renamed to `UnsafeMeta` to highlight that users should be careful when interacting with it.

- Remote Captive Core no longer includes the `present` field in the ledger response JSON.

### New Features
- `NewLedgerChangeReaderFromLedgerCloseMeta` and `NewLedgerTransactionReaderFromLedgerCloseMeta` are new ways to construct readers from a particular single ledger.

### Other Changes
- The remote Captive Core client timeout has doubled.

- Captive Core now creates a temporary directory (`captive-core-...`) in the specified storage path (current directory by default) that it cleans it up on shutdown rather than in the OS's temp directory.
