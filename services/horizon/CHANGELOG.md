# Changelog

All notable changes to this project will be documented in this
file. This project adheres to [Semantic Versioning](http://semver.org/).

## Pending

- Update default pubnet captive core configuration to replace Whalestack with Creit Technologies in the quorum set ([5564](https://github.com/stellar/go/pull/5564)).

### Fixed
-  Fix the account operations endpoint to include InvokeHostFunction operations. The fix ensures that all account operations will be listed going forward. However, it will not retroactively include these operations for previously ingested ledgers; reingesting the historical data is required to address that. ([5574](https://github.com/stellar/go/pull/5574)).

## 22.0.2

### Fixed
- Fix liquidity pool bug which resulted in invalid paths being included in the `/paths/strict-receive` response ([5541](https://github.com/stellar/go/pull/5541)).

## 22.0.1

### Fixed
- Fix the issue where the skip-txmeta flag is not being applied in `/transactions/{tx-id}` endpoint ([5523](https://github.com/stellar/go/pull/5523)).
- Fix `horizon_ingest_reap_lookup_tables_duration_seconds` metric which reports the total number of lookup table rows which were reaped ([5528](https://github.com/stellar/go/pull/5528)).

### Removed
- Removed defunct `init-genesis-state` command ([5504](https://github.com/stellar/go/pull/5504)).

## 22.0.0-rc2


### Deprecations

- The `errorResultXdr` field from the response of the async transaction submission endpoint has been temporarily reinstated. However it will be removed in the v23 release. ([5496](https://github.com/stellar/go/pull/5496))
- The `num_archived_contracts` and `archived_contracts_amount` fields from the `/assets` response have been deprecated and will be removed in the v23 Horizon release. ([5496](https://github.com/stellar/go/pull/5496))

## 22.0.0-rc1

**This release adds support for Protocol 22**

### Breaking Changes

- `--parallel-job-size` configuration parameter for the `stellar-horizon db reingest` command has been removed.
  Job size will be automatically determined based on the number of workers (configuration parameter --parallel-workers), distributing
  the range equally among them. The minimum job size will remain 64 ledgers and the start and end ledger range will be rounded to
  the nearest checkpoint. ([5484](https://github.com/stellar/go/pull/5484))

- Removed `num_accounts` and `amount` fields from Asset stats related endpoints, and `valid_before` and `valid_after` fields from transaction related endpoints.  Issue - [5438](https://github.com/stellar/go/issues/5438), PR - [5478](https://github.com/stellar/go/pull/5478)
  - These fields have already been functionally deprecated as of release v2.1.0. As a part of this release, these fields are omitted from API Response
  - Additionally, the `num_accounts` and `amount` columns have been dropped from `exp_asset_stats` table in Postgres.

- Renamed `errorResultXdr` field to `error_result_xdr` in the response of the async transaction submission endpoint, `POST /transactions_async`. ([5445](https://github.com/stellar/go/pull/5445))

### Added

- Added response for new constructor functionality (introduced in protocol 22) in invoke host function operation ([5488](https://github.com/stellar/go/pull/5488)).

- Add metrics which give insight into how long it takes for ingestion to startup ([5449](https://github.com/stellar/go/pull/5449)).

### Fixed

- Fixed the `error` field nested in the `extras` field in the response of the async transaction submission endpoint `POST /transactions_async`. ([5440](https://github.com/stellar/go/issues/5440))

- Improved performance of requests which query the lower boundary of horizon's history when running a horizon instance with the `--history-retention-count` flag. ([5410](https://github.com/stellar/go/pull/5410), [5448](https://github.com/stellar/go/pull/5448), [5465](https://github.com/stellar/go/pull/5465))

- Improve performance of ingestion when running horizon with the `--history-retention-count` flag by executing reaping of history lookup tables concurrently with ingestion. ([5405](https://github.com/stellar/go/pull/5405))

- Improve performance of ingestion when consuming ledgers via the BufferedStorageBackend. ([5494](https://github.com/stellar/go/pull/5494))

## 2.32.0

### Added

- Reingest from pre-computed tx meta on remote cloud storage. ([4911](https://github.com/stellar/go/issues/4911)), ([5374](https://github.com/stellar/go/pull/5374))
  - Configure horizon reingestion to obtain ledger tx meta in pre-computed files from a Google Cloud Storage(GCS) location. 
  - Using this option will no longer require a captive core binary be present and it no longer runs a captive core sub-process, instead obtaining the tx meta from the GCS backend.
  - Horizon supports this new feature with two new parameters `ledgerbackend` and `datastore-config` on the `reingest` command. Refer to [Reingestion README](./internal/ingest/README.md#reingestion).
- Add metrics for reaping of history lookup tables ([5385](https://github.com/stellar/go/pull/5385)).
- Add `--reap-lookup-tables` (defaults to true) which will disable reaping of history lookup tables when set to false. ([5366](https://github.com/stellar/go/pull/5366)).


### Fixed
- Fix ingestion duration metric so it includes time spent reaping history lookup tables ([5361](https://github.com/stellar/go/pull/5361)).
- Optimize query for reaping history lookup tables ([5393](https://github.com/stellar/go/pull/5393)).

## 2.31.0

### Breaking Changes

- Change ingestion filtering logic to store transactions if any filter matches on it. ([5303](https://github.com/stellar/go/pull/5303))
  - The previous behaviour was to store a tx only if both asset and account filters match together. So even if a tx matched an account filter but failed to match an asset filter, it would not be stored by Horizon.
- Captive-core configuration parameters updated to align with [stellar-core v21](https://github.com/stellar/stellar-core/issues/3811) ([5333](https://github.com/stellar/go/pull/5333))
  - BucketlistDB is now the default database for stellar-core, deprecating the usage of `EXPERIMENTAL_BUCKETLIST_DB` in captive core configuration toml.
  - A new mandatory parameter `DEPRECATED_SQL_LEDGER_STATE` (default: false) is required by stellar-core on its captive core configuration toml file. if the toml provided by `CAPTIVE_CORE_CONFIG_PATH` does not have this new setting, Horizon will add it automatically, therefore, no action required.
  - If using `EXPERIMENTAL_BUCKETLIST_DB_INDEX_PAGE_SIZE_EXPONENT` or `EXPERIMENTAL_BUCKETLIST_DB_INDEX_CUTOFF` in captive core configuration toml, they must be renamed to `BUCKETLIST_DB_INDEX_PAGE_SIZE_EXPONENT` and `BUCKETLIST_DB_INDEX_CUTOFF` respectively.  

### Added

- Bump XDR definitions ([5289](https://github.com/stellar/go/pull/5289)), ([5330](https://github.com/stellar/go/pull/5330))
- Add new async transaction submission endpoint ([5188](https://github.com/stellar/go/pull/5188))
- Add `horizon_ingest_errors_total` metric key ([5302](https://github.com/stellar/go/pull/5302))
- Add transaction hash to txsub timeout response ([5328](https://github.com/stellar/go/pull/5328))
- Add new captive-core flags for V1 Meta ([5309](https://github.com/stellar/go/pull/5309))
- Add version check for protocol 21 ([5346](https://github.com/stellar/go/pull/5346))
- Improve horizon history reaper ([5331](https://github.com/stellar/go/pull/5331)). New reaper configuration flags `REAP_FREQUENCY` - the frequency in units of ledgers for how often history is reaped.

### Fixed

- Fix the following ingestion error: `error preparing range: error starting prepare range: the previous Stellar-Core instance is still running` ([5307](https://github.com/stellar/go/pull/5307))


## 2.30.0
**This release adds support for Protocol 21**

### Added

- Bump XDR for [protocol 21](https://github.com/stellar/stellar-xdr/releases/tag/v21.0)
- Make reaping batch sizes configurable via `--history-retention-reap-count` ([5272](https://github.com/stellar/go/pull/5272))
- Log tx meta when ingestion failures occur ([5268](https://github.com/stellar/go/pull/5268))
- Add deprecation warning for `--captive-core-use-db` ([5231](https://github.com/stellar/go/pull/5231))

### Fixed
-  Optimized reingestion by addressing performance slowdown due to unnecessary operations on `history_transactions_filtered_tmp`. Removed obsolete `EnableIngestionFiltering` flag. ([5283](https://github.com/stellar/go/pull/5283))
- Fix deadlock in parallel ingestion ([5263](https://github.com/stellar/go/pull/5263))
- Add missing tables to TruncateIngestStateTables() ([5253](https://github.com/stellar/go/pull/5253))
- Performance improvements in ingest library
    - Reduce memory consumption of CheckpointChangeReader during state verification and state rebuild ([5270](https://github.com/stellar/go/pull/5270))
    - Remove unnecessary use of ChangeCompactor to reduce memory bloat during ingestion ([5252](https://github.com/stellar/go/pull/5252))

## 2.29.0

### Added
- New `db_error_total` metrics key with labels `ctx_error`, `db_error`, and `db_error_extra` ([5225](https://github.com/stellar/go/pull/5225)).
- Bumped go version to the latest (1.22.1) ([5232](https://github.com/stellar/go/pull/5232))
- Add metrics for ingestion loaders ([5209](https://github.com/stellar/go/pull/5209)).
- Add metrics for http api requests in flight and requests received ([5240](https://github.com/stellar/go/pull/5240)).
- Add `MAX_CONCURRENT_REQUESTS`, defaults to 1000, limits the number of horizon api requests in flight ([5244](https://github.com/stellar/go/pull/5244))

### Fixed
- History archive access is more effective when you pass list of URLs to Horizon: they will now be accessed in a round-robin fashion, use alternative archives on errors, and intelligently back off ([5224](https://github.com/stellar/go/pull/5224))
- Remove captive core info request error logs ([5145](https://github.com/stellar/go/pull/5145))
- Removed duplicate "Processed Ledger" log statement during resume state ([5152](https://github.com/stellar/go/pull/5152))
- Fixed incorrect duration for ingestion processor metric ([5216](https://github.com/stellar/go/pull/5216))
- Fixed sql performance on account transactions query ([5229](https://github.com/stellar/go/pull/5229))
- Fix bug in claimable balance change processor ([5246](https://github.com/stellar/go/pull/5246))
- Delay canceling queries from client side when there's a statement / transaction timeout configured in postgres ([5223](https://github.com/stellar/go/pull/5223))

### Breaking Changes
- The Horizon API Transaction resource field in json `result_meta_xdr` is now optional and Horizon API will not emit the field when Horizon has been configured with `SKIP_TXMETA=true`, effectively null, otherwise if Horizon is configured with `SKIP_TXMETA=false` which is default, then the API Transaction field `result_meta_xdr` will remain present and populated with base64 encoded xdr [5228](https://github.com/stellar/go/pull/5228).

## 2.28.3

### Fixed
- Fix claimable_balance_claimants subquery in GetClaimableBalances() ([5207](https://github.com/stellar/go/pull/5207))

### Added
- New optional config `SKIP_TXMETA` ([5189](https://github.com/stellar/go/issues/5189)). Defaults to `FALSE`, when `TRUE` the following will occur:
  * history_transactions.tx_meta column will have serialized xdr that equates to empty for any protocol version, such as for `xdr.TransactionMeta.V3`, `Operations`, `TxChangesAfter`, `TxChangesBefore` will be empty arrays and `SorobanMeta` will be nil.

### Breaking Changes
- Removed `DISABLE_SOROBAN_INGEST` configuration parameter, use the new `SKIP_TXMETA` parameter instead.

## 2.28.2

### Fixed
- History archive caching would cause file corruption in certain environments ([5197](https://github.com/stellar/go/pull/5197))
- Server error in claimable balance API when claimant, asset and cursor query params are supplied ([5200](https://github.com/stellar/go/pull/5200))

## 2.28.1

### Fixed
- Submitting transaction with a future gapped sequence number greater than 1 past current source account sequence, may result in delayed 60s timeout response, rather than expected HTTP 400 error response with `result_codes: {transaction: "tx_bad_seq"}` ([5191](https://github.com/stellar/go/pull/5191))

## 2.28.0

### Fixed
- Ingestion performance improvements ([4909](https://github.com/stellar/go/issues/4909))
- Trade aggregation rebuild errors reported on `db reingest range` with parallel workers ([5168](https://github.com/stellar/go/pull/5168))
- Limited global flags displayed on cli help output ([5077](https://github.com/stellar/go/pull/5077))
- Network usage has been significantly reduced with caching. **Warning:** To support the cache, disk requirements may increase by up to 15GB ([5171](https://github.com/stellar/go/pull/5171)).

### Added
- We now include metrics for history archive requests ([5166](https://github.com/stellar/go/pull/5166))
- Http history archive requests now include a unique user agent ([5166](https://github.com/stellar/go/pull/5166))
- Added a deprecation warning for using command-line flags when running Horizon ([5051](https://github.com/stellar/go/pull/5051))
- New optional config `DISABLE_SOROBAN_INGEST` ([5175](https://github.com/stellar/go/issues/5175)). Defaults to `FALSE`, when `TRUE` and a soroban transaction is ingested, the following will occur:
  * no effects will be generated for contract invocations.
  * history_transactions.tx_meta column will have serialized xdr that equates to an empty `xdr.TransactionMeta.V3`, `Operations`, `TxChangesAfter`, `TxChangesBefore` will empty arrays and `SorobanMeta` will be nil.
  * API transaction model for `result_meta_xdr` will have same empty serialized xdr for `xdr.TransactionMeta.V3`, `Operations`, `TxChangesAfter`, `TxChangesBefore` will empty arrays and `SorobanMeta` will be nil.
  * API `Operation` model for `InvokeHostFunctionOp` type, will have empty `asset_balance_changes`

### Breaking Changes
- Deprecation of legacy, non-captive core ingestion([5158](https://github.com/stellar/go/pull/5158)):
  * removed configuration flags `--stellar-core-url-db`, `--cursor-name` `--skip-cursor-update`, they are no longer usable.
  * removed automatic updating of core cursor from ingestion background processing.<br/>
    **Note** for upgrading on existing horizon deployments - Since horizon will no longer maintain advancement of this cursor on core, it may require manual removal of the cursor from the core process that your horizon was using for captive core, otherwise that core process may un-necessarily retain older data in buckets on disk up to the last cursor ledger sequence set by prior horizon release.

    The captive core process to check and verify presence of cursor usage is determined by the horizon deployment, if `NETWORK` is present, or `STELLAR_CORE_URL` is present or  `CAPTIVE-CORE-HTTP-PORT` is present and set to non-zero value, or `CAPTIVE-CORE_CONFIG_PATH` is used and the toml has `HTTP_PORT` set to non-zero and `PUBLIC_HTTP_PORT` is not set to false, then it is recommended to perform the following preventative measure on the machine hosting horizon after upgraded to 2.28.0 and process restarted:
    ```
    $ curl http://<captive_core_process_url:captive_core_process_port>/getcursor
    # If there are no cursors reported, done, no need for any action
    # If any horizon cursors exist they need to be dropped by id.
    # By default horizon sets cursor id to "HORIZON" but if it was customized
    # using the --cursor-name flag the id might be different
    $ curl http://<captive_core_process_url:captive_core_process_port>/dropcursor?id=<reported_id_from_getcursor>
    ```


## 2.27.0

### Fixed
- Ordering of effects are now deterministic. Previously the order of some Horizon effects could vary upon reingestion but this issue has now been fixed ([5070](https://github.com/stellar/go/pull/5070)).

## 2.27.0-rc2
### Fixed
- treat null is_payment values as equivalent to false values, avoid sql nil conversion errors([5060](https://github.com/stellar/go/pull/5060)).

## 2.27.0-rc1

**Upgrading to this version from <= 2.26.1 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

**This release adds support for Protocol 20**

### Breaking Changes
- The command line flag `--remote-captive-core-url` has been removed, as remote captive core functionality is now deprecated ([4940](https://github.com/stellar/go/pull/4940)).
- The functionality of generating default captive core configuration based on the --network-passphrase is now deprecated. Use the --network command instead ([4949](https://github.com/stellar/go/pull/4949)).

### Added
- Added new command-line flag `--network` to specify the Stellar network (pubnet or testnet), aiming at simplifying the configuration process by automatically configuring the following parameters based on the chosen network: `--history-archive-urls`, `--network-passphrase`, and `--captive-core-config-path` ([4949](https://github.com/stellar/go/pull/4949)).
- Added `contract_credited` and `contract_debited` effects which are emitted whenever a Soroban contracts sends or receives a Stellar asset ([4832](https://github.com/stellar/go/pull/4832)).
* Added `num_contracts` (total number of Soroban contracts which hold an asset) and `contracts_amount` (total amount of the asset held by all Soroban contracts) fields to asset stat summaries at `/assets` ([4805](https://github.com/stellar/go/pull/4805)).
* Added responses for new operations introduced in protocol 20: `invoke_host_function`, `bump_footprint_expiration`, and `restore_footprint` ([4905](https://github.com/stellar/go/pull/4905)).

### Fixed
- The same slippage calculation from the [`v2.26.1`](#2261) hotfix now properly excludes spikes for smoother trade aggregation plots ([4999](https://github.com/stellar/go/pull/4999)).
- Limit the display of global flags on command line help `-h` output ([5077](https://github.com/stellar/go/pull/5077)).

### DB Schema Migration
- Drop unused indices from the Horizon database. For the database with full history, the migration is anticipated to take up to an hour and is expected to free up approximately 1.3TB of storage ([5081](https://github.com/stellar/go/pull/5081)).

## 2.26.1

### Fixes
- Ingestion will not halt if liquidity pool overflows are detected.


## 2.26.0
### Changes
- Improve error handling for when stellar-core crashes ([4893](https://github.com/stellar/go/pull/4893))
- Suppress Core timeout error in log output such as `error ticking app: context deadline exceeded` when ingestion state machine is in build state. ([4860](https://github.com/stellar/go/pull/4860))


### Breaking Changes
- Modify the default value of `--captive-core-use-db` to true ([4856](https://github.com/stellar/go/issues/4856))
  - This updates the default behavior of captive core to start in on-disk mode.
  - To continue using the previous in-memory mode, explicitly set the `--captive-core-use-db` flag to false

## 2.25.0

### Changes

- Running Horizon with remote captive core is now deprecated ([4826](https://github.com/stellar/go/pull/4826)).
- Add two new configuration variables to control the behavior of state verification ([4821](https://github.com/stellar/go/pull/4821)):
  - `--ingest-state-verification-frequency` which specifies the frequency in checkpoints for how often state verification is run
  - `--ingest-state-verification-timeout` which specifies a timeout on how long state verification can run

### Fixes

* Fix crash in horizon ingestion when running horizon with a remote captive core ([4824](https://github.com/stellar/go/pull/4824)).


## 2.24.1

### Changes

- Bump Go to the latest version (1.20.1), including crypto/tls, mime/multipart, net/http, and path/filepath security fixes.

## 2.24.0

### Changes

- Add support for the experimental _BucketListDB_ to Horizon, the parameters have been added to the Captive Core configuration / TOML files ([4733](https://github.com/stellar/go/pull/4733)):

  * _BucketListDB_ is enabled by default when `--captive-core-use-db` is set and your `stellar-core` version >= 19.6
  * If `--captive-core-use-db` set but your `stellar-core` version < 19.6, an on-disk SQLite database is used (as before).
  * This update will not automatically trigger a state rebuild **unless** `EXPERIMENTAL_BUCKETLIST_DB` is set to false in the Captive Core TOML file.

### Fixes

* Improve error when setting `BUCKET_DIR_PATH` and using Captive Core ([4736](https://github.com/stellar/go/pull/4736)).

## 2.23.1

### Changes

- Bump Go to the latest version, including net/http security fixes.

## 2.23.0

**Upgrading to this version will trigger a state rebuild. During this process, Horizon will not ingest new ledgers.**

### Fixes

* Improve performance of `/claimable_balances` filters. This change should significantly improve `?asset=` and `?claimant=` filters. ([#4690](https://github.com/stellar/go/pull/4690)).
* Reallocate slices after offer removals in order book graph. This is done to prevent keeping a large chunks of allocated but unused memory that can lead to OOM crash.
* The ingestion subsystem will now properly use a pool of history archives if more than one is provided. ([#4687](https://github.com/stellar/go/pull/4687))
* Add `horizon ingest build-state` command which builds state at a specific ledger. Useful for debugging. ([#4636](https://github.com/stellar/go/pull/4636))

## 2.22.1

**Upgrading to this version from <= v2.8.3 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

### Fixes

- `horizon db migrate` commands will not apply migrations if ingestion is disabled ([4664](https://github.com/stellar/go/pull/4664)).

## 2.22.0

**Upgrading to this version from <= v2.8.3 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

### Fixes

- Database migrations will now acquire a lock on ingestion, preventing possible deadlocks ([4587](https://github.com/stellar/go/pull/4587)).

### Changes

- Optimizes startup by preserving the Captive Core storage directory if Horizon has ingested ahead of the requested ledger ([4605](https://github.com/stellar/go/pull/4605)).

### DB Schema Migration

- Introduces a new index that optimizes account queries filtered by asset ([4635](https://github.com/stellar/go/pull/4635)).

## 2.21.0

**Upgrading to this version from <= v2.8.3 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

### Changes

- Bump Go to the latest version, including net/http and net/url security fixes ([4577](https://github.com/stellar/go/pull/4577))

### DB Schema Migration

- Added indexes by id for assets in the respective `history_trades` tables. Ingestion will stop while the migration is being applied. ([4565](https://github.com/stellar/go/pull/4565))

## 2.20.0

**Upgrading to this version from <= v2.8.3 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

### DB Schema Migration

- Added indexes by id for claimable balance and liquidity pool id's in the respective tx/ops tables. Ingestion will stop while the migration is being applied. ([4455](https://github.com/stellar/go/pull/4477))

### Changes

- Orphaned rows in lookup tables (`history_accounts`, `history_claimable_balances` and `history_liquidity_pools`) are removed in small batches after each ledger if `--history-retention-count` is set. ([4518](https://github.com/stellar/go/pull/4518), [4525](https://github.com/stellar/go/pull/4525))
- Improve restart time of Captive-Core when started with `--captive-core-use-db` flag. The solution does not work on Windows. ([4471](https://github.com/stellar/go/pull/4471))
- Fix a bug in which state verifier would run if condition checking DB query fails. ([4523](https://github.com/stellar/go/pull/4523))

## 2.19.0

**Upgrading to this version from <= v2.8.3 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

### Breaking Changes

* Update core version to 19.3.0 ([4485](https://github.com/stellar/go/pull/4485)).
* Pass `--console` to captive core. This is due to a breaking change in stellar-core 19.3.0 ([4487](https://github.com/stellar/go/pull/4487)).

### Changes

* Run postgres autovacuum on `history_trades_60000` table more frequently. ([4412](https://github.com/stellar/go/pull/4412)).
* Change `protocols/horizon.Transaction.AccountSequence` to `int64` from `string`. ([4409](https://github.com/stellar/go/pull/4409)).
* Add missing signer key type names. ([4429](https://github.com/stellar/go/pull/4429)).
* Update core version to 19.2.0. ([4441](https://github.com/stellar/go/pull/4441)).
* Add `User-Agent` header to history archive HTTP requests. ([4463](https://github.com/stellar/go/pull/4463)).

### DB Schema Migration

The migration makes the following schema changes:

  - tunes the `history_trades_60000` table to run autovacuum more frequently. Migration should be brief, does not incur any data/runtime processing. It does not need a reingestion.

## 2.18.1

* Enabled txsub system to work if/when underlying horizon db connection is read only. ([4418](https://github.com/stellar/go/pull/4418))
* Optimize the claimable balance SQL query for best index query plan performance([4398](https://github.com/stellar/go/pull/4398))


### DB Schema Migration

The migration makes the following schema changes:

  - creates new, `history_transactions_filtered_tmp` table related to enabling tx sub processing on read only, only ingest writes to the table. Deprecates, drops prior `txsub_results` table. Migration should be brief, does not incur any data/runtime processing.


## 2.18.0

* New Experimental Ingestion Filters Feature: Provide the ability to select which ledger transactions are accepted at ingestion time to be stored on horizon's historical databse.

  Define filter rules through Admin API and the historical ingestion process will check the rules and only persist the ledger transactions that pass the filter rules. Initially, two filters and corresponding rules are possible:

  * 'whitelist by account id' ([4221](https://github.com/stellar/go/issues/4221))
  * 'whitelist by canonical asset id' ([4222](https://github.com/stellar/go/issues/4222))

  The filters and their configuration are optional features and must be enabled with horizon command line parameters `admin-port=4200` and `exp-enable-ingestion-filtering=true`

  Once set, filter configurations and their rules are initially empty and the filters are disabled by default. To enable filters, update the configuration settings, refer to the Admin API Docs which are published on the Admin Port at http://localhost:<admin_port>/, follow details and examples for endpoints:
  * `/ingestion/filters/account`
  * `/ingestion/filters/asset.`
- Querying claimable balances has been optimized ([4385](https://github.com/stellar/go/pull/4385)).
- Querying trade aggregations has been optimized ([4389](https://github.com/stellar/go/pull/4389)).
- Postgres connections for non ingesting Horizon instances are now configured to timeout on long running queries / transactions ([4390](https://github.com/stellar/go/pull/4390)).
- Added `disable-path-finding` Horizon flag to disable the path finding endpoints. This flag should be enabled on ingesting Horizon instances which do not serve HTTP traffic ([4399](https://github.com/stellar/go/pull/4399)).


## V2.17.1

- Querying claimable balances has been optimized ([4385](https://github.com/stellar/go/pull/4385)).
- Querying trade aggregations has been optimized ([4389](https://github.com/stellar/go/pull/4389)).
- Postgres connections for non ingesting Horizon instances are now configured to timeout on long running queries / transactions ([4390](https://github.com/stellar/go/pull/4390)).
- Added `disable-path-finding` Horizon flag to disable the path finding endpoints. This flag should be enabled on ingesting Horizon instances which do not serve HTTP traffic ([4399](https://github.com/stellar/go/pull/4399)).


## V2.17.0

This is the final release after the [release candidate](v2.17.0-release-candidate), including some small additional changes:

- The transaction precondition record now excludes ([4360](https://github.com/stellar/go/pull/4360)):
  * `min_account_sequence_age` when it's `"0"`, as this is the default value when the condition is not set
  * `preconditions.ledgerbounds.max_ledger` when it's set to 0 (this means that there is no upper bound)

- Timebounds within the `preconditions` object are strings containing int64 UNIX timestamps in seconds rather than formatted date-times (which was a bug) ([4361](https://github.com/stellar/go/pull/4361)).

## V2.17.0 Release Candidate

**Upgrading to this version from <= v2.8.3 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

**Support for Protocol 19** ([4340](https://github.com/stellar/go/pull/4340)):

  - Account records can now contain two new, optional fields:

```txt
    "sequence_ledger": 0, // uint32 ledger number
    "sequence_time": "0"  // uint64 unix time in seconds, as a string
```

  The absence of these fields indicates that the account hasn't taken any actions since prior to the Protocol 19 release. Note that they'll either be both present or both absent.

  - Transaction records can now contain the following optional object:

```txt
    "preconditions": {
      "timebounds": {
        "min_time": "0",  // uint64 unix time in seconds, as a string
        "max_time": "0"   // as above
      },
      "ledgerbounds": {
        "min_ledger": 0,  // uint32 ledger number
        "max_ledger": 0   // as above
      },
      "min_account_sequence": "0",          // int64 sequence number, as a string
      "min_account_sequence_age": "0",      // uint64 unix time in seconds, as a string
      "min_account_sequence_ledger_gap": 0, // uint32 ledger count

      "extra_signers": [] // list of signers as StrKeys
    }
```

  All of the top-level fields within this object are also optional. However, the "ledgerbounds" object will always have at least its `min_ledger` field set.

  Note that the existing "valid_before_time" and "valid_after_time" fields on the top-level object will be identical to the "preconditions.timebounds.min_time" and "preconditions.timebounds.min_time" fields, respectively, if those exist. The "valid_before_time" and "valid_after_time" fields are now considered deprecated and will be removed in Horizon v3.0.0.

### DB Schema Migration

The migration makes the following schema changes:

  - adds new, optional columns to the `history_transactions` table related to the new preconditions
  - adds new, optional columns to the `accounts` table related to the new account extension
  - amends the `signer` column of the `accounts_signers` table to allow signers of arbitrary length

### Deprecations

The following fields on transaction records have been deprecated and will be removed in a future version:

  - `"valid_before"` and `"valid_after"`

These fields are now represented by `preconditions.timebounds.min_time` and `preconditions.timebounds.max_time` as `uint64` UNIX timestamps, in seconds.

## V2.16.1

* v2.16.0 rebuilt using Golang 1.18.1 with security fixes for CVE-2022-24675, CVE-2022-28327 and CVE-2022-27536.

## V2.16.0

* Replace keybase with publicnode in the stellar core config. ([4291](https://github.com/stellar/go/pull/4291))
* Add a rate limit for path finding requests. ([4310](https://github.com/stellar/go/pull/4310))
* Horizonclient, fix multi-parameter url for claimable balance query. ([4248](https://github.com/stellar/go/pull/4248))

## v2.15.1

**Upgrading to this version from <= v2.8.3 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

### Fixes

* Fixed a regression preventing running multiple concurrent captive-core ingestion instances. ([4251](https://github.com/stellar/go/pull/4251))

## v2.15.0

**Upgrading to this version from <= v2.8.3 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

### DB Schema Migration

* DB migrations add columns to the `history_trades` table to enable filtering trades by "rounding slippage". This is very large table so migration may take a long time (depending on your DB hardware). Please test the migrations execution time on the copy of your production DB first.

### Features

* New feature, enable captive core based ingestion to use remote db persistence rather than in-memory for ledger states. Essentially moves what would have been stored in RAM to the external db instead. Recent profiling on the two approaches shows an approximate space usage of about 8GB for ledger states as of 02/2022 timeframe, but it will gradually continue to increase as more accounts/assets are added to network. Current horizon ingest behavior when configured for captive core usage will by default take this space from RAM, unless a new command line flag is specified `--captive-core-use-db=true`, which enables this space to be taken from the external db instead, and not RAM. The external db used is determined be setting `DATABASE` parameter in the captive core cfg/.toml file. If no value is set, then by default it uses sqlite and the db file is stored in `--captive-core-storage-path` - ([4092](https://github.com/stellar/go/pull/4092))
  * Note, if using this feature, we recommend using a storage device with capacity for at least 3000 write ops/second.

### Fixes

* Exclude trades with high "rounding slippage" from `/trade_aggregations` endpoint. ([4178](https://github.com/stellar/go/pull/4178))
  * Note, to apply this change retroactively to existing data you will need to reingest starting from protocol 18 (ledger `38115806`).
* Release DB connection in `/paths` when no longer needed. ([4228](https://github.com/stellar/go/pull/4228))
* Fixed false positive warning during orderbook verification in the horizon log output whenever the in memory orderbook is inconsistent with the postgres liquidity pool and offers table. ([4236](https://github.com/stellar/go/pull/4236))

## v2.14.0

* Restart Stellar-Core when it's context is cancelled. ([4192](https://github.com/stellar/go/pull/4192))
* Resume ingestion immediately when catching up. ([4196](https://github.com/stellar/go/pull/4196))
* Check if there are newer ledger when requested ledger does not exist. ([4198](https://github.com/stellar/go/pull/4198))
* Properly check against the HA array being empty. ([4152](https://github.com/stellar/go/pull/4152))

- Querying claimable balances has been optimized ([4385](https://github.com/stellar/go/pull/4385)).
- Querying trade aggregations has been optimized ([4389](https://github.com/stellar/go/pull/4389)).
- Postgres connections for non ingesting Horizon instances are now configured to timeout on long running queries / transactions ([4390](https://github.com/stellar/go/pull/4390)).
- Added `disable-path-finding` Horizon flag to disable the path finding endpoints. This flag should be enabled on ingesting Horizon instances which do not serve HTTP traffic ([4399](https://github.com/stellar/go/pull/4399)).

## V2.17.0

This is the final release after the [release candidate](v2.17.0-release-candidate), including some small additional changes:

- The transaction precondition record now excludes ([4360](https://github.com/stellar/go/pull/4360)):
  * `min_account_sequence_age` when it's `"0"`, as this is the default value when the condition is not set
  * `preconditions.ledgerbounds.max_ledger` when it's set to 0 (this means that there is no upper bound)

- Timebounds within the `preconditions` object are strings containing int64 UNIX timestamps in seconds rather than formatted date-times (which was a bug) ([4361](https://github.com/stellar/go/pull/4361)).

* New Ingestion Filters Feature: Provide the ability to select which ledger transactions are accepted at ingestion time to be stored on horizon's historical databse.

  Define filter rules through Admin API and the historical ingestion process will check the rules and only persist the ledger transactions that pass the filter rules. Initially, two filters and corresponding rules are possible:

  * 'whitelist by account id' ([4221](https://github.com/stellar/go/issues/4221))
  * 'whitelist by canonical asset id' ([4222](https://github.com/stellar/go/issues/4222))

  The filters and their configuration are optional features and must be enabled with horizon command line parameters `admin-port=4200` and `enable-ingestion-filtering=true`

  Once set, filter configurations and their rules are initially empty and the filters are disabled by default. To enable filters, update the configuration settings, refer to the Admin API Docs which are published on the Admin Port at http://localhost:<admin_port>/, follow details and examples for endpoints:
  * `/ingestion/filters/account`
  * `/ingestion/filters/asset.`

## V2.17.0 Release Candidate

**Upgrading to this version from <= v2.8.3 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

**Support for Protocol 19** ([4340](https://github.com/stellar/go/pull/4340)):

  - Account records can now contain two new, optional fields:

```txt
    "sequence_ledger": 0, // uint32 ledger number
    "sequence_time": "0"  // uint64 unix time in seconds, as a string
```

  The absence of these fields indicates that the account hasn't taken any actions since prior to the Protocol 19 release. Note that they'll either be both present or both absent.

  - Transaction records can now contain the following optional object:

```txt
    "preconditions": {
      "timebounds": {
        "min_time": "0",  // uint64 unix time in seconds, as a string
        "max_time": "0"   // as above
      },
      "ledgerbounds": {
        "min_ledger": 0,  // uint32 ledger number
        "max_ledger": 0   // as above
      },
      "min_account_sequence": "0",          // int64 sequence number, as a string
      "min_account_sequence_age": "0",      // uint64 unix time in seconds, as a string
      "min_account_sequence_ledger_gap": 0, // uint32 ledger count

      "extra_signers": [] // list of signers as StrKeys
    }
```

  All of the top-level fields within this object are also optional. However, the "ledgerbounds" object will always have at least its `min_ledger` field set.

  Note that the existing "valid_before_time" and "valid_after_time" fields on the top-level object will be identical to the "preconditions.timebounds.min_time" and "preconditions.timebounds.min_time" fields, respectively, if those exist. The "valid_before_time" and "valid_after_time" fields are now considered deprecated and will be removed in Horizon v3.0.0.

### DB Schema Migration

The migration makes the following schema changes:

  - adds new, optional columns to the `history_transactions` table related to the new preconditions
  - adds new, optional columns to the `accounts` table related to the new account extension
  - amends the `signer` column of the `accounts_signers` table to allow signers of arbitrary length

### Deprecations

The following fields on transaction records have been deprecated and will be removed in a future version:

  - `"valid_before"` and `"valid_after"`

These fields are now represented by `preconditions.timebounds.min_time` and `preconditions.timebounds.max_time` as `uint64` UNIX timestamps, in seconds.

## V2.16.1

* v2.16.0 rebuilt using Golang 1.18.1 with security fixes for CVE-2022-24675, CVE-2022-28327 and CVE-2022-27536.

## V2.16.0

* Replace keybase with publicnode in the stellar core config. ([4291](https://github.com/stellar/go/pull/4291))
* Add a rate limit for path finding requests. ([4310](https://github.com/stellar/go/pull/4310))
* Horizonclient, fix multi-parameter url for claimable balance query. ([4248](https://github.com/stellar/go/pull/4248))

## v2.15.1

**Upgrading to this version from <= v2.8.3 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

### Fixes

* Fixed a regression preventing running multiple concurrent captive-core ingestion instances. ([4251](https://github.com/stellar/go/pull/4251))

## v2.15.0

**Upgrading to this version from <= v2.8.3 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

### DB Schema Migration

* DB migrations add columns to the `history_trades` table to enable filtering trades by "rounding slippage". This is very large table so migration may take a long time (depending on your DB hardware). Please test the migrations execution time on the copy of your production DB first.

### Features

* New feature, enable captive core based ingestion to use remote db persistence rather than in-memory for ledger states. Essentially moves what would have been stored in RAM to the external db instead. Recent profiling on the two approaches shows an approximate space usage of about 8GB for ledger states as of 02/2022 timeframe, but it will gradually continue to increase as more accounts/assets are added to network. Current horizon ingest behavior when configured for captive core usage will by default take this space from RAM, unless a new command line flag is specified `--captive-core-use-db=true`, which enables this space to be taken from the external db instead, and not RAM. The external db used is determined be setting `DATABASE` parameter in the captive core cfg/.toml file. If no value is set, then by default it uses sqlite and the db file is stored in `--captive-core-storage-path` - ([4092](https://github.com/stellar/go/pull/4092))
  * Note, if using this feature, we recommend using a storage device with capacity for at least 3000 write ops/second.

### Fixes

* Exclude trades with high "rounding slippage" from `/trade_aggregations` endpoint. ([4178](https://github.com/stellar/go/pull/4178))
  * Note, to apply this change retroactively to existing data you will need to reingest starting from protocol 18 (ledger `38115806`).
* Release DB connection in `/paths` when no longer needed. ([4228](https://github.com/stellar/go/pull/4228))
* Fixed false positive warning during orderbook verification in the horizon log output whenever the in memory orderbook is inconsistent with the postgres liquidity pool and offers table. ([4236](https://github.com/stellar/go/pull/4236))

## v2.14.0

* Restart Stellar-Core when it's context is cancelled. ([4192](https://github.com/stellar/go/pull/4192))
* Resume ingestion immediately when catching up. ([4196](https://github.com/stellar/go/pull/4196))
* Check if there are newer ledger when requested ledger does not exist. ([4198](https://github.com/stellar/go/pull/4198))
* Properly check against the HA array being empty. ([4152](https://github.com/stellar/go/pull/4152))

## v2.13.0

### DB Schema Migration

* DB migrations add a column and index to the `history_trades` table to improve performance of some queries. This is very large table so migration may take a long time (depending on your DB hardware). Please test the migrations execution time on the copy of your production DB first.

### Changes

* Improve performance of `/trades?trade_type=liquidity_pool` requests. ([4149](https://github.com/stellar/go/pull/4149))
* Added `absBeforeEpoch` to ClaimableBalance API Resources. It will contain the Unix epoch representation of absolute before date. ([4148](https://github.com/stellar/go/pull/4148))
* Path finding results contain empty paths again (removed in Horizon 2.9.0). ([4137](https://github.com/stellar/go/pull/4137))
* Generate HTTP Status code of 499 for Client Disconnects, should propagate into `horizon_http_requests_duration_seconds_count` metric key with `status="499"` label. ([4098](https://github.com/stellar/go/pull/4098))
* Fix incorrect counting of rate limited events in stream requests. ([4163](https://github.com/stellar/go/pull/4163))
* Update cursor on every ledger when using old non Captive-Core ingestion backend. ([4150](https://github.com/stellar/go/pull/4150))
* Fix the code responsible for updating Stellar-Core status that could stop the metrics updates on connectivity issues. ([4180](https://github.com/stellar/go/pull/4180))

## v2.12.1

### Fixes
* Fixes a critical vulnerability in HTTP server of Golang <=1.17.4. An attacker can cause unbounded memory growth in a Go server accepting HTTP/2 requests.

## v2.12.0

### Features
* Result codes for fee-bump transactions will now also include the inner result codes ([4081](https://github.com/stellar/go/pull/4081))

### Performance improvements
* XDR encoding/decoding pipelines have been optimized ([4069](https://github.com/stellar/go/pull/4069), [4068](https://github.com/stellar/go/pull/4068), [4073](https://github.com/stellar/go/pull/4073), [4064](https://github.com/stellar/go/pull/4064), [4071](https://github.com/stellar/go/pull/4071), [4075](https://github.com/stellar/go/pull/4075), [4077](https://github.com/stellar/go/pull/4077))

* Path-finding on the `/paths` endpoint has been sped up significantly ([4091](https://github.com/stellar/go/pull/4091), [4096](https://github.com/stellar/go/pull/4096), [4102](https://github.com/stellar/go/pull/4102)), [4105](https://github.com/stellar/go/pull/4105), [4113](https://github.com/stellar/go/pull/4113)

* Unused database indices have been removed ([4085](https://github.com/stellar/go/pull/4085), [4089](https://github.com/stellar/go/pull/4089))

### Fixes
* Improves error parsing from Captive Core ([4066](https://github.com/stellar/go/pull/4066))

* Prevent duplicate errors related to liquidity pool tables during repeated reingestion of same range ([4114](https://github.com/stellar/go/pull/4114))

* In the 2.11.0 release there was a bug introduced which made the `horizon db reingest range` command ignore optional parameters like `--parallel-workers`. This bug is now fixed so all optional command line flags are parsed correctly ([4127](https://github.com/stellar/go/pull/4127))

## v2.11.0

### Changes

* Add a new horizon flag `--max-assets-per-path-request` (`15` by default) that sets the number of assets to consider for strict-send and strict-recieve requests ([4046](https://github.com/stellar/go/pull/4046))
* Add an endpoint `/liquidity_pools?account={account_id}` which returns the liquidity pools an account is participating in [4043](https://github.com/stellar/go/pull/4043)
* Add a new horizon command `horizon db fill-gaps` which fills any gaps in history in the horizon db. The command takes optional start and end ledger parameters. If the start and end ledger is provided then horizon will only fill the gaps found within the given ledger range [4060](https://github.com/stellar/go/pull/4060)
* Improve performance of `/liquidity_pools/{liquidity_pool_id}/effects` endpoint by optimizing the db query to fetch effects for a liquidity pool [4065](https://github.com/stellar/go/pull/4065)
* Include the captive core binary in the `stellar/horizon` Docker image [4019](https://github.com/stellar/go/pull/4019)
* Remove `--captive-core-reuse-storage-dir` horizon flag [4048](https://github.com/stellar/go/pull/4048)
* Improve performance of XDR encoding which should also improve ingestion speeds [4063](https://github.com/stellar/go/pull/4063), [4056](https://github.com/stellar/go/pull/4056), [3957](https://github.com/stellar/go/pull/3957)
* Improve detection of when the Stellar Core binary has been modified [4050](https://github.com/stellar/go/pull/4050)
* `horizon_ingest_state_verify_ledger_entries` metric was changed to gauge [4054](https://github.com/stellar/go/pull/4054)

## v2.10.0

This is a minor release with no DB Schema migrations nor explicit state rebuild.

### Changes

* Use the correct asset when calculating liquidity pool disbursements ([4018](https://github.com/stellar/go/pull/4018))
* Make sure Stellar-Core is not started before previous instance termination ([4020](https://github.com/stellar/go/pull/4020))
* Add a new feature flag `--ingest-enable-extended-log-ledger-stats` (`false` by default) that enables extra ledger stats when logging ledger processing info ([4017](https://github.com/stellar/go/pull/4017))
* Add a new command `horizon record-metrics` that records `:[ADMIN_PORT]/metrics` into a zip file for debugging purposes ([4023](https://github.com/stellar/go/pull/4023))
* Expose the `Latest-Ledger` header to browser web pages ([3995](https://github.com/stellar/go/pull/3995))
* Correct `horizon db reingest range` output command name when invoking `horizon db detect-gaps` ([4007](https://github.com/stellar/go/pull/4007))
* Add new prometheus metrics:
  * `round_trip_time_seconds`:  time required to run `select 1` query in the DB ([4009](https://github.com/stellar/go/pull/4009))
  * `state_verify_ledger_entries_count`: number of ledger entries downloaded from buckets in a single state verifier run ([4015](https://github.com/stellar/go/pull/4015))
  * `ledger_fetch_duration_seconds`: duration of fetching ledgers from ledger backend, sliding window = 10m ([4016](https://github.com/stellar/go/pull/4016))


## v2.9.0

**Upgrading to this version from <= v2.8.3 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

**Protocol 18 support:** This release adds support for Protocol 18 ([CAP 38](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0038.md): Automated Market Makers).

### DB Schema Migration

* This release comes with a DB migration removing `offer_id` field from `history_trades` table and adding new tables related to AMM. It should not take more than 15 minutes to complete the migration.

### Breaking changes

* There are multiple breaking changes that will activate on Protocol 18 upgrade. Please check the [Horizon Liquidity Pool API](https://docs.google.com/document/d/1pXL8kr1a2vfYSap9T67R-g72B_WWbaE1YsLMa04OgoU/edit) doc for more information. Please upgrade to the latest SDKs that are backward compatible.
* The `--ingest` flag is set by default. If `--captive-core-config-path` is not set, the config file is generated based on network passhprase ([3783](https://github.com/stellar/go/pull/3783)).

### Changes

* **[CAP 38](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0038.md): Automated Market Makers) support.** All the API changes have been outlined in [Horizon Liquidity Pool API](https://docs.google.com/document/d/1pXL8kr1a2vfYSap9T67R-g72B_WWbaE1YsLMa04OgoU/edit) doc.
* Update `/paths` endpoint to take liquidity pools into account when searching for possible routes between assets ([3818](https://github.com/stellar/go/pull/3818)).
* Multiple performance improvements in `/paths`: [3816](https://github.com/stellar/go/pull/3816), [3965](https://github.com/stellar/go/pull/3965), [3933](https://github.com/stellar/go/pull/3933).
* Requests to `/paths` are now cancelled, respecting `--connection-timeout` flag value ([3081](https://github.com/stellar/go/pull/3081)).
* Multiple performance improvements to state ingestion processors: [3945](https://github.com/stellar/go/pull/3945), [3956](https://github.com/stellar/go/pull/3956), [3963](https://github.com/stellar/go/pull/3963), [3953](https://github.com/stellar/go/pull/3953), [3944](https://github.com/stellar/go/pull/3944).
* Add missing tx result codes in `txsub` ([3866](https://github.com/stellar/go/pull/3866)).
* Add new metric `ProcessorsRunDurationSummary`, old `ProcessorsRunDuration` is deprecated ([3940](https://github.com/stellar/go/pull/3940)).
* Logs during state ingesiton now display `progress` value which is percentage progress indicator ([3946](https://github.com/stellar/go/pull/3946)).

## v2.8.3
**Upgrading to this version from <= v2.8.0 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

### DB Schema Migration

* This release comes with a small DB migration. It should not take more than a couple minutes.

### Scheduled Changes

**In the 2.9.0 Horizon release, the `--ingest` flag will be set to `true` by default.**

### Changes

* Fix ingestion of fee bump transactions which have muxed source accounts ([3948](https://github.com/stellar/go/pull/3948)).
* Add an index on trade aggregations, to improve ingestion performance ([3947](https://github.com/stellar/go/pull/3947)).

## v2.8.2
**Upgrading to this version from <= v2.8.0 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

**In the 2.9.0 Horizon release, the `--ingest` flag will be set to `true` by default.**

* Improve performance of `OffersProcessor`. This should speed up ingestion of latest Stellar Public Network activity by up to 30%. Please note that this change does not improve reingestion speed because ledger entries are not processed during reingestion. ([3917](https://github.com/stellar/go/pull/3917))

## v2.8.1
**Upgrading to this version from <= v2.8.0 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

**In the 2.9.0 Horizon release, the `--ingest` flag will be set to `true` by default.**

* Fix bug in asset balance classification where clawback is enabled. ([3847](https://github.com/stellar/go/pull/3847))

## v2.8.0
**Upgrading to this version from <= v2.1.1 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

**In the 2.9.0 Horizon release, the `--ingest` flag will be set to `true` by default.**

* Limit reap to 100k ledgers/second, to prevent excess CPU usage ([3823](https://github.com/stellar/go/pull/3823)).
* Improve performance of path finding endpoints ([3818](https://github.com/stellar/go/pull/3818)).

## v2.7.0
**Upgrading to this version from <= v2.1.1 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

**In the 2.9.0 Horizon release, the `--ingest` flag will be set to `true` by default.**

* If `--captive-core-config-path` is not set, the config file is generated based on network passhprase. ([3783](https://github.com/stellar/go/pull/3783))
* Fix bug in horizon reap system (used by `horizon db reap` command and when horizon is configured with `--history-retention-count`) which could lead to partial deletions. ([3754](https://github.com/stellar/go/pull/3754))
* Log debug messages from captive core at the appropriate log level. ([3746](https://github.com/stellar/go/pull/3746))
* Add a feature flag `--captive-core-reuse-storage-path`/`CAPTIVE_CORE_REUSE_STORAGE_PATH` that will reuse Captive Core's storage path for bucket files when applicable for better performance. ([3750](https://github.com/stellar/go/pull/3750))

* Add the ability to filter accounts by their participation in a particular liquidity pool ([3873](https://github.com/stellar/go/pull/3873)).

### Update
* Include pool shares in account balances ([3873](https://github.com/stellar/go/pull/3873)).

## v2.6.1

**Upgrading to this version from <= v2.1.1 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

* Fix bug introduced in v2.6.0 ([#3737](https://github.com/stellar/go/pull/3737)), preventing usage of `horizon db migrate up/down/redo` commands. ([#3762](https://github.com/stellar/go/pull/3762))

## v2.6.0

**Upgrading to this version from <= v2.1.1 will trigger a state rebuild. During this process (which will take at least 10 minutes), Horizon will not ingest new ledgers.**

* Precompute trade aggregations during ingestion to improve performance. Will rebuild the aggregations as part of the database migrations. ([3641](https://github.com/stellar/go/pull/3641) & [3760](https://github.com/stellar/go/pull/3760)).
* Require `COUNT` param when running `horizon db migrate down COUNT` to prevent accidentally running all downwards migrations. Add `horizon db migrate status` command. ([#3737](https://github.com/stellar/go/pull/3737))
* Fix a bug in `fee_account_muxed` and `fee_account_muxed_id` fields (the fields were incorrectly populated with the source account details). ([3735](https://github.com/stellar/go/pull/3735))
* Validate ledger range when calling `horizon db reingest range` so that we respond with an error when attempting to ingest ledgers which are not available in the history archives. ([3738](https://github.com/stellar/go/pull/3738))
* Improve performance of transaction submission. ([3563](https://github.com/stellar/go/pull/3563))


## v2.5.2

**Upgrading to this version from <= v2.1.1 will trigger a state rebuild. During this process (which can take up to 20 minutes), Horizon will not ingest new ledgers.**

* Fix a bug in the method unmarshaling payment operation details. ([#3722](https://github.com/stellar/go/pull/3722))

## v2.5.1

**Upgrading to this version from <= v2.1.1 will trigger a state rebuild. During this process (which can take up to 20 minutes), Horizon will not ingest new ledgers.**

* Fix for Stellar-Core 17.1.0 bug that can potentially corrupt Captive-Core storage dir.
* All muxed ID fields are now represented as strings. This is to support JS that may not handle uint64 values in JSON responses properly.

## v2.5.0

**Upgrading to this version from <= v2.1.1 will trigger a state rebuild. During this process (which can take up to 20 minutes), Horizon will not ingest new ledgers.**

* Add new command `horizon db detect-gaps`, which detects ingestion gaps in the database. The command prints out the `db reingest` commands to run in order to fill the gaps found ([3672](https://github.com/stellar/go/pull/3672)).
* Performance improvement: Captive Core now reuses bucket files whenever it finds existing ones in the corresponding `--captive-core-storage-path` (introduced in [v2.1.0](#v2.1.0) rather than generating a one-time temporary sub-directory ([3670](https://github.com/stellar/go/pull/3670)). **This feature requires Stellar-Core version 17.1 or later.**
* Horizon now monitors the Stellar Core binary on disk (pointed to by `--stellar-core-binary-path`/`STELLAR_CORE_BINARY_PATH`) and restarts its Captive Core subprocess if it detects changes (i.e a more recent file timestamp for the Stellar Core binary) ([3687](https://github.com/stellar/go/pull/3687)).
* `POST /transactions` return `503 Service Unavailable` instead of `504 Gateway Timeout` if connected Stellar-Core is out of sync ([3653](https://github.com/stellar/go/pull/3653)).
* Add protocol version metrics: `horizon_ingest_max_supported_protocol_version`, `horizon_ingest_captive_stellar_core_supported_protocol_version`, `horizon_stellar_core_supported_protocol_version` ([3634](https://github.com/stellar/go/pull/3634)).
* Fixed crash in `horizon ingest verify-range` command ([3682](https://github.com/stellar/go/pull/3682)).
* Handle replica conflict errors gracefully ([3674](https://github.com/stellar/go/pull/3674)).
* Fix data race in request parameters handling ([3690](https://github.com/stellar/go/pull/3690)).
* Fix bug where the configuration for `CAPTIVE_CORE_LOG_PATH`, `CAPTIVE_CORE_PEER_PORT`, and `CAPTIVE_CORE_HTTP_PORT` were ignored if they were configured via environment variables instead of command line arguments. ([3702](https://github.com/stellar/go/pull/3702)).
* Error when setting `BUCKET_DIR_PATH` through `--captive-core-config-path` ([3707](https://github.com/stellar/go/pull/3707)).

## v2.4.1

**Upgrading to this version from <= v2.1.1 will trigger a state rebuild. During this process (which can take up to 20 minutes), Horizon will not ingest new ledgers.**

### Bug Fixes
* Fix bug in `horizon db reingest range` command, which would throw a duplicate entry conflict error from the DB. ([3661](https://github.com/stellar/go/pull/3661)).
* Fix bug in DB metrics preventing Horizon from starting when read-only replica middleware is enabled. ([3668](https://github.com/stellar/go/pull/3668)).
* Fix bug in the value of `route` in the logs for rate-limited requests (previously it was set to `undefined`). ([3658](https://github.com/stellar/go/pull/3658)).

## v2.4.0

**Upgrading to this version from <= v2.1.1 will trigger a state rebuild. During this process (which can take up to 20 minutes), Horizon will not ingest new ledgers.**

### DB State Migration

* This release comes with a small DB schema change (new multiplexed-account-related columns are incorporated). It should not take more than five minutes to run due to new columns being NULL-able.

### Deprecations

* Deprecate `--captive-core-config-append-path` in favor of `--captive-core-config-path`. The difference between the two flags is that `--captive-core-config-path` will validate the configuration file to reject any fields which are not supported by captive core ([3629](https://github.com/stellar/go/pull/3629)).

### New features

* Add more in-depth Prometheus metrics (count & duration) for db queries. ([3597](https://github.com/stellar/go/pull/3597), [3605](https://github.com/stellar/go/pull/3605))

* HTTP request logs will now print the Origin header if Referer is not set. ([3599](https://github.com/stellar/go/pull/3599))

* Add Multiplexed Account details to API responses (additional `_muxed` and `_muxed_id` optional fields following what's described in [SEP 23](https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0023.md#horizon-api-changes)):
  * Transactions: `account_muxed`, `account_muxed_id`, `fee_account` and `fee_account_muxed`.
  * Operations: `source_account_muxed`, `source_account_muxed_id` and additional fields depending on the operation (e.g. `from_muxed`, `from_muxed_id`, `to_muxed` and `to_muxed_id` for Payment operations)
  * Effects: `account_muxed`, `account_muxed_id` and additional fields depending on the effect (e.g. `seller_muxed` and `seller_muxed_id`  for the Trade effect).

### Code Changes

* Fix bug in `horizon db reingest range` command which required the `--ingest` flag to be set ([3625](https://github.com/stellar/go/pull/3625)).

* Fix bug in causing database connections to be closed when the HTTP request was cancelled. ([3630](https://github.com/stellar/go/pull/3630))

## v2.3.0

**Upgrading to this version from <= v2.1.1 will trigger a state rebuild. During this process (which can take up to 20 minutes), Horizon will not ingest new ledgers.**

### New features
* Introduces a flag (`--ro-database-url` / `RO_DATABASE_URL`) which allows setting a connection to a read-replica database. This flag makes Horizon take into account data propagation lag to the replica instance, adding retries if the data is out of sync ([3574](https://github.com/stellar/go/pull/3574)).

### Code changes
* Improved test suite coverage and stability ([3560](https://github.com/stellar/go/pull/3560), [3562](https://github.com/stellar/go/pull/3562), [3551](https://github.com/stellar/go/pull/3551), and [3547](https://github.com/stellar/go/pull/3547)).

* Improved session handling and timeouts ([3576](https://github.com/stellar/go/pull/3576), [3545](https://github.com/stellar/go/pull/3545), and [3567](https://github.com/stellar/go/pull/3567)).

* Improved stability of Captive Core's configuration options. Specifically, it will now prefer either the command-line parameter (e.g. `--captive-core-peer-port` or its env-var equivalent) or the user-supplied append file (`--captive-core-append-path`) over Horizon's internal defaults. However, if a value is set in *both* the append file and at the command-line, an error will be thrown unless both values are equal ([3558](https://github.com/stellar/go/pull/3558)).


## v2.2.0

**Upgrading to this version will trigger state rebuild. During this process (which can take up to 20 minutes) it will not ingest new ledgers.**

* Add `num_claimable_balances` and `claimable_balances_amount` fields to asset stat summaries at `/assets` ([3502](https://github.com/stellar/go/pull/3502)).
* Improve ingestion reliability when running multiple Horizon ingestion instances ([3518](https://github.com/stellar/go/pull/3518)).

## v2.1.1

* When ingesting a backlog of ledgers, Horizon sometimes consumes ledgers faster than the rate at which Captive Core emits them. Previously this scenario caused failures in the ingestion system. That is now fixed in ([3531](https://github.com/stellar/go/pull/3531)).

## v2.1.0

### DB State Migration

* This release comes with an internal DB represenatation change: the `claimable_balances` table now represents the claimable balance identifiers as an hexadecimal string (as opposed to base64).

**The migration will be performed by the ingestion system (through a state rebuild) and, thus, if some of your Horizon nodes are not ingestors (i.e. no `--ingestion` flag enabled) you may experience 500s in the `GET /claimable_balances/` requests until an ingestion node is upgraded. Also, it's worth noting that the rebuild process will take several minutes and no new ledgers will be ingested until the rebuild is finished.**

### Breaking changes

* Add a flag `--captive-core-storage-path`/`CAPTIVE_CORE_STORAGE_PATH` that allows users to control the storage location for Captive Core bucket data ([3479](https://github.com/stellar/go/pull/3479)).
  - Previously, Horizon created a directory in `/tmp` to store Captive Core bucket data. Now, if the captive core storage path flag is not set, Horizon will default to using the current working directory.
* Add a flag `--captive-core-log-path`/`CAPTIVE_CORE_LOG_PATH` that allows users to control the location of the logs emitted by Captive Core ([3472](https://github.com/stellar/go/pull/3472)). If you have a `LOG_FILE_PATH` entry in your Captive Core toml file remove that entry and use the horizon flag instead.
* `--stellar-core-db-url` / `STELLAR_CORE_DATABASE_URL` should only be configured if Horizon ingestion is enabled otherwise Horizon will not start ([3477](https://github.com/stellar/go/pull/3477)).

### New features

* Add an endpoint which determines if Horizon is healthy enough to receive traffic ([3435](https://github.com/stellar/go/pull/3435)).
* Sanitize route regular expressions for Prometheus metrics ([3459](https://github.com/stellar/go/pull/3459)).
* Add asset stat summaries per trust-line flag category ([3454](https://github.com/stellar/go/pull/3454)).
  - The `amount`, and `num_accounts` fields in `/assets` endpoint are deprecated. Fields will be removed in Horizon 3.0. You can find the same data under `balances.authorized`, and `accounts.authorized`, respectively.
* Add a flag `--captive-core-peer-port`/`CAPTIVE_CORE_PEER_PORT` that allows users to control which port the Captive Core subprocess will bind to for connecting to the Stellar swarm. ([3483](https://github.com/stellar/go/pull/3484)).
* Add 2 new HTTP endpoints `GET claimable_balances/{id}/transactions` and `GET claimable_balances/{id}/operations`, which respectively return the transactions and operations related to a provided Claimable Balance Identifier `{id}`.
* Add Stellar Protocol 16 support. This release comes with support for Protocol 16 ([CAP 35](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0035.md): asset clawback). See [the downstream SDK issue template](https://gist.github.com/2opremio/89c4775104635382d51b6f5e41cbf6d5) for details on what changed on Horizon's side. For full details, please read [CAP 35](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0035.md).


## v2.0.0

### Before you upgrade

Please read the [Captive Core](https://github.com/stellar/go/blob/master/services/horizon/internal/docs/captive_core.md) doc which contains new requirements and migration guide.

### Captive Stellar-Core

Introducing the stable release with Captive Stellar-Core mode enabled by default. Captive mode relaxes Horizon's operational requirements. It allows running Horizon without a fully fledged Core instance and, most importantly, without a Core database. More information about this new mode can be found in [Captive Core](https://github.com/stellar/go/blob/master/services/horizon/internal/docs/captive_core.md) doc.

If you run into issues please check [Known Issues](https://github.com/stellar/go/blob/master/services/horizon/internal/docs/captive_core.md#known-issues) or [report an issue](https://github.com/stellar/go/issues/new/choose). Please ask questions in [Keybase](https://keybase.io/team/stellar.public) or [Stack Exchange](https://stellar.stackexchange.com/).

### Breaking changes

* There are new config params (below) required by Captive Stellar-Core. Please check the [Captive Core](https://github.com/stellar/go/blob/master/services/horizon/internal/docs/captive_core.md) guide for migration tips.
  * `STELLAR_CORE_BINARY_PATH` - a path to Stellar-Core binary,
  * `CAPTIVE_CORE_CONFIG_APPEND_PATH` - defines a path to a file to append to the Stellar Core configuration file used by captive core.
* The `expingest` command has been renamed to `ingest` since the ingestion system is not experimental anymore.
* Removed `--rate-limit-redis-key` and `--redis-url` configuration flags.

## v2.0.0 Release Candidate

**This is a release candidate: while SDF is confident that there are no critical bugs and release candidate is safe to use in production we encourage organizations to deploy it to production only after org-specific testing.**

### Before you upgrade

Please read the [Captive Core](https://github.com/stellar/go/blob/release-horizon-v2.0.0-beta/services/horizon/internal/docs/captive_core.md) doc which contains new requirements and migration guide.

### Captive Stellar-Core

Introducing the release candidate with Captive Stellar-Core mode enabled by default. Captive mode relaxes Horizon's operational requirements. It allows running Horizon without a fully fledged Core instance and, most importantly, without a Core database. More information about this new mode can be found in [Captive Core](https://github.com/stellar/go/blob/release-horizon-v2.0.0-beta/services/horizon/internal/docs/captive_core.md) doc.

If you run into issues please check [Known Issues](https://github.com/stellar/go/blob/release-horizon-v2.0.0-beta/services/horizon/internal/docs/captive_core.md#known-issues) or [report an issue](https://github.com/stellar/go/issues/new/choose). Please ask questions in [Keybase](https://keybase.io/team/stellar.public) or [Stack Exchange](https://stellar.stackexchange.com/).

### Breaking changes

* The `expingest` command has been renamed to `ingest` since the ingestion system is not experimental anymore.
* Removed `--rate-limit-redis-key` and `--redis-url` configuration flags.

## v1.14.0

* Fix bug `/fee_stats` endpoint. The endpoint was not including the additional base fee charge for fee bump transactions ([#3354](https://github.com/stellar/go/pull/3354))
* Expose the timestamp of the most recently ingested ledger in the root resource response and in the `/metrics` response ([#3281](https://github.com/stellar/go/pull/3281))
* Add `--checkpoint-frequency` flag to configure how many ledgers span a history archive checkpoint ([#3273](https://github.com/stellar/go/pull/3273)). This is useful in the context of creating standalone Stellar networks in [integration tests](internal/docs/captive_core.md#private-networks).

## v1.13.1

**Upgrading to this version from version before v1.10.0 will trigger state rebuild. During this process (which can take several minutes) it will not ingest new ledgers.**

* Fixed a bug in `/fee_stats` endpoint that could calculate invalid stats if fee bump transactions were included in the ledger ([#3326](https://github.com/stellar/go/pull/3326))

## v2.0.0 Beta

**THIS IS A BETA RELEASE! DO NOT USE IN PRODUCTION. The release may contain critical bugs. It's not suitable for production use.**

### Before you upgrade

Please read the [Captive Core](https://github.com/stellar/go/blob/release-horizon-v2.0.0-beta/services/horizon/internal/docs/captive_core.md) doc which contains new requirements and migration guide.

### Captive Stellar-Core

Introducing the beta release with Captive Stellar-Core mode enabled by default. Captive mode relaxes Horizon's operational requirements. It allows running Horizon without a fully fledged Core instance and, most importantly, without a Core database. More information about this new mode can be found in [Captive Core](https://github.com/stellar/go/blob/release-horizon-v2.0.0-beta/services/horizon/internal/docs/captive_core.md) doc.

This version may contain bugs. If you run into issues please check [Known Issues](https://github.com/stellar/go/blob/release-horizon-v2.0.0-beta/services/horizon/internal/docs/captive_core.md#known-issues) or [report an issue](https://github.com/stellar/go/issues/new/choose). Please ask questions in [Keybase](https://keybase.io/team/stellar.public) or [Stack Exchange](https://stellar.stackexchange.com/).

## v1.13.0

**Upgrading to this version from version before v1.10.0 will trigger state rebuild. During this process (which can take several minutes) it will not ingest new ledgers.**

* Improved performance of `OfferProcessor` ([#3249](https://github.com/stellar/go/pull/3249)).
* Improved speed of state verification startup time ([#3251](https://github.com/stellar/go/pull/3251)).
* Multiple Captive Core improvements and fixes ([#3237](https://github.com/stellar/go/pull/3237), [#3257](https://github.com/stellar/go/pull/3257), [#3260](https://github.com/stellar/go/pull/3260), [#3264](https://github.com/stellar/go/pull/3264), [#3262](https://github.com/stellar/go/pull/3262), [#3265](https://github.com/stellar/go/pull/3265), [#3269](https://github.com/stellar/go/pull/3269), [#3271](https://github.com/stellar/go/pull/3271), [#3270](https://github.com/stellar/go/pull/3270), [#3272](https://github.com/stellar/go/pull/3272)).

## v1.12.0

* Add Prometheus metrics for the duration of ingestion processors ([#3224](https://github.com/stellar/go/pull/3224))
* Many Captive Core improvements and fixes ([#3232](https://github.com/stellar/go/pull/3232), [#3223](https://github.com/stellar/go/pull/3223), [#3226](https://github.com/stellar/go/pull/3226), [#3203](https://github.com/stellar/go/pull/3203), [#3189](https://github.com/stellar/go/pull/3189),  [#3187](https://github.com/stellar/go/pull/3187))

## v1.11.1

* Fix bug in parsing `db-url` parameter in `horizon db migrate` and `horizon db init` commands ([#3192](https://github.com/stellar/go/pull/3192)).

## v1.11.0

* The `service` field emitted in ingestion logs has been changed from `expingest` to `ingest` ([#3118](https://github.com/stellar/go/pull/3118)).
* Ledger stats are now exported in `/metrics` in `horizon_ingest_ledger_stats_total` metric ([#3148](https://github.com/stellar/go/pull/3148)).
* Stellar Core database URL is no longer required when running in captive mode ([#3150](https://github.com/stellar/go/pull/3150)).
* xdr: Add a custom marshaller for claim predicate timestamp  ([#3183](https://github.com/stellar/go/pull/3183)).

## v1.10.1

* Bump max supported protocol version to 15.

## v1.10.0

**After upgrading Horizon will rebuild its state. During this process (which can take several minutes) it will not ingest new ledgers.**

* Fixed a bug that caused a fresh instance of Horizon to be unable to sync with testnet (Protocol 14) correctly. ([#3100](https://github.com/stellar/go/pull/3100))
* Add Golang- and process-related metrics. ([#3103](https://github.com/stellar/go/pull/3103))
* New `network_passphrase` field in History Archives (added in Stellar-Core 14.1.0) is now checked. Horizon will return error if incorrect archive is used. ([#3082](https://github.com/stellar/go/pull/3082))
* Fixed a bug that caused some errors to be logged with `info` level instead of `error` level. ([#3094](https://github.com/stellar/go/pull/3094))
* Fixed a bug in `/claimable_balances` that returned 500 error instead of 400 for some requests. ([#3088](https://github.com/stellar/go/pull/3088))
* Print a friendly message when Horizon does not support the current Stellar protocol version. ([#3093](https://github.com/stellar/go/pull/3093))

## v1.9.1

* Fixed a bug that caused a fresh instance of Horizon to be unable to sync with testnet (Protocol 14) correctly. ([#3096](https://github.com/stellar/go/pull/3096))
* Use underscore in JSON fields for claim predicate to make the API consistent. ([#3086](https://github.com/stellar/go/pull/3086))

## v1.9.0

This is release adds support for the upcoming Protocol 14 upgrade. However, Horizon still maintains backwards compatibility with Protocol 13, which means it is still safe to run this release before Protocol 14 is deployed.

**After upgrading Horizon will rebuild it's state. During this process (which can take several minutes) it will not ingest new ledgers.**

The two main features of Protocol 14 are [CAP 23 Claimable Balances](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0023.md) and [CAP 33 Sponsored Reserves](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0033.md).
Claimable balances provide a mechanism for setting up a payment which can be claimed in the future. This allows you to make payments to accounts which are currently not able to accept them.
Sponsored Reserves allows an account to pay the reserves on behalf of another account.

In this release there is a new claimable balance resource which has a unique id, an asset (describing which asset can be claimed), an amount (the amount of the asset that can be claimed), and a list of claimants (an immutable list of accounts that could potentially claim the balance).
The `GET /claimable_balances/{id}` endpoint was added to Horizon's API to allow looking up a claimable balance by its id. See the sample response below:

```json
{
  "_links": {
    "self": {
      "href": "/claimable_balances/000000000102030000000000000000000000000000000000000000000000000000000000"
    }
  },
  "id": "000000000102030000000000000000000000000000000000000000000000000000000000",
  "asset": "native",
  "amount": "10.0000000",
  "sponsor": "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
  "last_modified_ledger": 123,
  "claimants": [
    {
      "destination": "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
      "predicate": {
        "unconditional": true
      }
    }
  ],
  "paging_token": "123-000000000102030000000000000000000000000000000000000000000000000000000000"
}
```

There is also a `GET /claimable_balances` endpoint which searches for claimable balances by asset, sponsor, or claimant destination.

To support [CAP 33 Sponsored Reserves](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0033.md) we have added an optional `sponsor` attribute in the following Horizon resources: accounts, account signers, offers, trustlines, and claimable balances.
If the `sponsor` field is present it means that the account with id `sponsor` is paying for the reserves for the sponsored account / account signer / offer / trustline / claimable balance. We have also added an optional `sponsor` query parameter to the following endpoints:
* `GET /accounts`
* `GET /offers`
* `GET /claimable_balances`

If the `sponsor` query param is provided, Horizon will search for objects sponsored by the given account id.

## v1.8.2

* Fixed a bug which prevented Horizon from accepting TLS connections.

## v1.8.1

* Fixed a bug in a code ingesting fee bump transactions.

## v1.8.0

### Changes

* Added new and changed existing metrics:
  * `horizon_build_info` - contains build information in labels (`version` - Horizon version, `goversion` - Go runtime version),
  * `horizon_ingest_enable` - equals `1` if ingestion system is running, `0` otherwise,
  * `horizon_ingest_state_invalid` - equals `1` if state is invalid, `0` otherwise,
  * `horizon_db_max_open_connections` - determines the maximum possible opened DB connections,
  * `horizon_db_wait_duration_seconds_total` - changed the values to be in seconds instead of nanoseconds.
* Fixed a data race when shutting down the HTTP server. ([#2958](https://github.com/stellar/go/pull/2958)).
* Fixed emitting incorrect errors related to OrderBook Stream when shutting down the app. ([#2964](https://github.com/stellar/go/pull/2964))

### Experimental

The previous implementation of Captive Stellar-Core streams meta stream using a filesystem pipe. This implies that both Horizon and Stellar-Core had to be deployed to the same server. One of the disadvantages of such requirement is a need for detailed per-process monitoring to be able to connect potential issues (like memory leaks) to the specific service.

To solve this it's now possible to start a [`captivecore`](https://github.com/stellar/go/tree/master/exp/services/captivecore) on another machine and configure Horizon to use it in ingestion. This requires two config options set:
* `ENABLE_CAPTIVE_CORE_INGESTION=true`,
* `REMOTE_CAPTIVE_CORE_URL` - pointing to `captivecore` server.

## v1.7.1

This patch release fixes a regression introduced in 1.7.0, breaking the
 `/offers` endpoint. Thus, we recommend upgrading as soon as possible.

### Changes
* Fix path parameter mismatch in `/offers` endpoint
  [#2927](https://github.com/stellar/go/pull/2927).

## v1.7.0

### DB schema migration (expected migration time: < 10 mins)
  * Add new multicolumn index to improve the `/trades`'s
    endpoint performance [#2869](https://github.com/stellar/go/pull/2869).
  * Add constraints on database columns which cannot hold
    negative values [#2827](https://github.com/stellar/go/pull/2827).

### Changes
* Update Go toolchain to 1.14.6 in order to fix [golang/go#34775](https://github.com/golang/go/issues/34775),
  which caused some database queries to be executed instead of rolled back.
* Fix panic on missing command line arguments [#2872](https://github.com/stellar/go/pull/2872)
* Fix race condition where submitting a transaction to Horizon can result in a bad sequence error even though Stellar Core accepted the transaction. [#2877](https://github.com/stellar/go/pull/2877)
* Add new DB metrics ([#2844](https://github.com/stellar/go/pull/2844)):
  * `db_in_use_connections` - number of opened DB connections in use (not idle),
  * `db_wait_count` - number of connections waited for,
  * `db_wait_duration` - total time blocked waiting for a new connection.

## v1.6.0

* Add `--parallel-workers` and `--parallel-job-size` to `horizon db reingest range`. `--parallel-workers` will parallelize reingestion using the supplied number of workers. ([#2724](https://github.com/stellar/go/pull/2724))
* Remove Stellar Core's database dependency for non-ingesting instances of Horizon.  ([#2759](https://github.com/stellar/go/pull/2759))
  Horizon doesn't require access to a Stellar Core database if it is only serving HTTP request, this allows the separation of front-end and ingesting instances.
  The following config parameters were removed:
  - `core-db-max-open-connections`
  - `core-db-max-idle-connections`
* HAL response population is implemented using Go `strings` package instead of `regexp`, improving its performance. ([#2806](https://github.com/stellar/go/pull/2806))
* Fix a bug in `POST /transactions` that could cause `tx_bad_seq` errors instead of processing a valid transaction. ([#2805](https://github.com/stellar/go/pull/2805))
* The `--connection-timeout` param is ignored in `POST /transactions`. The requests sent to that endpoint will always timeout after 30 seconds. ([#2818](https://github.com/stellar/go/pull/2818))

### Experimental

* Add experimental support for live ingestion using a Stellar Core subprocess instead of a persistent Stellar Core database.

  Stellar-core now contains an experimental feature which allows replaying ledger's metadata in-memory. This feature starts paving the way to remove the dependency between Stellar Core's database and Horizon. Requires [Stellar Core v13.2.0](https://github.com/stellar/stellar-core/releases/tag/v13.2.0).

  To try out this new experimental feature, you need to specify the following parameters when starting ingesting Horizon instance:

  - `--enable-captive-core-ingestion` or `ENABLE_CAPTIVE_CORE_INGESTION=true`.
  - `--stellar-core-binary-path` or `STELLAR_CORE_BINARY_PATH`.

## v1.5.0

### Changes

* Remove `--ingest-failed-transactions` flag. From now on Horizon will always ingest failed transactions. WARNING: If your application is using Horizon DB directly (not recommended!) remember that now it will also contain failed txs. ([#2702](https://github.com/stellar/go/pull/2702)).
* Add transaction set operation count to `history_ledger`([#2690](https://github.com/stellar/go/pull/2690)).
Extend ingestion to store the total number of operations in the transaction set and expose it in the ledger resource via `tx_set_operation_count`. This feature allows you to assess the used capacity of a transaction set.
* Fix `/metrics` end-point ([#2717](https://github.com/stellar/go/pull/2717)).
* Gracefully handle incorrect assets in the query parameters of GET `/offers` ([#2634](https://github.com/stellar/go/pull/2634)).
* Fix logging message in OrderBookStream ([#2699](https://github.com/stellar/go/pull/2699)).
* Fix data race in root endpoint ([#2745](https://github.com/stellar/go/pull/2745)).

### Experimental

* Add experimental support for database reingestion using a Stellar Core subprocess instead of a persistent Stellar Core database ([#2695](https://github.com/stellar/go/pull/2695)).

  [Stellar Core v12.3.0](https://github.com/stellar/stellar-core/releases/tag/v12.3.0) added an experimental feature which allows replaying ledger's metadata in-memory. This feature speeds up reingestion and starts paving the way to remove the dependency between Stellar Core's database and Horizon.

  For now, this is only supported while running `horizon db reingest`. To try out this new experimental feature, you need to specify the following parameters:

  - `--enable-captive-core-ingestion` or `ENABLE_CAPTIVE_CORE_INGESTION=true`.
  - `--stellar-core-binary-path` or `STELLAR_CORE_BINARY_PATH`.

### SDK Maintainers: action needed

- Add the new field `tx_set_operation_count` to the `ledger` resource ([#2690](https://github.com/stellar/go/pull/2690)). This field can be a `number` or `null`.

## v1.4.0

* Drop support for MuxedAccounts strkeys (spec'ed in [SEP23](https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0023.md)).
  SEP23 is still a draft and we don't want to encourage storing strkeys which may not be definite.
* Replace `SequenceProvider` implementation with one which queries the Horizon DB for sequence numbers instead of the Stellar Core DB.
* Use the Horizon DB instead of Horizon's in memory order book graph to query orderbook details for the /order_book endpoint.
* Remove JSON variant of `GET /metrics`, both in the server and client code. It's using Prometheus format by default now.
* Decreased a memory usage of initial state ingestion stage and state verifier ([#2618](https://github.com/stellar/go/pull/2618)).
* Remove `--exp-ingest-in-memory-only` Horizon flag. The in memory order book graph which powers the path finding endpoints is now updated using the Horizon DB instead of directly via ingestion ([#2630](https://github.com/stellar/go/pull/2630)).

## v1.3.0

### Breaking changes

* The type for the following attributes has been changed from `int64` to `string` ([#2555](https://github.com/stellar/go/pull/2555)):
  - Attribute `fee_charged` in [Transaction](https://www.stellar.org/developers/horizon/reference/resources/transaction.html) resource.
  - Attribute `max_fee` in [Transaction](https://www.stellar.org/developers/horizon/reference/resources/transaction.html) resource.

### Changes

* Add `last_modified_time` to account responses. `last_modified_time` is the closing time of the most recent ledger in which the account was modified ([#2528](https://github.com/stellar/go/pull/2528)).
* Balances in the Account resource are now sorted by asset code and asset issuer ([#2516](https://github.com/stellar/go/pull/2516)).
* Ingestion system has its dedicated DB connection pool ([#2560](https://github.com/stellar/go/pull/2560)).
* A new metric has been added to `/metrics` ([#2537](https://github.com/stellar/go/pull/2537) and [#2553](https://github.com/stellar/go/pull/2553)):
  - `ingest.local_latest_ledger`: a gauge with the local latest ledger,
  - `txsub.v0`: a meter counting `v0` transactions in `POST /transaction`,
  - `txsub.v1`: a meter counting `v1` transactions in `POST /transaction`,
  - `txsub.feebump`: a meter counting `feebump` transactions in `POST /transaction`.
* Fix a memory leak in the code responsible for streaming ([#2548](https://github.com/stellar/go/pull/2548), [#2575](https://github.com/stellar/go/pull/2575) and [#2576](https://github.com/stellar/go/pull/2576)).

## v1.2.2

* Fix bug which occurs when ingesting ledgers containing both fee bump and normal transactions.

## v1.2.1

### Database migration notes

This version removes two unused columns that could overflow in catchup complete deployments. If your Horizon database contains entire public network history, you should upgrade to this version as soon as possible and run `horizon db migrate up`.

### Changes

* Remove `id` columns from `history_operation_participants` and `history_transaction_participants` to prevent possible integer overflow [#2532](https://github.com/stellar/go/pull/2532).
## v1.2.0

### Scheduled Breaking Changes

* The type for the following attributes will be changed from `int64` to `string` in 1.3.0:
  - Attribute `fee_charged` in [Transaction](https://www.stellar.org/developers/horizon/reference/resources/transaction.html) resource.
  - Attribute `max_fee` in [Transaction](https://www.stellar.org/developers/horizon/reference/resources/transaction.html) resource.

The changes are required by [CAP-15](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0015.md).

### Changes

* Added support for [CAP-27](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0027.md) and [SEP-23](https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0023.md) [#2491](https://github.com/stellar/go/pull/2491).
* The XDR definition of a transaction memo is a string.
However, XDR strings are actually binary blobs with no enforced encoding.
It is possible to set the memo in a transaction envelope to a binary sequence which is not valid ASCII or unicode.
Previously, if you wanted to recover the original binary sequence for a transaction memo, you would have to decode the transaction's envelope.
In this release, we have added a `memo_bytes` field to the Horizon transaction response for transactions with `memo_type` equal `text`.
`memo_bytes` stores the base 64 encoding of the memo bytes set in the transaction envelope [#2485](https://github.com/stellar/go/pull/2485).

## v1.1.0

### **IMPORTANT**: Database migration

This version includes a significant database migration which changes the column types of `fee_charged` and `max_fee` in the `history_transactions` table from `integer` to `bigint`. This essential change paves the way for fee bump transactions ([CAP 15](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0015.md)), a major improvement that will be released soon in Stellar Protocol 13.

This migration will run for a long time, especially if you have a Horizon database with full history. For reference, it took around 8 hours and 42 minutes to complete this migration on a AWS db.r4.8xlarge instance with full transaction history.

To execute the migration run `horizon db migrate up` using the Horizon v1.1.0 binary.

**Important Note**: Horizon should not be serving requests or ingesting while the migration is running. For service continuity, if you run a production Horizon deployment it is recommended that you perform the migration on a second instance and then switch over.

### Changes
* DB: Remove unnecessary duplicate indexes: `index_history_transactions_on_id`, `index_history_ledgers_on_id`, `exp_asset_stats_by_code`, and `asset_by_code` ([#2419](https://github.com/stellar/go/pull/2419)).
* DB: Remove asset_stats table which is no longer necessary ([#2419](https://github.com/stellar/go/pull/2419)).
* Validate transaction hash IDs as 64 lowercase hex chars. As such, wrongly-formatted parameters which used to cause 404 (`Not found`) errors will now cause 400 (`Bad request`) HTTP errors ([#2394](https://github.com/stellar/go/pull/2394)).
* Fix ask and bid price levels of `GET /order_book` when encountering non-canonical price values. The `limit` parameter is now respected and levels are coallesced properly. Also, `price_r` is now in canonical form ([#2400](https://github.com/stellar/go/pull/2400)).
* Added missing top-level HAL links to the `GET /` response ([#2407](https://github.com/stellar/go/pull/2407)).
* Full transaction details are now included in the `POST /transactions` response. If you submit a transaction and it succeeds, the response will match the `GET /transactions/{hash}` response ([#2406](https://github.com/stellar/go/pull/2406)).
* The following attributes are now included in the transaction resource:
    * `fee_account` (the account which paid the transaction fee)
    * `fee_bump_transaction` (only present in Protocol 13 fee bump transactions)
    * `inner_transaction` (only present in Protocol 13 fee bump transactions) ([#2406](https://github.com/stellar/go/pull/2406)).
* Add support for [CAP0018](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0018.md): Fine-Grained Control of Authorization (Protocol 13) ([#2423](https://github.com/stellar/go/pull/2423)).
  - Add `is_authorized_to_maintain_liabilities` to `Balance`.
    <pre>
    "balances": [
      {
        "is_authorized": true,
        <b>"is_authorized_to_maintain_liabilities": true,</b>
        "balance": "27.1374422",
        "limit": "922337203685.4775807",
        "buying_liabilities": "0.0000000",
        "selling_liabilities": "0.0000000",
        "last_modified_ledger": 28893780,
        "asset_type": "credit_alphanum4",
        "asset_code": "USD",
        "asset_issuer": "GBSTRUSD7IRX73RQZBL3RQUH6KS3O4NYFY3QCALDLZD77XMZOPWAVTUK"
      },
      {
        "balance": "1.5000000",
        "buying_liabilities": "0.0000000",
        "selling_liabilities": "0.0000000",
        "asset_type": "native"
      }
    ]
    </pre>
  - Add `authorize_to_maintain_liabilities` to `AllowTrust` operation.
    <pre>
    {
      "id": "124042211741474817",
      "paging_token": "124042211741474817",
      "transaction_successful": true,
      "source_account": "GBSTRUSD7IRX73RQZBL3RQUH6KS3O4NYFY3QCALDLZD77XMZOPWAVTUK",
      "type": "allow_trust",
      "type_i": 7,
      "created_at": "2020-03-27T03:40:10Z",
      "transaction_hash": "a77d4ee5346d55fb8026cdcdad6e4b5e0c440c96b4627e3727f4ccfa6d199e94",
      "asset_type": "credit_alphanum4",
      "asset_code": "USD",
      "asset_issuer": "GBSTRUSD7IRX73RQZBL3RQUH6KS3O4NYFY3QCALDLZD77XMZOPWAVTUK",
      "trustee": "GBSTRUSD7IRX73RQZBL3RQUH6KS3O4NYFY3QCALDLZD77XMZOPWAVTUK",
      "trustor": "GA332TXN6BX2DYKGYB7FW5BWV2JLQKERNX4T7EUJT4MHWOW2TSGC2SPM",
      "authorize": true,
      <b>"authorize_to_maintain_liabilities": true,</b>
    }
    </pre>
  - Add effect `trustline_authorized_to_maintain_liabilities`.
    <pre>
    {
      "id": "0124042211741474817-0000000001",
      "paging_token": "124042211741474817-1",
      "account": "GBSTRUSD7IRX73RQZBL3RQUH6KS3O4NYFY3QCALDLZD77XMZOPWAVTUK",
      <b>"type": "trustline_authorized_to_maintain_liabilities",</b>
      <b>"type_i": 25,</b>
      "created_at": "2020-03-27T03:40:10Z",
      "trustor": "GA332TXN6BX2DYKGYB7FW5BWV2JLQKERNX4T7EUJT4MHWOW2TSGC2SPM",
      "asset_type": "credit_alphanum4",
      "asset_code": "USD"
    }
    </pre>
* It is no longer possible to use Redis as a mechanism for rate-limiting requests ([#2409](https://github.com/stellar/go/pull/2409)).

* Make `GET /trades` generate an empty response instead of a 404 when no
 trades are found.

## v1.0.1

### Fixed
* Fix `horizon db reap` bug which caused the command to exit without deleting any history table rows ([#2336](https://github.com/stellar/go/pull/2336)).
* The horizon reap system now also deletes rows from `history_trades`. Previously, the reap system only deleted rows from `history_operation_participants`, `history_operations`, `history_transaction_participants`, `history_transactions`, `history_ledgers`, and `history_effects` ([#2336](https://github.com/stellar/go/pull/2336)).
* Fix deadlock when running `horizon db reingest range` ([#2373](https://github.com/stellar/go/pull/2373)).
* Fix signer update effects ([#2375](https://github.com/stellar/go/pull/2375)).
* Fix incorrect error in log when shutting down the system while `verifyState` is running ([#2366](https://github.com/stellar/go/pull/2366)).
* Expose date header to CORS clients ([#2316](https://github.com/stellar/go/pull/2316)).
* Fix inconsistent ledger view in `/accounts/{id}` when streaming ([#2344](https://github.com/stellar/go/pull/2344)).

### Removed
* Dropped support for Go 1.12. ([#2346](https://github.com/stellar/go/pull/2346)).

## v1.0.0

### Before you upgrade

* If you were using the new ingestion in one of the previous versions of Horizon, you must first remove `ENABLE_EXPERIMENTAL_INGESTION` feature flag and restart all Horizon instances before deploying a new version.
* The init stage (state ingestion) for the public Stellar network requires around 1.5GB of RAM. This memory is released after the state ingestion. State ingestion is performed only once. Restarting the server will not trigger it unless Horizon has been upgraded to a newer version (with an updated ingestion pipeline). It's worth noting that the required memory will become smaller and smaller as more of the buckets in the history archive become [CAP-20](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0020.md) compatible. Some endpoints are **not available** during state ingestion.
* The CPU footprint of the new ingestion is modest. We were able to successfully run ingestion on an [AWS `c5.xlarge`](https://aws.amazon.com/ec2/instance-types/c5/) instance. The init stage takes a few minutes on `c5.xlarge`. `c5.xlarge` is the equivalent of 4 vCPUs and 8GB of RAM. The definition of vCPU for the c5 large family in AWS is the following:
> The 2nd generation Intel Xeon Scalable Processors (Cascade Lake) or 1st generation Intel Xeon Platinum 8000 series (Skylake-SP) processor with a sustained all core Turbo frequency of up to 3.4GHz, and single core turbo frequency of up to 3.5 GHz.

* The state data requires an additional 6GB DB disk space for the public Stellar network (as of February 2020). The disk usage will increase when the number of Stellar ledger entries increases.
  * `accounts_signers` table: 2340 MB
  * `trust_lines` table: 2052 MB
  * `accounts` table: 1545 MB
  * `offers` table: 61 MB
  * `accounts_data` table: 15 MB
  * `exp_asset_stats` table: less than 1 MB
* A new environment variable (or command line flag) needs to be set so that Horizon can ingest state from the history archives:
   * `HISTORY_ARCHIVE_URLS="archive1,archive2,archive3"` (if you don't have your own pubnet history archive, you can use one of SDF's archives, for example `https://history.stellar.org/prd/core-live/core_live_001`)
* Horizon serves the endpoints `/paths` and `/order_book` from an in-memory graph, which is only available on ingesting instances. If some of the instances in your cluster are not configured to ingest, you can configure your proxy server to route those endpoints to the ingesting instances. This is beyond the scope of this document - consult the relevant documentation for your proxy server. A better solution for this will be released in the next Horizon version.

### New Ingestion System

The most substantial element of this release is a full rewrite of Horizon's ledger ingestion engine, which enables some key features:

* A set of important new endpoints (see below). Some of these were impossible under the previous ingestion architecture.
* An in-memory order book graph for rapid querying.
* The ability to run parallel ingestion over multiple Horizon hosts, improving service availability for production deployments.

The new engine resolves multiple issues that were present in the old system. For example:

* Horizon's coupling to Stellar-Core's database is dramatically reduced.
* Data inconsistency due to lag between endpoints is eliminated.
* Slow endpoints (path-finding for example) are now speedy.

Finally, the rearchitecting makes new reliability features possible. An example is the new internal state verifier, which guarantees consistency between the local Horizon state and the public history archives.

The [admin guide](https://github.com/stellar/go/blob/release-horizon-v0.25.0/services/horizon/internal/docs/admin.md) contains all the information needed to operate the new ingestion system.

### Added

- Add [/accounts](https://www.stellar.org/developers/horizon/reference/endpoints/accounts.html) endpoint, which allows filtering accounts that have a given signer or a trustline to an asset.
- Add [/offers](https://www.stellar.org/developers/horizon/reference/endpoints/offers.html) endpoint, which lists all offers on the network and allows filtering by seller account or by selling or buying asset.
- Add [/paths/strict-send](https://www.stellar.org/developers/horizon/reference/endpoints/path-finding-strict-send.html) endpoint, which enables discovery of optimal "strict send" paths between assets.
- Add [/paths/strict-receive](https://www.stellar.org/developers/horizon/reference/endpoints/path-finding-strict-receive.html) endpoint, which enables discovery of optimal "strict receive" paths between assets.
- Add the fields `max_fee` and `fee_charged` to [/fee_stats](https://www.stellar.org/developers/horizon/reference/endpoints/fee-stats.html).

### Breaking changes

### Changed

- Change multiple operation types to their canonical names for [operation resources](https://www.stellar.org/developers/horizon/reference/resources/operation.html) ([#2134](https://github.com/stellar/go/pull/2134)).
- Change the type of the following fields from `number` to `string`:

    - Attribute `offer_id` in [manage buy offer](https://www.stellar.org/developers/horizon/reference/resources/operation.html#manage-buy-offer) and [manage sell offer](https://www.stellar.org/developers/horizon/reference/resources/operation.html#manage-sell-offer) operations.
    - Attribute `offer_id` in `Trade` [effect](https://www.stellar.org/developers/horizon/reference/resources/effect.html#trading-effects).
    - Attribute `id` in [Offer](https://www.stellar.org/developers/horizon/reference/resources/offer.html) resource.
    - Attribute `timestamp` and `trade_count` in [Trade Aggregation](https://www.stellar.org/developers/horizon/reference/resources/trade_aggregation.html) resource.

    See [#1609](https://github.com/stellar/go/issues/1609), [#1909](https://github.com/stellar/go/pull/1909) and [#1912](https://github.com/stellar/go/issues/1912) for more details.

### Removed

- `/metrics` endpoint is no longer part of the public API. It is now served on `ADMIN_PORT/metrics`. `ADMIN_PORT` can be set using env variable or `--admin-port` CLI param.
- Remove the following fields from [/fee_stats](https://www.stellar.org/developers/horizon/reference/endpoints/fee-stats.html):

    - `min_accepted_fee`
    - `mode_accepted_fee`
    - `p10_accepted_fee`
    - `p20_accepted_fee`
    - `p30_accepted_fee`
    - `p40_accepted_fee`
    - `p50_accepted_fee`
    - `p60_accepted_fee`
    - `p70_accepted_fee`
    - `p80_accepted_fee`
    - `p90_accepted_fee`
    - `p95_accepted_fee`
    - `p99_accepted_fee`

- Remove `fee_paid` field from [Transaction resource](https://www.stellar.org/developers/horizon/reference/resources/transaction.html) (Use `fee_charged` and `max_fee` fields instead - see [#1372](https://github.com/stellar/go/issues/1372)).

## v0.24.1

* Add cache to improve performance of experimental ingestion system (#[2004](https://github.com/stellar/go/pull/2004)).
* Fix experimental ingestion bug where ledger changes were not applied in the correct order (#[2050](https://github.com/stellar/go/pull/2050)).
* Fix experimental ingestion bug where unique constraint errors are incurred when the ingestion system has to reingest state from history archive checkpoints (#[2055](https://github.com/stellar/go/pull/2055)).
* Fix experimental ingestion bug where a race condition during shutdown leads to a crash (#[2058](https://github.com/stellar/go/pull/2058)).

## v0.24.0

* Add `fee_charged` and `max_fee` objects to `/fee_stats` endpoint ([#1964](https://github.com/stellar/go/pull/1964)).
* Experimental ledger header ingestion processor ([#1949](https://github.com/stellar/go/pull/1949)).
* Improved performance of asset stats processor ([#1987](https://github.com/stellar/go/pull/1987)).
* Provide mechanism for retrying XDR stream errors ([#1899](https://github.com/stellar/go/pull/1899)).
* Emit error level log after 3 failed attempts to validate state ([#1918](https://github.com/stellar/go/pull/1918)).
* Fixed out of bounds error in ledger backend reader ([#1914](https://github.com/stellar/go/pull/1914)).
* Fixed out of bounds error in URL params handler ([#1973](https://github.com/stellar/go/pull/1973)).
* Rename `OperationFeeStats` to `FeeStats` ([#1952](https://github.com/stellar/go/pull/1952)).
* All DB queries are now cancelled when request is cancelled/timeout. ([#1950](https://github.com/stellar/go/pull/1950)).
* Fixed multiple issues connected to graceful shutdown of Horizon.

### Scheduled Breaking Changes

* All `*_accepted_fee` fields in `/fee_stats` endpoint are deprecated. Fields will be removed in Horizon 0.25.0.

Previously scheduled breaking changes reminders:

* The following operation type names have been deprecated: `path_payment`, `manage_offer` and `create_passive_offer`. The names will be changed to: `path_payment_strict_receive`, `manage_sell_offer` and `create_passive_sell_offer` in 0.25.0. This has been previously scheduled for 0.22.0 release.
* `fee_paid` field on Transaction resource has been deprecated and will be removed in 0.25.0 (previously scheduled for 0.22.0). Please use new fields added in 0.18.0: `max_fee` that defines the maximum fee the source account is willing to pay and `fee_charged` that defines the fee that was actually paid for a transaction. See [CAP-0005](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0005.md) for more information.
* The type for the following attributes will be changed from `int64` to `string` in 0.25.0 (previously scheduled for 0.22.0):
  - Attribute `offer_id` in [manage buy offer](https://www.stellar.org/developers/horizon/reference/resources/operation.html#manage-buy-offer) and [manage sell offer](https://www.stellar.org/developers/horizon/reference/resources/operation.html#manage-sell-offer) operations.
  - Attribute `offer_id` in `Trade` effect.
  - Attribute `id` in [Offer](https://www.stellar.org/developers/horizon/reference/resources/offer.html) resource.
  - Attribute `timestamp` and `trade_count` in [Trade Aggregation](https://www.stellar.org/developers/horizon/reference/resources/trade_aggregation.html) resource.

Check [Beta Testing New Ingestion System](https://github.com/stellar/go/blob/master/services/horizon/internal/expingest/BETA_TESTING.md) if you want to test the new ingestion system.

## v0.23.1

* Add `ReadTimeout` to Horizon HTTP server configuration to fix potential DoS vector.

## v0.23.0

* New features in experimental ingestion (to enable: set `--enable-experimental-ingestion` CLI param or `ENABLE_EXPERIMENTAL_INGESTION=true` env variable):
  * All state-related endpoints (i.e. ledger entries) are now served from Horizon DB (except `/account/{account_id}`)

  * `/order_book` offers data is served from in-memory store ([#1761](https://github.com/stellar/go/pull/1761))

  * Add `Latest-Ledger` header with the sequence number of the most recent ledger processed by the experimental ingestion system. Endpoints built on the experimental ingestion system will always respond with data which is consistent with the ledger in `Latest-Ledger` ([#1830](https://github.com/stellar/go/pull/1830))

  * Add experimental support for filtering accounts who are trustees to an asset via `/accounts`. Example:\
  `/accounts?asset=COP:GC2GFGZ5CZCFCDJSQF3YYEAYBOS3ZREXJSPU7LUJ7JU3LP3BQNHY7YKS`\
  returns all accounts who have a trustline to the asset `COP` issued by account `GC2GFG...` ([#1835](https://github.com/stellar/go/pull/1835))

  * Experimental "Accounts For Signers" end-point now returns a full account resource ([#1876](https://github.com/stellar/go/issues/1875))
* Prevent "`multiple response.WriteHeader calls`" errors when streaming ([#1870](https://github.com/stellar/go/issues/1870))
* Fix an interpolation bug in `/fee_stats` ([#1857](https://github.com/stellar/go/pull/1857))
* Fix a bug in `/paths/strict-send` where occasionally bad paths were returned ([#1863](https://github.com/stellar/go/pull/1863))

## v0.22.2

* Fixes a bug in accounts for signer ingestion processor.

## v0.22.1

* Fixes a bug in path payment ingestion code.

## v0.22.0

* Adds support for Stellar Protocol v12.

### Scheduled Breaking Changes

* The following operation type names have been deprecated: `path_payment`, `manage_offer` and `create_passive_offer`. The names will be changed to: `path_payment_strict_receive`, `manage_sell_offer` and `create_passive_sell_offer` in 0.25.0. This has been previously scheduled for 0.22.0 release.
* `fee_paid` field on Transaction resource has been deprecated and will be removed in 0.23.0 (previously scheduled for 0.22.0). Please use new fields added in 0.18.0: `max_fee` that defines the maximum fee the source account is willing to pay and `fee_charged` that defines the fee that was actually paid for a transaction. See [CAP-0005](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0005.md) for more information.
* The type for the following attributes will be changed from `int64` to `string` in 0.23.0 (previously scheduled for 0.22.0):
  - Attribute `offer_id` in [manage buy offer](https://www.stellar.org/developers/horizon/reference/resources/operation.html#manage-buy-offer) and [manage sell offer](https://www.stellar.org/developers/horizon/reference/resources/operation.html#manage-sell-offer) operations.
  - Attribute `offer_id` in `Trade` effect.
  - Attribute `id` in [Offer](https://www.stellar.org/developers/horizon/reference/resources/offer.html) resource.
  - Attribute `timestamp` and `trade_count` in [Trade Aggregation](https://www.stellar.org/developers/horizon/reference/resources/trade_aggregation.html) resource.

## v0.21.1

* Fixes a bug in initial schema migration file.

## v0.21.0

### Database migration notes

This version adds a new index on a table used by experimental ingestion system. If it has not been enabled, migration will be instant. If you migrate from a previous version with experimental ingestion system enabled database migration can take a couple minutes.

### Changes

* `/paths/strict-send` can now accept a `destination_account` parameter. If `destination_account` is provided then the endpoint will return all payment paths which terminate with an asset held by `destination_account`. Note that the endpoint will accept `destination_account` or `destination_assets` but not both. `destination_assets` is a comma separated list of assets encoded as `native` or `code:issuer`.
* `/paths/strict-receive` can now accept a `source_assets` parameter instead of `source_account` parameter. If `source_assets` is provided the endpoint will return all payment paths originating from an asset in `source_assets`. Note that the endpoint will accept `source_account` or `source_assets` but not both. `source_assets` is a comma separated list of assets encoded as `native` or `code:issuer`.
* Add experimental support for `/offers`. To enable it, set `--enable-experimental-ingestion` CLI param or `ENABLE_EXPERIMENTAL_INGESTION=true` env variable.
* When experimental ingestion is enabled a state verification routine is started every 64 ledgers to ensure a local state is the same as in history buckets. This can be disabled by setting `--ingest-disable-state-verification` CLI param or `INGEST-DISABLE-STATE-VERIFICATION` env variable.
* Add flag to apply pending migrations before running horizon. If there are pending migrations, previously you needed to run `horizon db migrate up` before running `horizon`. Those two steps can be combined into one with the `--apply-migrations` flag (`APPLY_MIGRATIONS` env variable).
* Improved the speed of state ingestion in experimental ingestion system.
* Fixed a bug in "Signers for Account" (experimental) transaction meta ingesting code.
* Fixed performance issue in Effects related endpoints.
* Fixed DoS vector in Go HTTP/2 implementation.
* Dropped support for Go 1.10, 1.11.

Check [Beta Testing New Ingestion System](https://github.com/stellar/go/blob/master/services/horizon/internal/expingest/BETA_TESTING.md) if you want to test new ingestion system.

## v0.20.1

* Add `--ingest-state-reader-temp-set` flag (`INGEST_STATE_READER_TEMP_SET` env variable) which defines the storage type used for temporary objects during state ingestion in the new ingestion system. The possible options are: `memory` (requires ~1.5GB RAM, fast) and `postgres` (stores data in temporary table in Postgres, less RAM but slower).

Check [Beta Testing New Ingestion System](https://github.com/stellar/go/blob/master/services/horizon/internal/expingest/BETA_TESTING.md) if you want to test new ingestion system.

## v0.20.0

If you want to use experimental ingestion skip this version and use v0.20.1 instead. v0.20.0 has a performance issue.

### Changes

* Experimental ingestion system is now run concurrently on all Horizon servers (with feature flag set - see below). This improves ingestion availability.
* Add experimental path finding endpoints which use an in memory order book for improved performance. To enable the endpoints set `--enable-experimental-ingestion` CLI param or `ENABLE_EXPERIMENTAL_INGESTION=true` env variable. Note that the `enable-experimental-ingestion` flag enables both the new path finding endpoints and the accounts for signer endpoint. There are two path finding endpoints. `/paths/strict-send` returns payment paths where both the source and destination assets are fixed. This endpoint is able to answer questions like: "Get me the most EUR possible for my 10 USD." `/paths/strict-receive` is the other endpoint which is an alias to the existing `/paths` endpoint.
* `--enable-accounts-for-signer` CLI param or `ENABLE_ACCOUNTS_FOR_SIGNER=true` env variable are merged with `--enable-experimental-ingestion` CLI param or `ENABLE_EXPERIMENTAL_INGESTION=true` env variable.
* Add experimental get offers by id endpoint`/offers/{id}` which uses the new ingestion system to fill up the offers table. To enable it, set `--enable-experimental-ingestion` CLI param or `ENABLE_EXPERIMENTAL_INGESTION=true` env variable.

Check [Beta Testing New Ingestion System](https://github.com/stellar/go/blob/master/services/horizon/internal/expingest/BETA_TESTING.md) if you want to test new ingestion system.

### Scheduled Breaking Changes

* `fee_paid` field on Transaction resource has been deprecated and will be removed in 0.22.0. Please use new fields added in 0.18.0: `max_fee` that defines the maximum fee the source account is willing to pay and `fee_charged` that defines the fee that was actually paid for a transaction. See [CAP-0005](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0005.md) for more information. This change has been previously scheduled for 0.19.0 release.
* The following operation type names have been deprecated: `manage_offer` and `create_passive_offer`. The names will be changed to: `manage_sell_offer` and `create_passive_offer` in 0.22.0. This has been previously scheduled for 0.19.0 release.
* The type for the following attributes will be changed from `int64` to `string` in 0.22.0:
  - Attribute `offer_id` in [manage buy offer](https://www.stellar.org/developers/horizon/reference/resources/operation.html#manage-buy-offer) and [manage sell offer](https://www.stellar.org/developers/horizon/reference/resources/operation.html#manage-sell-offer) operations.
  - Attribute `offer_id` in `Trade` effect.
  - Attribute `id` in [Offer](https://www.stellar.org/developers/horizon/reference/resources/offer.html) resource.
  - Attribute `timestamp` and `trade_count` in [Trade Aggregation](https://www.stellar.org/developers/horizon/reference/resources/trade_aggregation.html) resource.

If you are an SDK maintainer, update your code to prepare for this change.

## v0.19.0

* Add `join` parameter to operations and payments endpoints. Currently, the only valid value for the parameter is `transactions`. If `join=transactions` is included in a request then the response will include a `transaction` field for each operation in the response.
* Add experimental "Accounts For Signers" endpoint. To enable it set `--enable-accounts-for-signer` CLI param or `ENABLE_ACCOUNTS_FOR_SIGNER=true` env variable. Additionally new feature requires links to history archive: CLI: `--history-archive-urls="archive1,archive2,archive3"`, env variable: `HISTORY_ARCHIVE_URLS="archive1,archive2,archive3"`. This will expose `/accounts` endpoint. This requires around 4GB of RAM for initial state ingestion.

Check [Beta Testing New Ingestion System](https://github.com/stellar/go/blob/master/services/horizon/internal/expingest/BETA_TESTING.md) if you want to test new ingestion system.

## v0.18.1

* Fixed `/fee_stats` to correctly calculate ledger capacity in protocol v11.
* Fixed `horizon db clean` command to truncate all tables.

## v0.18.0

### Breaking changes

* Horizon requires Postgres 9.5+.
* Removed `paging_token` field from `/accounts/{id}` endpoint.
* Removed `/operation_fee_stats` endpoint. Please use `/fee_stats`.

### Deprecations

* `fee_paid` field on Transaction resource has been deprecated and will be removed in 0.19.0. Two new fields have been added: `max_fee` that defines the maximum fee the source account is willing to pay and `fee_charged` that defines the fee that was actually paid for a transaction. See [CAP-0005](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0005.md) for more information.
* The following operation type names have been deprecated: `manage_offer` and `create_passive_offer`. The names will be changed to: `manage_sell_offer` and `create_passive_offer` in 0.19.0.

### Changes

* The following new config parameters were added. When old `max-db-connections` config parameter is set, it has a priority over the the new params. Run `horizon help` for more information.
  * `horizon-db-max-open-connections`,
  * `horizon-db-max-idle-connections`,
  * `core-db-max-open-connections`,
  * `core-db-max-idle-connections`.
* Fixed `fee_paid` value in Transaction resource (#1358).
* Fix "int64: value out of range" errors in trade aggregations (#1319).
* Improved `horizon db reingest range` command.

## v0.17.6 - 2019-04-29

* Fixed a bug in `/order_book` when sum of amounts at a single price level exceeds `int64_max` (#1037).
* Fixed a bug generating `ERROR` level log entries for bad requests (#1186).

## v0.17.5 - 2019-04-24

* Support for stellar-core [v11.0.0](https://github.com/stellar/stellar-core/releases/tag/v11.0.0).
* Display trustline authorization state in the balances list.
* Improved actions code.
* Improved `horizon db reingest` command handling code.
* Tracking app name and version that connects to Horizon (`X-App-Name`, `X-App-Version`).

## v0.17.4 - 2019-03-14

* Support for Stellar-Core 10.3.0 (new database schema v9).
* Fix a bug in `horizon db reingest` command (no log output).
* Multiple code improvements.

## v0.17.3 - 2019-03-01

* Fix a bug in `txsub` package that caused returning invalid status when resubmitting old transactions (#969).

## v0.17.2 - 2019-02-28

* Critical fix bug

## v0.17.1 - 2019-02-28

### Changes

* Fixes high severity error in ingestion system.
* Account detail endpoint (`/accounts/{id}`) includes `last_modified_ledger` field for account and for each non-native asset balance.

## v0.17.0 - 2019-02-26

### Upgrade notes

This release introduces ingestion of failed transactions. This feature is turned off by default. To turn it on set environment variable: `INGEST_FAILED_TRANSACTIONS=true` or CLI param: `--ingest-failed-transactions=true`. Please note that ingesting failed transactions can double DB space requirements (especially important for full history deployments).

### Database migration notes

Previous versions work fine with new schema so you can migrate (`horizon db migrate up` using new binary) database without stopping the Horizon process. To reingest ledgers run `horizon db reingest` using Horizon 0.17.0 binary. You can take advantage of the new `horizon db reingest range` for parallel reingestion.

### Deprecations

* `/operation_fee_stats` is deprecated in favour of `/fee_stats`. Will be removed in v0.18.0.

### Breaking changes

* Fields removed in this version:
  * Root > `protocol_version`, use `current_protocol_version` and `core_supported_protocol_version`.
  * Ledger > `transaction_count`, use `successful_transaction_count` and `failed_transaction_count`.
  * Signer > `public_key`, use `key`.
* This Horizon version no longer supports Core <10.0.0. Horizon can still ingest version <10 ledgers.
* Error event name during streaming changed to `error` to follow W3C specification.

### Changes

* Added ingestion of failed transactions (see Upgrade notes). Use `include_failed=true` GET parameter to display failed transactions and operations in collection endpoints.
* `/fee_stats` endpoint has been extended with fee percentiles and ledger capacity usage. Both are useful in transaction fee estimations.
* Fixed a bug causing slice bounds out of range at `/account/{id}/offers` endpoint during streaming.
* Added `horizon db reingest range X Y` that reingests ledgers between X and Y sequence number (closed intervals).
* Many code improvements.

## v0.16.0 - 2019-02-04

### Upgrade notes

* Ledger > Admins need to reingest old ledgers because we introduced `successful_transaction_count` and `failed_transaction_count`.

### Database migration notes

Previous versions work fine with Horizon 0.16.0 schema so you can migrate (`horizon db migrate up`) database without stopping the Horizon process. To reingest ledgers run `horizon db reingest` using Horizon 0.16.0 binary.

### Deprecations

* Root > `protocol_version` will be removed in v0.17.0. It is replaced by `current_protocol_version` and `core_supported_protocol_version`.
* Ledger > `transaction_count` will be removed in v0.17.0.
* Signer > `public_key` will be removed in v0.17.0.

### Changes

* Improved `horizon db migrate` script. It will now either success or show a detailed message regarding why it failed.
* Fixed effects ingestion of circular payments.
* Improved account query performances for payments and operations.
* Added `successful_transaction_count` and `failed_transaction_count` to `ledger` resource.
* Fixed the wrong protocol version displayed in `root` resource by adding `current_protocol_version` and `core_supported_protocol_version`.
* Improved streaming for single objects. It won't send an event back if the current event is the same as the last event sent.
* Fixed ingesting effects of empty trades. Empty trades will be ignored during ingestion.

## v0.15.4 - 2019-01-17

* Fixed multiple issues in transaction submission subsystem.
* Support for client fingerprint headers.
* Fixed parameter checking in `horizon db backfill` command.

## v0.15.3 - 2019-01-07

* Fixed a bug in Horizon DB reaping code.
* Fixed query checking code that generated `ERROR`-level log entries for invalid input.

## v0.15.2 - 2018-12-13

* Added `horizon db init-asset-stats` command to initialize `asset_stats` table. This command should be run once before starting ingestion if asset stats are enabled (`ENABLE_ASSET_STATS=true`).
* Fixed `asset_stats` table to support longer `home_domain`s.
* Fixed slow trades DB query.

## v0.15.1 - 2018-11-09

* Fixed memory leak in SSE stream code.

## v0.15.0 - 2018-11-06

DB migrations add a new fields and indexes on `history_trades` table. This is a very large table in `CATCHUP_COMPLETE` deployments so migration may take a long time (depending on your DB hardware). Please test the migrations execution time on the copy of your production DB first.

This release contains several bug fixes and improvements:

* New `/operation_fee_stats` endpoint includes fee stats for the last 5 ledgers.
* ["Trades"](https://www.stellar.org/developers/horizon/reference/endpoints/trades.html) endpoint can now be streamed.
* In ["Trade Aggregations"](https://www.stellar.org/developers/horizon/reference/endpoints/trade_aggregations.html) endpoint, `offset` parameter has been added.
* Path finding bugs have been fixed and the algorithm has been improved. Check [#719](https://github.com/stellar/go/pull/719) for more information.
* Connections (including streams) are closed after timeout defined using `--connection-timeout` CLI param or `CONNECTION_TIMEOUT` environment variable. If Horizon is behind a load balancer with idle timeout set, it is recommended to set this to a value equal a few seconds less than idle timeout so streams can be properly closed by Horizon.
* Streams have been improved to check for updates every `--sse-update-frequency` CLI param or `SSE_UPDATE_FREQUENCY` environment variable seconds. If a new ledger has been closed in this period, new events will be sent to a stream. Previously streams checked for new events every 1 second, even when there were no new ledgers.
* Rate limiting algorithm has been changed to [GCRA](https://brandur.org/rate-limiting#gcra).
* Rate limiting in streams has been changed to be more fair. Now 1 *credit* has to be *paid* every time there's a new ledger instead of per request.
* Rate limiting can be disabled completely by setting `--per-hour-rate-limit=0` CLI param or `PER_HOUR_RATE_LIMIT=0` environment variable.
* Account flags now display `auth_immutable` value.
* Logs can be sent to a file. Destination file can be set using an environment variable (`LOG_FILE={file}`) or CLI parameter (`--log-file={file}`).

### Breaking changes

* Assets stats are disabled by default. This can be changed using an environment variable (`ENABLE_ASSET_STATS=true`) or CLI parameter (`--enable-asset-stats=true`). Please note that it has a negative impact on a DB and ingestion time.
* In ["Offers for Account"](https://www.stellar.org/developers/horizon/reference/endpoints/offers-for-account.html), `last_modified_time` field  endpoint can be `null` when ledger data is not available (has not been ingested yet).
* ["Trades for Offer"](https://www.stellar.org/developers/horizon/reference/endpoints/trades-for-offer.html) endpoint will query for trades that match the given offer on either side of trades, rather than just the "sell" offer. Offer IDs are now [synthetic](https://www.stellar.org/developers/horizon/reference/resources/trade.html#synthetic-offer-ids). You have to reingest history to update offer IDs.

### Other bug fixes

* `horizon db backfill` command has been fixed.
* Fixed `remoteAddrIP` function to support IPv6.
* Fixed `route` field in the logs when the request is rate limited.

## v0.14.2 - 2018-09-27

### Bug fixes

* Fixed and improved `txsub` package (#695). This should resolve many issues connected to `Timeout` responses.
* Improve stream error reporting (#680).
* Checking `ingest.Cursor` errors in `Session` (#679).
* Added account ID validation in `/account/{id}` endpoints (#684).

## v0.14.1 - 2018-09-19

This release contains several bug fixes:

* Assets stats can cause high CPU usage on stellar-core DB. If this slows down the database it's now possible to turn off this feature by setting `DISABLE_ASSET_STATS` feature flag. This can be set as environment variable (`DISABLE_ASSET_STATS=true`) or CLI parameter (`--disable-asset-stats=true`).
* Sometimes `/accounts/{id}/offers` returns `500 Internal Server Error` response when ledger data is not available yet (for new ledgers) or no longer available (`CATCHUP_RECENT` deployments). It's possible to set `ALLOW_EMPTY_LEDGER_DATA_RESPONSES` feature flag as environment variable (`ALLOW_EMPTY_LEDGER_DATA_RESPONSES=true`) or CLI parameter (`--allow-empty-ledger-data-responses=true`). With the flag set to `true` "Offers for Account" endpoint will return `null` in `last_modified_time` field when ledger data is not available, instead of `500 Internal Server Error` error.

### Bug fixes

* Feature flag to disable asset stats (#668).
* Feature flag to allow null ledger data in responses (#672).
* Fix empty memo field in JSON when memo_type is text (#635).
* Improved logging: some bad requests no longer generate `ERROR` level log entries (#654).
* `/friendbot` endpoint is available only when `FriendbotURL` is set in the config.

## v0.14.0 - 2018-09-06

### Breaking changes

* Offer resource `last_modified` field removed (see Bug Fixes section).
* Trade aggregations endpoint accepts only specific time ranges now (1/5/15 minutes, 1 hour, 1 day, 1 week).
* Horizon sends `Cache-Control: no-cache, no-store, max-age=0` HTTP header for all responses.

### Deprecations

* Account > Signers collection `public_key` field is deprecated, replaced by `key`.

### Changes

* Protocol V10 features:
  * New `bump_sequence` operation (as in [CAP-0001](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0001.md)).
    * New [`bump_sequence`](https://www.stellar.org/developers/horizon/reference/resources/operation.html#bump-sequence) operation.
    * New `sequence_bumped` effect.
    * Please check [CAP-0001](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0001.md) for new error codes for transaction submission.
  * Offer liabilities (as in [CAP-0003](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0003.md)):
    * `/accounts/{id}` resources contain new fields: `buying_liabilities` and `selling_liabilities` for each entry in `balances`.
    * Please check [CAP-0003](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0003.md) for new error codes for transaction submission.
* Added `source_amount` field to `path_payment` operations.
* Added `account_credited` and `account_debited` effects for `path_payment` operations.
* Friendbot link in Root endpoint is empty if not set in configuration.
* Improved `ingest` package logging.
* Improved HTTP logging (`forwarded_ip`, `route` fields, `duration` is always in seconds).
* `LOGGLY_HOST` env variable has been replaced with `LOGGLY_TAG` and is adding a tag to every log event.
* Dropped support for Go 1.8.

### Bug fixes

* New fields in Offer resource: `last_modified_ledger` and `last_modified_time`, replace buggy `last_modified` (#478).
* Fixed pagination in Trades for account endpoint (#486).
* Fixed a synchronization issue in `ingest` package (#603).
* Fixed Order Book resource links in Root endpoint.
* Fixed streaming in Offers for Account endpoint.

## v0.13.3 - 2018-08-23

### Bug fixes

* Fixed large amounts rendering in `/assets`.

## v0.13.2 - 2018-08-13

### Bug fixes

* Fixed a bug in `amount` and `price` packages triggering long calculations.

## v0.13.1 - 2018-07-26

### Bug fixes

* Fixed a conversion bug when `timebounds.max_time` is set to `INT64_MAX`.

## v0.13.0 - 2018-06-06

### Breaking changes

- `amount` field in `/assets` is now a String (to support Stellar amounts larger than `int64`).

### Changes

- Effect resource contains a new `created_at` field.
- Horizon responses are compressed.
- Ingestion errors have been improved.
- `horizon rebase` command was improved.

### Bug fixes

- Horizon now returns `400 Bad Request` for negative `cursor` values.

**Upgrade notes**

DB migrations add a new indexes on `history_trades`. This is very large table so migration may take a long time (depending on your DB hardware). Please test the migrations execution time on the copy of your production DB first.

## v0.12.3 - 2017-03-20

### Bug fixes

- Fix a service stutter caused by excessive `info` commands being issued from the root endpoint.


## v0.12.2 - 2017-03-14

This release is a bug fix release for v0.12.1 and v0.12.2.  *Please see the upgrade notes below if you did not already migrate your db for v0.12.0*

### Changes

- Remove strict validation on the `resolution` parameter for trade aggregations endpoint.  We will add this feature back in to the next major release.


## v0.12.1 - 2017-03-13

This release is a bug fix release for v0.12.0.  *Please see the upgrade notes below if you did not already migrate your db for v0.12.0*

### Bug fixes

- Fixed an issue caused by un-migrated trade rows. (https://github.com/stellar/go/issues/357)
- Command line flags are now useable for subcommands of horizon.


## v0.12.0 - 2017-03-08

Big release this time for horizon:  We've made a number of breaking changes since v0.11.0 and have revised both our database schema as well as our data ingestion system.  We recommend that you take a backup of your horizon database prior to upgrading, just in case.

### Upgrade Notes

Since this release changes both the schema and the data ingestion system, we recommend the following upgrade path to minimize downtime:

1. Upgrade horizon binaries, but do not restart the service
2. Run `horizon db migrate up` to migrate the db schema
3. Run `horizon db reingest` in a background session to begin the data reingestion process
4. Restart horizon

### Added

- Operation and payment resources were changed to add `transaction_hash` and `created_at` properties.
- The ledger resource was changed to add a `header_xdr` property.  Existing horizon installations should re-ingest all ledgers to populate the history database tables with the data.  In future versions of horizon we will disallow null values in this column.  Going forward, this change reduces the coupling of horizon to stellar-core, ensuring that horizon can re-import history even when the data is no longer stored within stellar-core's database.
- All Assets endpoint (`/assets`) that returns a list of all the assets in the system along with some stats per asset. The filters allow you to narrow down to any specific asset of interest.
- Trade Aggregations endpoint (`/trade_aggregations`) allow for efficient gathering of historical trade data. This is done by dividing a given time range into segments and aggregate statistics, for a given asset pair (`base`, `counter`) over each of these segments.

### Bug fixes

- Ingestion performance and stability has been improved.
- Changes to an account's inflation destination no longer produce erroneous "signer_updated" effects. (https://github.com/stellar/horizon/issues/390)


### Changed

- BREAKING CHANGE: The `base_fee` property of the ledger resource has been renamed to `base_fee_in_stroops`
- BREAKING CHANGE: The `base_reserve` property of the ledger resource has been renamed to `base_reserve_in_stroops` and is now expressed in stroops (rather than lumens) and as a JSON number.
- BREAKING CHANGE: The "Orderbook Trades" (`/orderbook/trades`) endpoint has been removed and replaced by the "All Trades" (`/trades`) endpoint.
- BREAKING CHANGE: The Trade resource has been modified to generalize assets as (`base`, `counter`) pairs, rather than the previous (`sold`,`bought`) pairs.
- Full reingestion (i.e. running `horizon db reingest`) now runs in reverse chronological order.

### Removed

- BREAKING CHANGE: Friendbot has been extracted to an external microservice.


## [v0.11.0] - 2017-08-15

### Bug fixes

- The ingestion system can now properly import envelopes that contain signatures that are zero-length strings.
- BREAKING CHANGE: specifying a `limit` of `0` now triggers an error instead of interpreting the value to mean "use the default limit".
- Requests that ask for more records than the maximum page size now trigger a bad request error, instead of an internal server error.
- Upstream bug fixes to xdr decoding from `github.com/stellar/go`.

### Changed

- BREAKING CHANGE: The payments endpoint now includes `account_merge` operations in the response.
- "Finished Request" log lines now include additional fields: `streaming`, `path`, `ip`, and `host`.
- Responses now include a `Content-Disposition: inline` header.


## [v0.10.1] - 2017-03-29

### Fixed
- Ingestion was fixed to protect against text memos that contain null bytes.  While memos with null bytes are legal in stellar-core, PostgreSQL does not support such values in string columns.  Horizon now strips those null bytes to fix the issue.

## [v0.10.0] - 2017-03-20

This is a fix release for v0.9.0 and v0.9.1


### Added
- Added `horizon db clear` helper command to clear previously ingested history.

### Fixed

- Embedded sql files for the database schema have been fixed agsain to be compatible with postgres 9.5. The configuration setting `row_security` has been removed from the dumped files.

## [v0.9.1] - 2017-03-20

### Fixed

- Embedded sql files for the database schema have been fixed to be compatible with postgres 9.5. The configuration setting `idle_in_transaction_session_timeout` has been removed from the dumped files.

## [v0.9.0] - 2017-03-20

This release was retracted due to a bug discovered after release.

### Added
- Horizon now exposes the stellar network protocol in several places:  It shows the currently reported protocol version (as returned by the stellar-core `info` command) on the root endpoint, and it reports the protocol version of each ledger resource.
- Trade resources now include a `created_at` timestamp.

### Fixed

- BREAKING CHANGE: The reingestion process has been updated.  Prior versions of horizon would enter a failed state when a gap between the imported history and the stellar-core database formed or when a previously imported ledger was no longer found in the stellar-core database.  This usually occurs when running stellar-core with the `CATCHUP_RECENT` config option.  With these changed, horizon will automatically trim "abandonded" ledgers: ledgers that are older than the core elder ledger.


## [v0.8.0] - 2017-02-07

### Added

- account signer resources now contain a type specifying the type of the signer: `ed25519_public_key`, `sha256_hash`, and `preauth_tx` are the present values used for the respective signer types.

### Changed

- The `public_key` field on signer effects and an account's signer summary has been renamed to `key` to reflect that new signer types are not necessarily specifying a public key anymore.

### Deprecated

- The `public_key` field on account signers and signer effects are deprecated

## [v0.7.1] - 2017-01-12

### Bug fixes

- Trade resources now include "bought_amount" and "sold_amount" fields when being viewed through the "Orderbook Trades" endpoint.
- Fixes #322: orderbook summaries with over 20 bids now return the correct price levels, starting with the closest to the spread.

## [v0.7.0] - 2017-01-10

### Added

- The account resource now includes links to the account's trades and data values.

### Bug fixes

- Fixes paging_token attribute of account resource
- Fixes race conditions in friendbot
- Fixes #202: Add price and price_r to "manage_offer" operation resources
- Fixes #318: order books for the native currency now filters correctly.

## [v0.6.2] - 2016-08-18

### Bug fixes

- Fixes streaming (SSE) requests, which were broken in v0.6.0

## [v0.6.1] - 2016-07-26

### Bug fixes

- Fixed an issue where accounts were not being properly returned when the  history database had no record of the account.


## [v0.6.0] - 2016-07-20

This release contains the initial implementation of the "Abridged History System".  It allows a horizon system to be operated without complete knowledge of the ledger's history.  With this release, horizon will start ingesting data from the earliest point known to the connected stellar-core instance, rather than ledger 1 as it behaved previously.  See the admin guide section titled "Ingesting stellar-core data" for more details.

### Added

- *Elder* ledgers have been introduced:  An elder ledger is the oldest ledger known to a db.  For example, the `core_elder_ledger` attribute on the root endpoint refers to the oldest known ledger stored in the connected stellar-core database.
- Added the `history-retention-count` command line flag, used to specify the amount of historical data to keep in the history db.  This is expressed as a number of ledgers, for example a value of `362880` would retain roughly 6 weeks of data given an average of 10 seconds per ledger.
- Added the `history-stale-threshold` command line flag to enable stale history protection.  See the admin guide for more info.
- Horizon now reports the last ledger ingested to stellar-core using the `setcursor` command.
- Requests for data that precede the recorded window of history stored by horizon will receive a `410 Gone` http response to allow software to differentiate from other "not found" situations.
- The new `db reap` command will manually trigger the deletion of unretained historical data
- A background process on the server now deletes unretained historical once per hour.

### Changed

- BREAKING: When making a streaming request, a normal error response will be returned if an error occurs prior to sending the first event.  Additionally, the initial http response and streaming preamble will not be sent until the first event is available.
- BREAKING: `horizon_latest_ledger` has renamed to `history_latest_ledger`
- Horizon no longer needs to begin the ingestion of historical data from ledger sequence 1.
- Rows in the `history_accounts` table are no longer identified using the "Total Order ID" that other historical records  use, but are rather using a simple auto-incremented id.

### Removed

- The `/accounts` endpoint, which lets a consumer page through the entire set of accounts in the ledger, has been removed.  The change from complete to an abridged history in horizon makes the endpoint mostly useless, and after consulting with the community we have decided to remove the endpoint.

## [v0.5.1] - 2016-04-28

### Added

  - ManageData operation data is now rendered in the various operation end points.

### Bug fixes

- Transaction memos that contain utf-8 are now properly rendered in browsers by properly setting the charset of the http response.

## [v0.5.0] - 2016-04-22

### Added

- BREAKING: Horizon can now import data from stellar-core without the aid of the horizon-importer project.  This process is now known as "ingestion", and is enabled by either setting the `INGEST` environment variable to "true" or specifying "--ingest" on the launch arguments for the horizon process.  Only one process should be running in this mode for any given horizon database.
- Add `horizon db init`, used to install the latest bundled schema for the horizon database.
- Add `horizon db reingest` command, used to update outdated or corrupt horizon database information.  Admins may now use `horizon db reingest outdated` to migrate any old data when updated horizon.
- Added `network_passphrase` field to root resource.
- Added `fee_meta_xdr` field to transaction resource.

### Bug fixes
- Corrected casing on the "offers" link of an account resource.

## [v0.4.0] - 2016-02-19

### Added

- Add `horizon db migrate [up|down|redo]` commands, used for installing schema migrations.  This work is in service of porting the horizon-importer project directly to horizon.
- Add support for TLS: specify `--tls-cert` and `--tls-key` to enable.
- Add support for HTTP/2.  To enable, use TLS.

### Removed

- BREAKING CHANGE: Removed support for building on go versions lower than 1.6

## [v0.3.0] - 2016-01-29

### Changes

- Fixed incorrect `source_amount` attribute on pathfinding responses.
- BREAKING CHANGE: Sequence numbers are now encoded as strings in JSON responses.
- Fixed broken link in the successful response to a posted transaction

## [v0.2.0] - 2015-12-01
### Changes

- BREAKING CHANGE: the `address` field of a signer in the account resource has been renamed to `public_key`.
- BREAKING CHANGE: the `address` on the account resource has been renamed to `account_id`.

## [v0.1.1] - 2015-12-01

### Added
- Github releases are created from tagged travis builds automatically

[v0.11.0]: https://github.com/stellar/horizon/compare/v0.10.1...v0.11.0
[v0.10.1]: https://github.com/stellar/horizon/compare/v0.10.0...v0.10.1
[v0.10.0]: https://github.com/stellar/horizon/compare/v0.9.1...v0.10.0
[v0.9.1]: https://github.com/stellar/horizon/compare/v0.9.0...v0.9.1
[v0.9.0]: https://github.com/stellar/horizon/compare/v0.8.0...v0.9.0
[v0.8.0]: https://github.com/stellar/horizon/compare/v0.7.1...v0.8.0
[v0.7.1]: https://github.com/stellar/horizon/compare/v0.7.0...v0.7.1
[v0.7.0]: https://github.com/stellar/horizon/compare/v0.6.2...v0.7.0
[v0.6.2]: https://github.com/stellar/horizon/compare/v0.6.1...v0.6.2
[v0.6.1]: https://github.com/stellar/horizon/compare/v0.6.0...v0.6.1
[v0.6.0]: https://github.com/stellar/horizon/compare/v0.5.1...v0.6.0
[v0.5.1]: https://github.com/stellar/horizon/compare/v0.5.0...v0.5.1
[v0.5.0]: https://github.com/stellar/horizon/compare/v0.4.0...v0.5.0
[v0.4.0]: https://github.com/stellar/horizon/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/stellar/horizon/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/stellar/horizon/compare/v0.1.1...v0.2.0
[v0.1.1]: https://github.com/stellar/horizon/compare/v0.1.0...v0.1.1
