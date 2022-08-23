# Change log

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). See [MAINTAINERS.md](./MAINTAINERS.md)
for instructions to keep up to date.

## v1.0.0-UNRELEASED

### BREAKING CHANGES

* Protobuf Block model is now tagged `sf.ethereum.type.v2`
  * If you depend on the proto file, update `import "sf/ethereum/type/v1/type.proto"` to `import "sf/ethereum/type/v2/type.proto"`
  * If you depend on the proto file, update all occurrences of `sf.ethereum.type.v1.<Something>` to `sf.ethereum.type.v2.<Something>`
  * If you depend on `sf-ethereum/types` as a library, update all occurrences of `github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v1` to `github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v2`.
* Requires Firehose instrumented binary with instrumentation version *2.0* (tagged `fh2`)
* Requires reprocessing *all merged block files* and *block indexes (combined: call/logs)* (no more irreversible-index are needed)
* The Firehose Blocks protocol is now under `sf.firehose.v2` (bumped from `sf.firehose.v1`). Firehose clients must be adapted.
* The ethereum block protocol is now under `sf.ethereum.type.v2` (bumped from `sf.ethereum.type.v1`). Firehose clients must be adapted.

### MIGRATION

* Because of the changes in the ethereum block protocol, an existing deployment cannot be migrated in-place.
* sf-ethereum v1.0.0 must be deployed to a new environment from block 0, under a new URL (or behind a GRPC load-balancer that routes `/sf.firehose.v2.Stream/*` and `/sf.firehose.v1.Stream/*` to your different versions.
* a compatibility layer has been added so that `sf.firehose.v1.Stream` is also exposed, but only for specific values for 'ForkSteps' (either 'irreversible' or 'new+undo')

### DETAILED CHANGES

#### Firehose V2 protocol

 * it now only allows 2 modes of operation for steps: `NEW,UNDO` (default) and `FINAL` (only serving blocks that reached finality, with parameter `final_blocks_only`)
 * more fields have been removed or renamed

#### Ethereum V2 blocks (with fh2-instrumented nodes)

 * Fixed Gas Price on dynamic transactions (post-London-fork on ethereum mainnet, EIP-1559)
 * Added "Total Ordering" concept, 'Ordinal' field on all events within a block (trx begin/end, call, log, balance change, etc.)
 * Added TotalDifficulty field to ethereum blocks
 * Fixed wrong transaction status for contract deployments that fail due to out of gas on pre-Homestead transactions (aligned with status reported by chain: SUCCESS -- even if no contract code is set)
 * Added more instrumentation around AccessList and DynamicFee transaction, removed some elements that were useless or not could be derived from other elements in the structure, ex: gasEvents

#### Final-blocks only in merged-blocks

 * There are no more "forked blocks" in the merged-blocks bundles. The forked blocks stay in the one-blocks-store, along with the reversible segment close to the HEAD, until finality is passed.
 * To allow resolving cursors that point to forked blocks, you must ensure that you keep those one-block-files for a while (--merger-prune-forked-blocks-after=<number-of-blocks>)

#### Changed top-level-flags and behavior

* `sfeth --log-to-file` defaulted to `true` and is now `false`. Be explicit if you want to log to a file.
* `sfeth --config-file` defaulted to `./sf.yaml` and failed if not present, and now defaults to `""` (doesn't fail is nothing is specified)
* Default verbosity is to show all loggers as `INFO` level (previously only loggers whose app's name was `sfeth` were at `INFO` by default). `-v` will now activate `DEBUG` logs.
* All logs now output on *stderr* instead of *stdout* like previously

#### "Combined" call+log indexes as new app 'combined-index-builder'

* Deprecated the "Call" and "log" indexes, now replaced by "combined" index
  * Generate new indices like this: `sfeth start generate-combined-index --common-blocks-store-url=/path/to/blocks --common-index-store-url=/path/to/index --combined-index-builder-index-size=10000 --combined-index-builder-start-block=0 [--combined-index-builder-stop-block=10000] --combined-index-builder-grpc-listen-addr=:9000`
  * Delete previous indices named `xxxxxxxxxx.yyy.calladdrsig.idx` and `xxxxxxxxxx.yyy.logaddrsig.idx`
* There is no more need for an irreversible index, because the merged-blocks only contain final blocks now.

#### Changes to merger and mindreader

* The mindreader *does not automatically merge old blocks* anymore: you need to run block extraction jobs with both mindreader and merger (using `--common-first-streamable-block=x` and `--merger-stop-block=y` to produce merged-blocks files from `x` to `y`)
* The merger does not expose `PreMergedBlocks` endpoint over GRPC anymore, only HealthCheck. (relayer does not need to talk to it)

#### Added tools

* Added firehose client command `sfeth tools firehose-client [--plaintext] [-a NONE] <firehose:endpoint> <start> [stop]` with filter/index options like `--call-filters=0xAddr1+0xAddr2:,0xAddr3:0xMethod1+0xmethod2`

#### Various

* Automatically setting the flag `--firehose-deep-mind-genesis` on `mindreader` nodes if their `mindreader-node-bootstrap-data-url` config value is sets to a `genesis.json` file.
* Note to other Firehose implementors: we changed all command line flags to fit the required/optional format referred to here: https://en.wikipedia.org/wiki/Usage_message
* Removed merger 'state file' (and `merger-state-file` flag) -- merger now finds where it should merge based on `common-first-streamable-block` and existing merged files...
* Removed `merger-next-exclusive-highest-block-limit`, it will now ONLY base its decision based on `common-first-streamable-block`, up to the last found merged block
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
