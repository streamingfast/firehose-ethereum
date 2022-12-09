# Change log

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). See [MAINTAINERS.md](./MAINTAINERS.md)
for instructions to keep up to date.

## Unreleased

### Added

* New Prometheus metric `console_reader_trx_read_count` can be used to obtain a transaction rate of how many transactions were read from the node over a period of time.

* New Prometheus metric `console_reader_block_read_count` can be used to obtain a block rate of how many blocks were read from the node over a period of time.

* Added `--header-only` support on `fireeth tools firehose-client`.

* Added `HeaderOnly` transform that can be used to return only the Block's header a few top-level fields `Ver`, `Hash`, `Number` and `Size`.

* Added `fireeth tools firehose-prometheus-exporter` to use as a client-side monitoring tool of a Firehose endpoint.

### Deprecated

* **Deprecated** `LightBlock` is deprecated and will be removed in the next major version, it's goal is now much better handled by `CombineFilter` transform or `HeaderOnly` transform if you required only Block's header.

## v1.2.2

* Hotfix 'nil pointer' panic when saving uninitialized cache.

## v1.2.1

### Substreams improvements

#### Performance

* Changed cache file format for stores and outputs (faster with vtproto) -- requires removing the existing state files.
* Various improvements to scheduling.

#### Fixes

* Fixed `eth_call` handler not flagging `out of gas` error as deterministic.
* Fixed Memory leak in wasmtime.

### Merger fixes

* Removed the unused 'previous' one-block in merged-blocks (99 inside bundle:100).
* Fix: also prevent rare bug of bundling "very old" one-blocks in merged-blocks.

## v1.2.0

### Added

* Added `sf.firehose.v2.Fetch/Block` endpoint on firehose, allows fetching single block by num, num+ID or cursor.
* Added `tools firehose-single-block-client` to call that new endpoint.

### Changed

* Renamed tools `normalize-merged-blocks` to `upgrade-merged-blocks`.

### Fixed

* Fixed `common-blocks-cache-dir` flag's description.
* Fixed `DELEGATECALL`'s `caller` (a.k.a `from`). -> requires upgrade of blocks to `version: 3`
* Fixed `execution aborted (timeout = 5s)` hard-coded timeout value when detecting in Substreams if `eth_call` error response was deterministic.

### Upgrade Procedure

Assuming that you are running a firehose deployment v1.1.0 writing blocks to folders `/v2-oneblock`, `/v2-forked` and `/v2`,
you will deploy a new setup that writes blocks to folders `/v3-oneblock`, `v3-forked` and `/v3`

This procedure describes an upgrade without any downtime. With proper parallelization, it should be possible to complete this upgrade within a single day.

1. Launch a new reader with this code, running instrumented geth binary: https://github.com/streamingfast/go-ethereum/releases/tag/geth-v1.10.25-fh2.1
   (you can start from a backup that is close to head)
2. Upgrade your merged-blocks from `version: 2` to `version: 3` using `fireeth tools upgrade-merged-blocks /path/to/v2 /path/to/v3 {start} {stop}`
   (you can run multiple upgrade commands in parallel to cover the whole blocks range)
3. Create combined indexes from those new blocks with `fireeth start combined-index-builder`
   (you can run multiple commands in parallel to fill the block range)
4. When your merged-blocks have been upgraded and the one-block-files are being produced by the new reader, launch a merger
5. When the reader, merger and combined-index-builder caught up to live, you can launch the relayer(s), firehose(s)
6. When the firehoses are ready, you can now switch traffic to them.

## v1.1.0

### Added

* Added 'SendAllBlockHeaders' param to CombinedFilter transform when we want to prevent skipping blocks but still want to filter out trxs.

### Changed

* Reduced how many times `reader read statistics` is displayed down to each 30s (previously each 5s) (and re-wrote log to `reader node statistics`).

### Fixed

* Fix `fireeth tools download-blocks-from-firehose` tool that was not working anymore.
* Simplify `forkablehub` startup performance cases.
* Fix relayer detection of a hole in stream blocks (restart on unrecoverable issue).
* Fix possible panic in hub when calls to one-block store are timing out.
* Fix merger slow one-block-file deletions when there are more than 10000 of them.

## v1.0.0

### BREAKING CHANGES

#### Project rename

* The binary name has changed from `sfeth` to `fireeth` (aligned with https://firehose.streamingfast.io/references/naming-conventions)
* The repo name has changed from `sf-ethereum` to `firehose-ethereum`

#### Ethereum V2 blocks (with fh2-instrumented nodes)

* **This will require reprocessing the chain to produce new blocks**
* Protobuf Block model is now tagged `sf.ethereum.type.v2` and contains the following improvements:
  * Fixed Gas Price on dynamic transactions (post-London-fork on ethereum mainnet, EIP-1559)
  * Added "Total Ordering" concept, 'Ordinal' field on all events within a block (trx begin/end, call, log, balance change, etc.)
  * Added TotalDifficulty field to ethereum blocks
  * Fixed wrong transaction status for contract deployments that fail due to out of gas on pre-Homestead transactions (aligned with status reported by chain: SUCCESS -- even if no contract code is set)
  * Added more instrumentation around AccessList and DynamicFee transaction, removed some elements that were useless or could not be derived from other elements in the structure, ex: gasEvents
  * Added support for finalized block numbers (moved outside the proto-ethereum block, to firehose bstream v2 block)
* There are *no more "forked blocks"* in the merged-blocks bundles:
  * The merged-blocks are therefore produced only after finality passed (before The Merge, this means after 200 confirmations).
  * One-block-files close to HEAD stay in the one-blocks-store for longer
  * The blocks that do not make it in the merged-blocks (forked out because of a re-org) are uploaded to another store (common-forked-blocks-store-url) and kept there for a while (to allow resolving cursors)

#### Firehose V2 Protocol

* **This will require changes in most firehose clients**
* A compatibility layer has been added to still support `sf.firehose.v1.Stream/Blocks` but only for specific values for 'ForkSteps' in request: 'irreversible' or 'new+undo'
* The Firehose Blocks protocol is now under `sf.firehose.v2` (bumped from `sf.firehose.v1`).
  * Step type `IRREVERSIBLE` renamed to `FINAL`
  * `Blocks` request now only allows 2 modes regarding steps: `NEW,UNDO` and `FINAL` (gated by the `final_blocks_only` boolean flag)
  * Blocks that are sent out can have the combined step `NEW+FINAL` to prevent sending the same blocks over and over if they are already final

#### Block Indexes

* Removed the Irreversible indices completely (because the merged-blocks only contain final blocks now)
* Deprecated the "Call" and "log" indices (`xxxxxxxxxx.yyy.calladdrsig.idx` and `xxxxxxxxxx.yyy.logaddrsig.idx`), now replaced by "combined" index
* Moved out the `sfeth tools generate-...` command to a new app that can be launched with `sfeth start generate-combined-index[,...]`

#### Flags and environment variables

* All config via environment variables that started with `SFETH_` now starts with `FIREETH_`
* All logs now output on *stderr* instead of *stdout* like previously
* Changed `config-file` default from `./sf.yaml` to `""`, preventing failure without this flag.
* Renamed `common-blocks-store-url` to `common-merged-blocks-store-url`
* Renamed `common-oneblock-store-url` to `common-one-block-store-url` *now used by firehose and relayer apps*
* Renamed `common-blockstream-addr` to `common-live-blocks-addr`
* Renamed the `mindreader` application to `reader`
* Renamed all the `mindreader-node-*` flags to `reader-node-*`
* Added `common-forked-blocks-store-url` flag *used by merger and firehose*
* Changed `--log-to-file` default from `true` to `false`
* Changed default verbosity level: now all loggers are `INFO` (instead of having most of them to `WARN`). `-v` will now activate all `DEBUG` logs
* Removed `common-block-index-sizes`, `common-index-store-url`
* Removed `merger-state-file`, `merger-next-exclusive-highest-block-limit`, `merger-max-one-block-operations-batch-size`, `merger-one-block-deletion-threads`, `merger-writers-leeway`
* Added `merger-stop-block`, `merger-prune-forked-blocks-after`, `merger-time-between-store-pruning`
* Removed `mindreader-node-start-block-num`, `mindreader-node-wait-upload-complete-on-shutdown`, `mindreader-node-merge-and-store-directly`, `mindreader-node-merge-threshold-block-age`
* Removed `firehose-block-index-sizes`,`firehose-block-index-sizes`, `firehose-irreversible-blocks-index-bundle-sizes`, `firehose-irreversible-blocks-index-url`, `firehose-realtime-tolerance`
* Removed `relayer-buffer-size`, `relayer-merger-addr`, `relayer-min-start-offset`

### MIGRATION

#### Clients

* If you depend on the proto file, update `import "sf/ethereum/type/v1/type.proto"` to `import "sf/ethereum/type/v2/type.proto"`
* If you depend on the proto file, update all occurrences of `sf.ethereum.type.v1.<Something>` to `sf.ethereum.type.v2.<Something>`
* If you depend on `sf-ethereum/types` as a library, update all occurrences of `github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v1` to `github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2`.

### Server-side

#### Deployment

* The `reader` requires Firehose-instrumented Geth binary with instrumentation version *2.x* (tagged `fh2`)
* Because of the changes in the ethereum block protocol, an existing deployment cannot be migrated in-place.
* You must deploy firehose-ethereum v1.0.0 on a new environment (without any prior block or index data)
* You can put this new deployment behind a GRPC load-balancer that routes `/sf.firehose.v2.Stream/*` and `/sf.firehose.v1.Stream/*` to your different versions.
* Go through the list of changed "Flags and environment variables" and adjust your deployment accordingly.
  * Determine a (shared) location for your `forked-blocks`.
  * Make sure that you set the `one-block-store` and `forked-blocks-store` correctly on all the apps that now require it.
  * Add the `generate-combined-index` app to your new deployment instead of the `tools` command for call/logs indices.
* If you want to reprocess blocks in batches while you set up a "live" deployment:
  * run your reader node from prior data (ex: from a snapshot)
  * use the `--common-first-streamable-block` flag to a 100-block-aligned boundary right after where this snapshot starts (use this flag on all apps)
  * perform batch merged-blocks reprocessing jobs
  * when all the blocks are present, set the `common-first-streamable-block` flag to 0 on your deployment to serve the whole range

#### Producing merged-blocks in batch

* The `reader` requires Firehose-instrumented Geth binary with instrumentation version *2.x* (tagged `fh2`)
* The `reader` *does NOT merge block files directly anymore*: you need to run it alongside a `merger`:
  * determine a `start` and `stop` block for your reprocessing job, aligned on a 100-blocks boundary right after your Geth data snapshot
  * set `--common-first-streamable-block` to your start-block
  * set `--merger-stop-block` to your stop-block
  * set `--common-one-block-store-url` to a local folder accessible to both `merger` and `mindreader` apps
  * set `--common-merged-blocks-store-url` to the final (ex: remote) folder where you will store your merged-blocks
  * run both apps like this `fireeth start reader,merger --...`
* You can run as many batch jobs like this as you like in parallel to produce the merged-blocks, as long as you have data snapshots for Geth that start at this point

#### Producing combined block indices in batch

* Run batch jobs like this: `fireeth start generate-combined-index --common-blocks-store-url=/path/to/blocks --common-index-store-url=/path/to/index --combined-index-builder-index-size=10000 --combined-index-builder-start-block=0 [--combined-index-builder-stop-block=10000] --combined-index-builder-grpc-listen-addr=:9000`

### Other (non-breaking) changes

#### Added tools and apps

* Added `tools firehose-client` command with filter/index options
* Added `tools normalize-merged-blocks` command to remove forked blocks from merged-blocks files (cannot transform ethereum blocks V1 into V2 because some fields are missing in V1)
* Added substreams server support in firehose app (*alpha*) through `--substreams-enabled` flag

#### Various

* The firehose GRPC endpoint now supports requests that are compressed using `gzip` or `zstd`
* The merger does not expose `PreMergedBlocks` endpoint over GRPC anymore, only HealthCheck. (relayer does not need to talk to it)
* Automatically setting the flag `--firehose-genesis-file` on `reader` nodes if their `reader-node-bootstrap-data-url` config value is sets to a `genesis.json` file.
* Note to other Firehose implementors: we changed all command line flags to fit the required/optional format referred to here: https://en.wikipedia.org/wiki/Usage_message
* Added prometheus boolean metric to all apps called 'ready' with label 'app' (firehose, merger, mindreader-node, node, relayer, combined-index-builder)

## v0.10.2

* Removed `firehose-blocks-store-urls` flag (feature for using multiple stores now deprecated -> causes confusion and issues with block-caching), use `common-blocks-sture-url` instead.

## v0.10.2

* Fixed problem using S3 provider where the S3 API returns empty filename (we ignore at the consuming time when we receive an empty filename result).

## v0.10.1

* Fixed an issue where the merger could panic on a new deployment

## v0.10.0

* Fixed an issue where the `merger` would get stuck when too many (more than 2000) one-block-files were lying around, with block numbers below the current bundle high boundary.

## v0.10.0-rc.5

#### Changed

* Renamed common `atm` 4 flags to `blocks-cache`:
  `--common-blocks-cache-{enabled|dir|max-recent-entry-bytes|max-entry-by-age-bytes}`

#### Fixed

* Fixed `tools check merged-blocks` block hole detection behavior on missing ranges (bumped `sf-tools`)
* Fixed a deadlock issue related to s3 storage error handling (bumped `dstore`)

#### Added

* Added `tools download-from-firehose` command to fetch blocks and save them as merged-blocks files locally.
* Added `cloud-gcp://` auth module (bumped `dauth`)

## v0.10.0-rc.4

#### Added

* substreams-alpha client
* gke-pvc-snapshot backup module

#### Fixed
* Fixed a potential 'panic' in `merger` on a new chain

## v0.10.0

#### Fixed
* Fixed an issue where the `merger` would get stuck when too many (more than 2000) one-block-files were lying around, with block numbers below the current bundle high boundary.

## v0.10.0-rc.5

### Changed

* Renamed common `atm` 4 flags to `blocks-cache`:
  `--common-blocks-cache-{enabled|dir|max-recent-entry-bytes|max-entry-by-age-bytes}`

#### Fixed

* Fixed `tools check merged-blocks` block hole detection behavior on missing ranges (bumped `sf-tools`)

#### Added

* Added `tools download-from-firehose` command to fetch blocks and save them as merged-blocks files locally.
* Added `cloud-gcp://` auth module (bumped `dauth`)

## v0.10.0-rc.4

### Changed

* The default text `encoder` use to encode log entries now emits the level when coloring is disabled.
* Default value for flag `--mindreader-node-enforce-peers` is now `""`, this has been changed because the default value was useful only in development when running a local `node-manager` as either the miner or a peering node.

## v0.10.0-rc.1

#### Added

* Added block data file caching (called `ATM`), this is to reduce the memory usage of component keeping block objects in memory.
* Added transforms: LogFilter, MultiLogFilter, CallToFilter, MultiCallToFilter to only return transaction traces that match logs or called addresses.
* Added support for irreversibility indexes in firehose to prevent replaying reorgs when streaming old blocks.
* Added support for log and call indexes to skip old blocks that do not match any transform filter.

### Changed

* Updated all Firehose stack direct dependencies.
* Updated confusing flag behavior for `--common-system-shutdown-signal-delay` and its interaction with `gRPC` connection draining in `firehose` component sometimes preventing it from shutting down.
* Reporting an error is if flag `merge-threshold-block-age` is way too low (< 30s).

#### Removed

* Removed some old components that are not required by Firehose stack directly, the repository is as lean as it ca now.

#### Fixed

* Fixed Firehose gRPC listening address over plain text.
* Fixed automatic merging of files within the `mindreader` is much more robust then before.
