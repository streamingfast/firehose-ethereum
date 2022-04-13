# Change log

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). See [MAINTAINERS.md](./MAINTAINERS.md)
for instructions to keep up to date.

## Unreleased

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
