# Change log

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). See [MAINTAINERS.md](./MAINTAINERS.md)
for instructions to keep up to date.

## Unreleased


#### Added

* Added firehose client command `sfeth tools firehose-client {firehose:endpoint} {start} [stop]` with filter/index options like `--call-filters=0xAddr1+0xAddr2:,0xAddr3:0xMethod1+0xmethod2` 
* `--common-first-streamable-block` to allow partial block production

#### Modified
* merger now exits with an error code if it detects a hole in one-block-file (30 blocks is enough to trigger this behavior) (instead of waiting forever)
* node-manager now exits with an error code if it should be merging but detects a hole between the previous one-block-files and the first block that it receives (you must manually delete the blocks lying around)

#### Removed
* removed merger 'state file' (and `merger-state-file` flag) -- merger now finds where it should merge based on `common-first-streamable-block` and existing merged files...
* removed `merger-next-exclusive-highest-block-limit`, it will now ONLY base its decision based on `common-first-streamable-block`, up to the last found merged block

#### BREAKING CHANGES --requires reprocessing all merged block files and block indexes--

* Requires Firehose instrumented binary with instrumentation version *2.0* (tagged `fh2`)

* Produced / consumed block Protobuf payload version bumped 1 -> 2
  * Fixed Gas Price on dynamic transactions (post-London-fork on ethereum mainnet)
  * Added "Total Ordering" concept, 'Ordinal' field on all events within a block (trx begin/end, call, log, balance change, etc.)
  * Added TotalDifficulty field to ethereum blocks

* Changed default values for two top-level flags

  * `sfeth --log-to-file` defaulted to `true` and is now `false`. Be explicit if you want to log to a file.
  * `sfeth --config-file` defaulted to `./sf.yaml` and failed if not present, and now defaults to `""` (doesn't fail is nothing is specified)

* Deprecated the "Call" and "log" indexes, now replaced by "combined" index
  * Generate new indices like this: `sfeth tools generate-combined-index --combined-indexes-size=1000 {src-blocks-url} {dest-index-url} {irreversible-index-url} {start-block} [stop-block]`
  * Delete previous indices named `xxxxxxxxxx.yyy.calladdrsig.idx` and `xxxxxxxxxx.yyy.logaddrsig.idx`


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
