# Change log

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). See [MAINTAINERS.md](./MAINTAINERS.md)
for instructions to keep up to date.

## Unreleased

* Substreams server: performance improvements: less redundant module execution (at the cost of more storage)

## v2.3.7

* Fixed `tools check merged-blocks` default range when `-r <range>` is not provided to now be `[0, +∞]` (was previously `[HEAD, +∞]`).

* Fixed `tools check merged-blocks` to be able to run without a block range provided.

* Added API Key based authentication to `tools firehose-client` and `tools firehose-single-block-client`, specify the value through environment variable `FIREHOSE_API_KEY` (you can use flag `--api-key-env-var` to change variable's name to something else than `FIREHOSE_API_KEY`).

* Fixed `tools check merged-blocks` examples using block range (range should be specified as `[<start>]?:[<end>]`).

* Added `--substreams-tier2-max-concurrent-requests` to limit the number of concurrent requests to the tier2 Substreams service.

## v2.3.6

* Adding traceID for RPCCalls
* BlockFetcher: added support for WithdrawalsRoot, BlobGasUsed, BlobExcessGas and ParentBeaconRoot fields when fetching blocks from RPC.
* Substreams: add support for `substreams-tier2-max-concurrent-requests` flag to limit the number of concurrent requests to tier2

## v2.3.5

### Substreams

  > [!WARNING]
  > This release deprecates the "RPC Cache (for eth_calls)" feature of substreams: It has been turned off by default and will not be supported in future releases.
  > The RPC cache was a not-well-known feature that cached all eth_calls responses by default and loaded them on each request.
  > It is being deprecated because it has a negative impact on global performance.
  > If you want to cache your eth_call responses, you should do it in a specialized proxy instead of having substreams manage this.
  > Until the feature is completely removed, you can keep the previous behavior by setting the `--substreams-rpc-cache-store-url` flag to a non-empty value (its previous default value was `{data-dir}/rpc-cache`)

* Performance: prevent reprocessing jobs when there is only a mapper in production mode and everything is already cached
* Performance: prevent "UpdateStats" from running too often and stalling other operations when running with a high parallel jobs count
* Performance: fixed bug in scheduler ramp-up function sometimes waiting before raising the number of workers
* Added the output module's hash to the "incoming request" log
* Substreams RPC: add `--substreams-rpc-gas-limit` flag to allow overriding default of 50M. Arbitrum chains behave better with a value of `0` to avoid `intrinsic gas too low (supplied gas 50000000)` errors

### Reader node

* The `reader-node-bootstrap-url` gained the ability to be bootstrapped from a `bash` script.

	If the bootstrap URL is of the form `bash:///<path/to/script>?<parameters>`, the bash script at
	`<path/to/script>` will be executed. The script is going to receive in environment variables the resolved
	reader node variables in the form of `READER_NODE_<VARIABLE_NAME>`. The fully resolved node arguments
	(from `reader-node-arguments`) are passed as args to the bash script. The query parameters accepted are:

	- `arg=<value>` | Pass as extra argument to the script, prepended to the list of resolved node arguments
	- `env=<key>%3d<value>` | Pass as extra environment variable as `<key>=<value>` with key being upper-cased (multiple(s) allowed)
	- `env_<key>=<value>` | Pass as extra environment variable as `<key>=<value>` with key being upper-cased (multiple(s) allowed)
	- `cwd=<path>` | Change the working directory to `<path>` before running the script
	- `interpreter=<path>` | Use `<path>` as the interpreter to run the script
	- `interpreter_arg=<arg>` | Pass `<interpreter_arg>` as arguments to the interpreter before the script path (multiple(s) allowed)

  > [!NOTE]
  > The `bash:///` script support is currently experimental and might change in upcoming releases, the behavior changes will be
    clearly documented here.

## v2.3.4

* Fix JSON decoding in the client tools (firehose-client, print merged-blocks, etc.).

## v2.3.3

### Known issues ###

* The block decoding to JSON is broken in the client tools (firehose-client, print merged-blocks, etc.). Use version v2.3.1

### Hotfix

* Fix block poller panic on v2.3.2

## v2.3.2

### Known issues ###

* This release has a broken RPC poller component. Upgrade to v2.3.3.
* The block decoding to JSON is broken in the client tools (firehose-client, print merged-blocks, etc.). Use version v2.3.1

### Auth and metering

* Add missing metering events for `sf.firehose.v2.Fetch/Block` responses.
* Changed default polling interval in 'continuous authentication' from 10s to 60s, added 'interval' query param to URL.

### Substreams

* Fixed bug in scheduler ramp-up function sometimes waiting before raising the number of workers
* Fixed load-balancing from tier1 to tier2 when using dns:/// (round-robin policy was not set correctly)
* Added `trace_id` in grpc authentication calls
* Bumped connect-go library to new "connectrpc.com/connect" location

## v2.3.1

### Operators

* Firehose blocks that were produced using the RPC Poller will have to be extracted again to fix the Transaction Status and the potential missing receipt (ex: arb-one pre-nitro, Avalanche, Optimism ...)

### Fixes

* Fix race condition in RPC Poller which would cause some missing transaction receipts
* Fix conversion of transaction status from RPC Poller: failed transactions would show up as "status unknown" in firehose blocks.

### Added

* Added the support the FORCE_FINALITY_AFTER_BLOCKS environment variable: setting it to a value like '200' will make the 'reader' mark blocks as final after a maximum of 200 block confirmations, even if the chain implements finality via a beacon that lags behind.

## v2.3.0

* Reduce logging and logging "payload".

* Tools printing Firehose `Block` model to JSON now have `--proto-paths` take higher precedence over well-known types and even the chain itself, the order is `--proto-paths` > `chain` > `well-known` (so `well-known` is lookup last).

* The `tools print one-block` now works correctly on blocks generated by omni-chain `firecore` binary.

* The various health endpoint now sets `Content-Type: application/json` header prior sending back their response to the client.

* The `firehose`, `substreams-tier1` and `substream-tier2` health endpoint now respects the `common-system-shutdown-signal-delay` configuration value meaning that the health endpoint will return `false` now if `SIGINT` has been received but we are still in the shutdown unready period defined by the config value. If you use some sort of load balancer, you should make sure they are configured to use the health endpoint and you should `common-system-shutdown-signal-delay` to something like `15s`.

* Changed `reader` logger back to `reader-node` to fit with the app's name which is `reader-node`.

* Fix `tools compare-blocks` that would fail on new format.

* Fix `substreams` to correctly delete `.partial` files when serving a request that is not on a boundary

## v2.2.2

The Cancun hard fork happened on Goerli and after further review, we decided to change the Protobuf definition for the new `BlockHeader`, `Transaction` and `TransactionReceipt` fields that are related to blob transaction.

We made explicit that those fields are optional in the Protobuf definition which will render them in your language of choice using the appropriate "null" mechanism. For example on Golang, those fields are generated as `BlobGasUsed *uint64` and `ExcessBlobGas *uint64` which will make it clear that those fields are not populated at all.

The affected fields are:
  - [BlockHeader.blob_gas_used](./proto/sf/ethereum/type/v2/type.proto#L173), now `optional uint64`.
  - [BlockHeader.excess_blob_gas](./proto/sf/ethereum/type/v2/type.proto#L176), now `optional uint64`.
  - [TransactionTrace.blob_gas](./proto/sf/ethereum/type/v2/type.proto#L369), now `optional uint64`.
  - [TransactionTrace.blob_gas_fee_cap](./proto/sf/ethereum/type/v2/type.proto#L377), now `optional BigInt`.
  - [TransactionReceipt.blob_gas_used](./proto/sf/ethereum/type/v2/type.proto#L428), now `optional uint64`.
  - [TransactionReceipt.blob_gas_price](./proto/sf/ethereum/type/v2/type.proto#L436), now `optional BigInt`.

This is technically a breaking change for those that could have consumed those fields already but we think he impact is so minimal that it's better to make the change right now.

### Operators

You will need to reprocess a small Goerli range. You should update to new version to produce the newer version and the reprocess from block 10377700 up to when you upgraded to v2.2.2.

The block 10377700 was chosen since it is the block at the time of the first release we did supporting Cancun where we introduced those new field. If you know when you deploy either `v2.2.0` or `v2.2.1`, you should reprocess from that point.

An alternative to reprocessing is updating your blocks by having a [StreamingFast API Token](https://substreams.streamingfast.io/getting-started/quickstart#run-your-first-substreams) and using `fireeth tools download-from-firehose goerli.eth.streamingfast.io:443 -a SUBSTREAMS_API_TOKEN 10377700:<recent block rounded to 100s> <destination>`.

> [!NOTE]
> You should download the blocks to a temporary destination and copy over to your production destination once you have them all.

You can reach to us on Discord if you need help on something.

## v2.2.1

* Updated the documentation for some of the upcoming new Cancun hard-fork fields:
  - [TransactionTrace.blob_gas](./proto/sf/ethereum/type/v2/type.proto#L369)
  - [TransactionReceipt.blob_gas_used](./proto/sf/ethereum/type/v2/type.proto#L428)
  - [TransactionReceipt.blob_gas_price](./proto/sf/ethereum/type/v2/type.proto#L436)

## v2.2.0

### Support for Cancun fork (Goerli: Jan 17th)

* Added support for EIP-4844 (upcoming with activation of Cancun fork), through instrumented go-ethereum nodes with version `fh2.4`. This adds new fields in the Ethereum Block model, fields that will be non-empty when the Ethereum network your pulling have EIP-4844 activated.  The fields in questions are:
  - [Block.system_calls](./proto/sf/ethereum/type/v2/type.proto#L69)
  - [BlockHeader.blob_gas_used](./proto/sf/ethereum/type/v2/type.proto#L173)
  - [BlockHeader.excess_blob_gas](./proto/sf/ethereum/type/v2/type.proto#L176)
  - [BlockHeader.parent_beacon_root](./proto/sf/ethereum/type/v2/type.proto#L179)
  - [TransactionTrace.blob_gas](./proto/sf/ethereum/type/v2/type.proto#L369)
  - [TransactionTrace.blob_gas_fee_cap](./proto/sf/ethereum/type/v2/type.proto#L377)
  - [TransactionTrace.blob_hashes](./proto/sf/ethereum/type/v2/type.proto#L387)
  - [TransactionReceipt.blob_gas_used](./proto/sf/ethereum/type/v2/type.proto#L428)
  - [TransactionReceipt.blob_gas_price](./proto/sf/ethereum/type/v2/type.proto#L436)
  - A new `TransactionTrace.Type` value [TRX_TYPE_BLOB](./proto/sf/ethereum/type/v2/type.proto#L283)

> [!IMPORTANT]
> Operators running Goerli chain will need to upgrade to this version, with this geth node release: https://github.com/streamingfast/go-ethereum/releases/tag/geth-v1.13.10-fh2.4

### Substreams server (bumped to v1.3.1)

* Fixed error-passing between tier2 and tier1 (tier1 will not retry sending requests that fail deterministicly to tier2)
* Tier1 will now schedule a single job on tier2, quickly ramping up to the requested number of workers after 4 seconds of delay, to catch early exceptions
* "store became too big" is now considered a deterministic error and returns code "InvalidArgument"

### Misc

* Added `tools poller generic-evm` subcommand. It is identical to optimism/arb-one in feature at the moment and should work for most evm chains.

## v2.1.0

* Bump to major release firehose-core v1.0.0

### Operators

> [!IMPORTANT]
> When upgrading your stack to this release, be sure to upgrade all components simultaneously because the block encapsulation format has changed.
> Blocks that are merged using the new merger will not be readable by previous versions.
> There is no simple way to revert, except by deleting the all the one-blocks and merged-blocks that were produced with this version.

### Changed

* Blocks files (one-blocks and merged) are now stored with a new format using `google.protobuf.any` format. Previous blocks can still be read and processed.

### Added

* Added RPC pollers for Optimism and Arb-one: These can be used from by running the reader-node with `--reader-node-path=/path/to/fireeth` and `--reader-node-arguments="tools poller {optimism|arb-one} [more flags...]"`
* Added `tools fix-any-type` to rewrite the previous merged-blocks (OPTIONAL)

## v2.0.2

* Fixed grpc error code when shutting down: changed from Canceled to Unavailable

## v2.0.1

* Fixed SF_TRACING feature (regression broke the ability to specify a tracing endpoint)
* Fixed substreams GRPC/Connect error codes not propagating correctly
* Firehose connections rate-limiting will now force an (increased) delay of between 1 and 4 seconds (random value)  before refusing a connection when under heavy load

## v2.0.0

### Fixed

* Fixed the `fix-polygon-index` tool (parsing error made it unusable in v2.0.0-rc.1)
* Fixed some false positives in `compare-blocks-rpc`

## v2.0.0-rc.1

### Highlights

This releases refactor `firehose-ethereum` repository to use the common shared Firehose Core library (https://github.com/streamingfast/firehose-core) that every single Firehose supported chain should use and follow.

Both at the data level and gRPC level, there is no changes in behavior to all core components which are `reader-node`, `merger`, `relayer`, `firehose`, `substreams-tier1` and `substreams-tier2`.

A lot of changes happened at the operators level however and some superflous mode have been removed, especially around the `reader-node` application. The full changes is listed below, operators should review thoroughly the changelog.

> [!IMPORTANT]
> It's important to emphasis that at the data level, nothing changed, so reverting to 1.4.22 in case of a problem is quite easy and no special data migration is required outside of changing back to the old set of flags that was used before.

#### Operators

You will find below the detailed upgrade procedure for the configuration file operators usually use. If you are using the flags based approach, simply update the corresponding flags.

> [!IMPORTANT]
> We have had reports of older versions of this software creating corrupted merged-blocks-files (with duplicate or out-of-bound blocks)
> This release adds additional validation of merged-blocks to prevent serving duplicate blocks from the firehose or substreams service.
> This may cause service outage if you have produced those blocks or downloaded them from another party who was affected by this bug.
> See the **Finding and fixing corrupted merged-blocks-files** to see how you can prevent service outage.

##### Quick Upgrade

Here a bullet list for upgrading your instance, we still recommend to fully read each section below, the list here can serve as a check list. The list below is done in such way that you get back the same "instance" as before. The listening addresses changes can be omitted as long as you update other tools to account for the port changes list your load balancer.

- Add config `config-file: ./sf.yaml` if not present already
- Add config `data-dir: ./sf-data` if not present already
- Rename config `verbose` to `log-verbosity` if present
- Add config `common-blocks-cache-dir: ./sf-data/blocks-cache` if not present already
- Remove config `common-chain-id` if present
- Remove config `common-deployment-id` if present
- Remove config `common-network-id` if present
- Add config `common-live-blocks-addr: :13011` if not present already
- Add config `relayer-grpc-listen-addr: :13011` if `common-live-blocks-addr` has been added in previous step
- Add config `reader-node-grpc-listen-addr: :13010` if not present already
- Add config `relayer-source: :13010` if `reader-node-grpc-listen-addr` has been added in previous step
- Remove config `reader-node-enforce-peers` if present
- Remove config `reader-node-log-to-zap` if present
- Remove config `reader-node-ipc-path` if present
- Remove config `reader-node-type` if present
- Replace config `reader-node-arguments: +--<flag1> --<flag2> ...` by `reader-node-arguments: --networkid=<network-id> --datadir={node-data-dir} --port=30305 --http --http.api=eth,net,web3 --http.port=8547 --http.addr=0.0.0.0 --http.vhosts=* --firehose-enabled --<flag1> --<flag2> ...`

  > [!NOTE]
  > The `<network-id>` is dynamic and should be replace with a literal value like `1` for Ethereum Mainnet. The `{node-data-dir}` value is actually a templating value that is going o be resolved for you (resolves to value of config `reader-node-data-dir`).!

  > [!IMPORTANT]
  > Ensure that `--firehose-enabled` is part of the flag! Moreover, tweak flags to avoid repetitions if your were overriding some of them.

- Remove `node` under `start: args:` list
- Add config `merger-grpc-listen-addr: :13012` if not present already
- Add config `firehose-grpc-listen-addr: :13042` if not present already
- Add config `substreams-tier1-grpc-listen-addr: :13044` if not present already
- Add config `substreams-tier1-grpc-listen-addr: :13044` if not present already
- Add config `substreams-tier2-grpc-listen-addr: :13045` if not present already
- Add config `substreams-tier1-subrequests-endpoint: :13045` if `substreams-tier1-grpc-listen-addr` has been added in previous step
- Replace config `combined-index-builder` to `index-builder` under `start: args:` list
- Rename config `common-block-index-sizes` to `common-index-block-sizes` if present
- Rename config `combined-index-builder-grpc-listen-addr` to `index-builder-grpc-listen-addr` if present
- Add config `index-builder-grpc-listen-addr: :13043` if you didn't have `combined-index-builder-grpc-listen-addr` previously
- Rename config `combined-index-builder-index-size` to `index-builder-index-size` if present
- Rename config `combined-index-builder-start-block` to `index-builder-start-block` if present
- Rename config `combined-index-builder-stop-block` to `index-builder-stop-block` if present
- Replace any occurrences of `{sf-data-dir}` to `{data-dir}` in any of your configuration values if present

#### Common Changes

* The default value for `config-file` changed from `sf.yaml` to `firehose.yaml`. If you didn't had this flag defined and wish to keep the old default, define `config-file: sf.yaml`.

* The default value for `data-dir` changed from `sf-data` to `firehose-data`. If you didn't had this flag defined before, you should either move `sf-data` to `firehose-data` or define `data-dir: sf-data`.

  > [!NOTE]
  > This is an important change, forgetting to change it will change expected locations of data leading to errors or wrong data.

* **Deprecated** The `{sf-data-dir}` templating argument used in various flags to resolve to the `--data-dir=<location>` value has been deprecated and should now be simply `{data-dir}`. The older replacement is still going to work but you should replace any occurrences of `{sf-data-dir}` in your flag definition by `{data-dir}`.

* The default value for `common-blocks-cache-dir` changed from `{sf-data-dir}/blocks-cache` to `file://{data-dir}/storage/blocks-cache`. If you didn't had this flag defined and you had `common-blocks-cache-enabled: true`, you should define `common-blocks-cache-dir: file://{data-dir}/blocks-cache`.

* The default value for `common-live-blocks-addr` changed from `:13011` to `:10014`. If you didn't had this flag defined and wish to keep the old default, define `common-live-blocks-addr: 13011` and ensure you also modify `relayer-grpc-listen-addr: :13011` (see next entry for details).

* The Go module `github.com/streamingfast/firehose-ethereum/types` has been removed, if you were depending on `github.com/streamingfast/firehose-ethereum/types` in your project before, depend directly on `github.com/streamingfast/firehose-ethereum` instead.

  > [!NOTE]
  > This will pull much more dependencies then before, if you're reluctant of such additions, talk to us on Discord and we can offer alternatives depending on what you were using.

* The config value `verbose` has been renamed to `log-verbosity` keeping the same semantic and default value as before

  > [!NOTE]
  > The short flag version is still `-v` and can still be provided multiple times like `-vvvv`.

#### App `reader-node` changes

This change will impact all operators currently running Firehose on Ethereum so it's important to pay attention to the upgrade procedure below, if you are unsure of something, reach to us on [Discord](https://discord.gg/jZwqxJAvRs).

Before this release, the `reader-node` app was managing for you a portion of the `reader-node-arguments` configuration value, prepending some arguments that would be passed to `geth` when invoking it, the list of arguments that were automatically provided before:

- `--networkid=<value of config value 'common-network-id'>`
- `--datadir=<value of config value 'reader-node-data-dir'>`
- `--ipcpath=<value of config value 'reader-node-ipc-path'>`
- `--port=30305`
- `--http`
- `--http.api=eth,net,web3`
- `--http.port=8547`
- `--http.addr=0.0.0.0`
- `--http.vhosts=*`
- `--firehose-enabled`

We have now removed those magical additions and operators are now responsible of providing the flags they required to properly run a Firehose-enabled native `geth` node. The `+` sign that was used to append/override the flags has been removed also since no default additions is performed, the `+` was now useless. To make some flag easier to define and avoid repetition, a few templating variable can be used within the `reader-node-arguments` value:

- `{data-dir}`         The current data-dir path defined by the config value `data-dir`
- `{node-data-dir}`    The node data dir path defined by the flag `reader-node-data-dir`
- `{hostname}`         The machine's hostname
- `{start-block-num}`  The resolved start block number defined by the flag `reader-node-start-block-num` (can be overwritten)
- `{stop-block-num}`   The stop block number defined by the flag `reader-node-stop-block-num`

As an example, if you provide the config value `reader-node-data-dir=/var/geth` for example, then you could use `reader-node-arguments: --datadir={node-data-dir}` and that would resolve to `reader-node-arguments: --datadir=/var/geth` for you.

> [!NOTE]
> The `reader-node-arguments` is a string that is parsed using Shell word splitting rules which means for example that double quotes are supported like `--datadir="/var/with space/path"` and the argument will be correctly accepted. We use https://github.com/kballard/go-shellquote as your parsing library.

We also removed the following `reader-node` configuration value:

- `reader-node-type` (No replacement needed, just remove it)
- `reader-node-ipc-path` (If you were using that, define it manually using `geth` flag `--ipcpath=...`)
- `reader-node-enforce-peers` (If you were using that, use a `geth` config file to add static peers to your node, read about static peers for `geth` on the Web)

Default listening addresses changed also to be the same on all `firehose-<...>` project, meaning consistent ports across all chains for operators. The `reader-node-grpc-listen-addr` default listen address went from `:13010` to `:10010` and `reader-node-manager-api-addr` from `:13009` to `:10011`. If you have no occurrences of `13010` or `13009` in your config file or your scripts, there is nothing to do. Otherwise, feel free to adjust the default port to fit your needs, if you do change `reader-node-grpc-listen-addr`, ensure `--relayer-source` is also updated as by default it points to `:10010`.

Here an example of the required changes.

Change:

```yaml
start:
  args:
  - ...
  - reader-node
  - ...
  flags:
    ...
    reader-node-bootstrap-data-url: ./reader/genesis.json
    reader-node-enforce-peers: localhost:13041
    reader-node-arguments: +--firehose-genesis-file=./reader/genesis.json --authrpc.port=8552
    reader-node-log-to-zap: false
    ...
```

To:

```yaml
start:
  args:
  - ...
  - reader-node
  - ...
  flags:
    ...
    reader-node-bootstrap-data-url: ./reader/genesis.json
    reader-node-arguments:
      --networkid=1515
      --datadir={node-data-dir}
      --ipcpath={data-dir}/reader/ipc
      --port=30305
      --http
      --http.api=eth,net,web3
      --http.port=8547
      --http.addr=0.0.0.0
      --http.vhosts=*
      --firehose-enabled
      --firehose-genesis-file=./reader/genesis.json
      --authrpc.port=8552
    ...
```

> [!NOTE]
> Adjust the `--networkid=1515` value to fit your targeted chain, see https://chainlist.org/ for a list of Ethereum chain and their `network-id` value.

#### App `node` removed

In previous version of `firehose-ethereum`, it was possible to use the `node` app to launch managed "peering/backup/whatever" Ethereum node, this is not possible anymore. If you were using the `node` app previously, like in this config:

```yaml
start:
  args:
  - ...
  - node
  - ...
  flags:
    ...
    node-...
```

You must now remove the `node` app from `args` and any flags starting with `node-`. The migration path is to run those on your own without the use of `fireeth` and using whatever tools fits your desired needs.

We have completely drop support to concentrate on the core mission of Firehose which is to run reader nodes to extract Firehose blocks from it.

> **Note** This is about the `node` app and **not** the `reader-node`, we think usage of this app is minimal/inexistent.

#### Rename of `combined-index-builder` to `index-builder`

The app has been renamed to simply `index-builder` and the flags has been completely renamed removing the prefix `combined-` in front of them.

Change:

```yaml
start:
  args:
  - ...
  - combined-index-builder
  - ...
  flags:
    ...
    combined-index-builder-grpc-listen-addr: ":9999"
    combined-index-builder-index-size: 10000
    combined-index-builder-start-block: 0
    combined-index-builder-stop-block: 0
    ...
```

To:

```yaml
start:
  args:
  - ...
  - index-builder
  - ...
  flags:
    ...
    index-builder-grpc-listen-addr: ":9999"
    index-builder-index-size: 10000
    index-builder-start-block: 0
    index-builder-stop-block: 0
    ...
```

* Flag `common-block-index-sizes` has been renamed to `common-index-block-sizes`.

> [!NOTE]
> Rename only configuration item you had previously defined, do not copy paste verbatim example above.

#### App `relayer` changes

* The default value for `relayer-grpc-listen-addr` changed from `:13011` to `:10014`. If you didn't had this flag defined and wish to keep the old default, define `relayer-grpc-listen-addr: 13011` and ensure you also modify `common-live-blocks-addr: :13011` (see previous entry for details).

* The default value for `relayer-source` changed from `:13010` to `:10010`. If you didn't had this flag defined and wish to keep the old default, define `relayer-source: 13010` and ensure you also modify `reader-node-grpc-listen-addr: :13010`.

  > [!NOTE]
  > Must align with `reader-node-grpc-listen-addr`!

#### App `firehose` changes

* The default value for `firehose-grpc-listen-addr` changed from `:13042` to `:10015`. If you didn't had this flag defined and wish to keep the old default, define `firehose-grpc-listen-addr: :13042`.
* Firehose logs now include auth information (userID, keyID, realIP) along with blocks + egress bytes sent.

#### App `merger` changed

* The default value for `merger-grpc-listen-addr` changed from `:13012` to `:10012`. If you didn't had this flag defined and wish to keep the old default, define `merger-grpc-listen-addr: :13012`.

#### App `substreams-tier1` and `substreams-tier2` changed

* The default value for `substreams-tier1-grpc-listen-addr` changed from `:13044` to `:10016`. If you didn't had this flag defined and wish to keep the old default, define `substreams-tier1-grpc-listen-addr: :13044`.

* The default value for `substreams-tier1-subrequests-endpoint` changed from `:13045` to `:10017`. If you didn't had this flag defined and wish to keep the old default, define `substreams-tier1-subrequests-endpoint: :13044`.

  > [!NOTE]
  > Must align with `substreams-tier1-grpc-listen-addr`!

* The default value for `substreams-tier2-grpc-listen-addr` changed from `:13045` to `:10017`. If you didn't had this flag defined and wish to keep the old default, define `substreams-tier2-grpc-listen-addr: :13045`.

#### Protobuf model changes

* Added field `DetailLevel` (Base, Extended(default)) to `sf.ethereum.type.v2.Block` to distinguish the new blocks produced from polling RPC (base) from the blocks normally produced with firehose instrumentation (extended)

#### Tools changes

* Added command `tools fix-bloated-merged-blocks` to go through a range of possibly corrupted merged-blocks (with duplicates and out-of-range blocks) and try to fix them, writing the fixed merged-blocks files to another destination.

#### Removed

* Transform `sf.ethereum.transform.v1.LightBlock` is not supported, this has been deprecated for a long time and should not be used anywhere.

#### Finding and fixing corrupted merged-blocks files

You may have certain merged-blocks files (most likely OLD blocks) that contain more than 100 blocks (with duplicate or extra out-of-bound blocks)

* Find the affected files by running the following command (can be run multiple times in parallel, over smaller ranges)

```
tools check merged-blocks-batch <merged-blocks-store> <start> <stop>
```

* If you see any affected range, produce fixed merged-blocks files with the following command, on each range:

```
tools fix-bloated-merged-blocks <merged-blocks-store> <output-store> <start>:<stop>
```

* Copy the merged-blocks files created in output-store over to the your merged-blocks-store, replacing the corrupted files.

# v1.4.22

* Fixed a regression where `reader-node-role` was changed to `dev` by default, putting back the default `geth` value.

## v1.4.21

* Bump Substreams to `v1.1.20` with a fix for some minor bug fixes related to start block processing

## v1.4.20

### Added

* Added `tools poll-rpc-blocks` command to launch an RPC-based poller that acts as a firehose extractor node, printing base64-encoded protobuf blocks to stdout (used by the 'dev' node-type). It creates "light" blocks, without traces and ordinals.
* Added `--dev` flag to the `start` command to simplify running a local firehose+substreams stack from a development node (ex: Hardhat).
  * This flag overrides the `--reader-node-path`, instead pointing to the fireeth binary itself.
  * This flag overrides the `--reader-node-type`, setting it to `dev` instead of `geth`.
    This node type has the following default `reader-node-arguments`: `tools poll-rpc-blocks http://localhost:8545 0`
  * It also removes `node` from the list of default apps

### Fixed

* Substreams: fixed metrics calculations (per-module processing-time and external calls were wrong)
* Substreams: fixed immediate EOF when streaming from block 0 to (unbounded) in dev mode

## v1.4.19

* Bumped substreams to `v1.1.18` with a regression fix for when a substreams has a start block in the reversible segment

## v1.4.18
* Bumped substreams to `v1.1.17` with fix `missing decrement on metrics `substreams_active_requests`

## v1.4.17

### Added

The `--common-auth-plugin` got back the ability to use `secret://<expected_secret>?[user_id=<user_id>]&[api_key_id=<api_key_id>]` in which case request are authenticated based on the `Authorization: Bearer <actual_secret>` and continue only if `<actual_secret> == <expected_secret>`.

### Changed

* Bumped substreams to `v1.1.16` with support of metrics `substreams_active_requests` and `substreams_counter`

## v1.4.16

### Operators of Polygon/Mumbai chains

* If you started reprocessing the blockchain blocks using release v1.4.14 or v1.4.15, you will need to run the following command to fix the blocks affected by another bug:
  `fireeth tools fix-polygon-index /your/merged/blocks /temporary/destination 0 48200000` (note that you can run multiple instances of this command in parallel to cover the range of blocks from 0 to current HEAD in smaller chunks)

### Fixed

* Fix another data issue found in polygon blocks: blocks that contain a single "system" transaction have "Index=1" for that transaction instead of "Index=0"

## v1.4.15

### Fixed
* (Substreams) fixed regressions for relative start-blocks for substreams (see https://github.com/streamingfast/substreams/releases/tag/v1.1.14)

## v1.4.14

### Operators

If you are indexing Polygon or Mumbai chains, you will need to reprocess the chain from genesis, as your existing Firehose blocks are missing some system transactions.

As always, this can be done with multiple client nodes working in parallel on different chain's segment if you have snapshots at various block heights.

Golang `1.21+` is now also required to build the project.

### Fixed

* Fixed post-processing of polygon blocks: some system transactions were not "bundled" correctly.
* (Substreams) fixed validations for invalid start-blocks (see https://github.com/streamingfast/substreams/releases/tag/v1.1.13)

### Added

* Added `tools compare-oneblock-rpc` command to perform a validation between a firehose 'one-block-file' blocks+trx+logs fetched from an RPC endpoint

### Changed

* The `tools print` subcommands now use hex to encode values instead of base64, making them easier to use

## v1.4.13

> [!IMPORTANT]
> The Substreams service exposed from this version will send progress messages that cannot be decoded by substreams clients prior to v1.1.12.
> Streaming of the actual data will not be affected. Clients will need to be upgraded to properly decode the new progress messages.

### Changed

* Bumped substreams to `v1.1.12` to support the new progress message format. Progression now relates to **stages** instead of modules. You can get stage information using the `substreams info` command starting at version `v1.1.12`.

### Added

* added `tools compare-blocks-rpc` command to perform a validation between firehose blocks and blocks+trx+logs fetched from an RPC endpoint

### Fixed

* More tolerant retry/timeouts on filesource (prevent "Context Deadline Exceeded")

## v1.4.12

### Highlights

#### Operators

This release mainly brings `reader-node` Firehose Protocol 2.3 support for all networks and not just Polygon. This is important for the upcoming release of Firehose-enabled `geth` version 1.2.11 and 1.2.12 that are going to be releases shortly.

Golang `1.20+` is now also required to build the project.

### Added

* Support reader node Firehose Protocol 2.3 on all networks now (and not just Polygon).

### Removed

* Removed `--substreams-tier1-request-stats` and `--substreams-tier1-request-stats` (substreams request-stats are now always sent to clients)

### Fixed

* `tools check merged-blocks` now correctly prints missing block gaps even without print-full or print-stats.

### Changed

* Now requires Go 1.20+ to compile the project.
* Substreams bumped: better "Progress" messages

## v1.4.11

### Fixes

* Bumped `firehose` and `substreams` library to fix a bug where live blocks were not metered correctly.

## v1.4.10

### Fixes

* Fixed: jobs would hang when flags `--substreams-state-bundle-size` and `--substreams-tier1-subrequests-size` had different values. The latter flag has been completely **removed**, subrequests will be bound to the state bundle size.

### Added

* Added support for *continuous authentication* via the grpc auth plugin (allowing cutoff triggered by the auth system).

## v1.4.9

### Highlights

#### Substreams State Store Selection

The `substreams` server now accepts `X-Sf-Substreams-Cache-Tag` header to select which Substreams state store URL should be used by the request. When performing a Substreams request, the servers will pick the state store based on the header. This enable consumers to stay on the same cache version when the operators needs to bump the data version (reasons for this could be a bug in Substreams software that caused some cached data to be corrupted on invalid).

To benefit from this, operators that have a version currently in their state store URL should move the version part from `--substreams-state-store-url` to the new flag `--substreams-state-store-default-tag`. For example if today you have in your config:

```yaml
start:
  ...
  flags:
    substreams-state-store-url: /<some>/<path>/v3
```

You should convert to:

```yaml
start:
  ...
  flags:
    substreams-state-store-url: /<some>/<path>
    substreams-state-store-default-tag: v3
```

#### Substreams Scheduler Improvements for Parallel Processing

The `substreams` scheduler has been improved to reduce the number of required jobs for parallel processing. This affects `backprocessing` (preparing the states of modules up to a "start-block") and `forward processing` (preparing the states and the outputs to speed up streaming in production-mode).

Jobs on `tier2` workers are now divided in "stages", each stage generating the partial states for all the modules that have the same dependencies. A `substreams` that has a single store won't be affected, but one that has 3 top-level stores, which used to run 3 jobs for every segment now only runs a single job per segment to get all the states ready.

### Operators Upgrade

The app `substreams-tier1` and `substreams-tier2` should be upgraded concurrently. Some calls will fail while versions are misaligned.

### Backend Changes

* Substreams bumped to version v1.1.9
* Authentication plugin `trust` can now specify an exclusive list of `allowed` headers (all lowercase), ex: `trust://?allowed=x-sf-user-id,x-sf-api-key-id,x-real-ip,x-sf-substreams-cache-tag`
* The `tier2` app no longer uses the `common-auth-plugin`, `trust` will always be used, so that `tier1` can pass down its headers (ex: `X-Sf-Substreams-Cache-Tag`).

## v1.4.8

### Fixed

* Fixed a bug in `substreams-tier1` and `substreams-tier2` which caused "live" blocks to be sent while the stream previously received block(s) were historic.

### Added

* Added a check for readiness of the `dauth` provider when answering "/healthz" on firehose and substreams


### Changed

* Changed `--substreams-tier1-debug-request-stats` to `--substreams-tier1-request-stats` which enabled request stats logging on Substreams Tier1
* Changed `--substreams-tier2-debug-request-stats` to `--substreams-tier2-request-stats` which enabled request stats logging on Substreams Tier2

## v1.4.7

* Fixed an occasional panic in substreams-tier1 caused by a race condition
* Fixed the grpc error codes for substreams tier1: Unauthenticated on bad auth, Canceled (endpoint is shutting down, please reconnect) on shutdown
* Fixed the grpc healthcheck method on substreams-tier1 (regression)
* Fixed the default value for flag `common-auth-plugin`: now set to 'trusted://' instead of panicking on removed 'null://'

## v1.4.6

### Changed

* Substreams (@v1.1.6) is now out of the `firehose` app, and must be started using `substreams-tier1` and `substreams-tier2` apps!
* Most substreams-related flags have been changed:
  * common: `--substreams-rpc-cache-chunk-size`,`--substreams-rpc-cache-store-url`,`--substreams-rpc-endpoints`,`--substreams-state-bundle-size`,`--substreams-state-store-url`
  * tier1: `--substreams-tier1-debug-request-stats`,`--substreams-tier1-discovery-service-url`,`--substreams-tier1-grpc-listen-addr`,`--substreams-tier1-max-subrequests`,`--substreams-tier1-subrequests-endpoint`,`--substreams-tier1-subrequests-insecure`,`--substreams-tier1-subrequests-plaintext`,`--substreams-tier1-subrequests-size`
  * tier2: `--substreams-tier2-discovery-service-url`,`--substreams-tier2-grpc-listen-addr`
* Some auth plugins have been removed, the new available plugins for `--common-auth-plugins` are `trust://` and `grpc://`. See https://github.com/streamingfast/dauth for details
* Metering features have been added, the available plugins for `--common-metering-plugin` are `null://`, `logger://`, `grpc://`. See https://github.com/streamingfast/dmetering for details

### Added

* Support for reader node Firehose Protocol 2.3 (for parallel processing of transactions, added to polygon 'bor' v0.4.0)

### Removed

* Removed the `tools upgrade-merged-blocks` command. Normalization is now part of consolereader within 'codec', not the 'types' package, and cannot be done a posteriori.
* Updated metering to fix dependencies

## v1.4.5

* Updated metering (bumped versions of `dmetering`, `dauth`, and `firehose` libraries.)
* Fixed firehose service healthcheck on shutdown
* Fixed panic on download-blocks-from-firehose tool

## v1.4.4

#### Operators

* When upgrading a substreams server to this version, you should delete all existing module caches to benefit from deterministic output

### Substreams changes

* Switch default engine from `wasmtime` to `wazero`
* Prevent reusing memory between blocks in wasm engine to fix determinism
* Switch our store operations from bigdecimal to fixed point decimal to fix determinism
* Sort the store deltas from `DeletePrefixes()` to fix determinism
* Implement staged module execution within a single block.
* "Fail fast" on repeating requests with deterministic failures for a "blacklist period", preventing waste of resources
* SessionInit protobuf message now includes resolvedStartBlock and MaxWorkers, sent back to the client

## v1.4.3

### Highlights

* This release brings an update to `substreams` to `v1.1.4` which includes the following:
  - Changes the module hash computation implementation to allow reusing caches accross substreams that 'import' other substreams as a dependency.
  - Faster shutdown of requests that fail deterministically
  - Fixed memory leak in RPC calls

### Note for Operators

> **Note** This upgrade procedure is applies if your Substreams deployment topology includes both `tier1` and `tier2` processes. If you have defined somewhere the config value `substreams-tier2: true`, then this applies to you, otherwise, if you can ignore the upgrade procedure.

The components should be deployed simultaneously to `tier1` and `tier2`, or users will end up with backend error(s) saying that some partial file are not found. These errors will be resolved when both tiers are upgraded.

### Added

* Added Substreams scheduler tracing support. Enable tracing by setting the ENV variables `SF_TRACING` to one of the following:
  - `stdout://`
  - `cloudtrace://[host:port]?project_id=<project_id>&ratio=<0.25>`
  - `jaeger://[host:port]?scheme=<http|https>`
  - `zipkin://[host:port]?scheme=<http|https>`
  - `otelcol://[host:port]`

## v1.4.2

### Highlights

* This release brings an update to `substreams` to `v1.1.3` which includes the following:
  - Fixes an important bug that could have generated corrupted store state files. This is important for developers and operators.
  - Fixes for race conditions that would return a failure when multiple identical requests are backprocessing.
  - Fixes and speed/scaling improvements around the engine.

### Note for Operators

> **Note** This upgrade procedure is applies if your Substreams deployment topology includes both `tier1` and `tier2` processes. If you have defined somewhere the config value `substreams-tier2: true`, then this applies to you, otherwise, if you can ignore the upgrade procedure.

This release includes a small change in the internal RPC layer between `tier1` processes and `tier2` processes. This change requires an ordered upgrade of the processes to avoid errors.

The components should be deployed in this order:
1. Deploy and roll out `tier1` processes first
2. Deploy and roll out `tier2` processes in second

If you upgrade in the wrong order or if somehow `tier2` processes start using the new protocol without `tier1` being aware, user will end up with backend error(s) saying that some partial file are not found. Those will be resolved only when `tier1` processes have been upgraded successfully.

## v1.4.1

### Fixed

* Substreams running without a specific tier2 `substreams-client-endpoint` will now expose tier2 service `sf.substreams.internal.v2.Substreams` so it can be used internally.

> **Warning**
> If you don't use dedicated tier2 nodes, make sure that you don't expose `sf.substreams.internal.v2.Substreams` to the public (from your load-balancer or using a firewall)


### Breaking changes

* flag `substreams-partial-mode-enabled` renamed to `substreams-tier2`
* flag `substreams-client-endpoint` now defaults to empty string, which means it is its own client-endpoint (as it was before the change to protocol V2)

## v1.4.0

### Substreams RPC protocol V2

Substreams protocol changed from `sf.substreams.v1.Stream/Blocks` to `sf.substreams.rpc.v2.Stream/Blocks` for client-facing service. This changes the way that substreams clients are notified of chain reorgs.
All substreams clients need to be upgraded to support this new protocol.

See https://github.com/streamingfast/substreams/releases/tag/v1.1.1 for details.

### Added

* `firehose-client` tool now accepts `--limit` flag to only send that number of blocks. Get the latest block like this: `fireeth tools firehose-client <endpoint> --limit=1 -- -1 0`

## v1.3.8

### Highlights

This is a bug fix release for node operators that are about to upgrade to Shanghai release. The Firehose instrumented `geth` compatible with Shanghai release introduced a new message `CANCEL_BLOCK`. It seems in some circumstances, we had a bug in the console reader that was actually panicking but the message was received but no block was actively being assembled.

This release fix this bogus behavior by simply ignoring `CANCEL_BLOCK` message when there is no active block which is harmless. Every node operators that upgrade to https://github.com/streamingfast/go-ethereum/releases/tag/geth-v1.11.5-fh2.2 should upgrade to this version.

> **Note** There is no need to update the Firehose instrumented `geth` binary, only `fireeth` needs to be bumped if you already are at the latest `geth` version.

### Fixed

* Fixed a bug on console reader when seeing `CANCEL_BLOCK` on certain circumstances.

### Changed

* Now using Golang 1.20 for building releases.

* Changed default value of flag `substreams-sub-request-block-range-size` from `1000` to `10000`.

## v1.3.7

### Fixed

* Fixed a bug in data normalization for Polygon chain which would cause panics on certain blocks.

### Added

* Support for gcp `archive` types of snapshots

## v1.3.6

### Highlights

* This release implements the new `CANCEL_BLOCK` instruction from Firehose protocol 2.2 (`fh2.2`), to reject blocks that failed post-validation.
* This release fixes polygon "StateSync" transactions by grouping the calls inside an artificial transaction.

If you had previous blocks from a Polygon chain (bor), you will need to reprocess all your blocks from the node because some StateSync transactions may be missing on some blocks.

#### Operators

This release now supports the new Firehose node exchange format 2.2 which introduced a new exchanged message `CANCEL_BLOCK`. This has an implication on the Firehose instrumented `Geth` binary you can use with the release.

- If you use Firehose instrumented `Geth` binary tagged `fh2.2` (like `geth-v1.11.4-fh2.2-1`), you must use `firehose-ethereum` version `>= 1.3.6`
- If you use Firehose instrumented `Geth` binary tagged `fh2.1` (like `geth-v1.11.3-fh2.1`), you can use `firehose-ethereum` version `>= 1.0.0`

New releases of Firehose instrumented `Geth` binary for all chain will soon all be tagged `fh2.2`, so upgrade to `>= 1.3.6` of `firehose-ethereum` will be required.

## v1.3.5

### Highlights

This release is required if you run on Goerli and is mostly about supporting the upcoming Shanghai fork that has been activated on Goerli on March 14th.

### Changed

* Added support for `withdrawal` balance change reason in block model, this is required for running on most recent Goerli Shanghai hard fork.
* Added support for `withdrawals_root` on `Header` in the block model, this will be populated only if the chain has activated Shanghai hard fork.
* `--substreams-max-fuel-per-block-module` will limit the number of wasmtime instructions for a single module in a single block.

## v1.3.4

### Highlights

#### Fixed the 'upgrade-merged-blocks' from v2 to v3

Blocks that were migrated from v2 to v3 using the 'upgrade-merged-blocks' should now be considered invalid.
The upgrade mechanism did not correctly fix the "caller" on DELEGATECALLs when these calls were nested under another DELEGATECALL.

You should run the `upgrade-merged-blocks` again if you previously used 'v2' blocks that were upgraded to 'v3'.

#### Backoff mechanism for bursts

This mechanism uses a leaky-bucket mechanism, allowing an initial burst of X connections, allowing a new connection every Y seconds or whenever an existing connection closes.

Use `--firehose-rate-limit-bucket-size=50` and `--firehose-rate-limit-bucket-fill-rate=1s` to allow 50 connections instantly, and another connection every second.
Note that when the server is above the limit, it waits 500ms before it returns codes.Unavailable to the client, forcing a minimal back-off.

### Fixed

* Substreams `RpcCall` object are now validated before being performed to ensure they are correct.
* Substreams `RpcCall` JSON-RPC code `-32602` is now treated as a deterministic error (invalid request).
* `tools compare-blocks` now correctly handle segment health reporting and properly prints all differences with `-diff`.
* `tools compare-blocks` now ignores 'unknown fields' in the protobuf message, unless `--include-unknown-fields=true`
* `tools compare-blocks` now ignores when a block bundle contains the 'last block of previous bundle' (a now-deprecated feature)

### Added

* support for "requester pays" buckets on Google Storage in url, ex: `gs://my-bucket/path?project=my-project-id`
* substreams were also bumped to current March 1st develop HEAD

## v1.3.3

### Changed

* Increased gRPC max received message size accepted by Firehose and Substreams gRPC endpoints to 25 MiB.

### Removed

* Command `fireeth init` has been removed, this was a leftover from another time and the command was not working anyway.

### Added

* flag `common-auto-max-procs` to optimize go thread management using github.com/uber-go/automaxprocs
* flag `common-auto-mem-limit-percent` to specify GOMEMLIMIT based on a percentage of available memory

## v1.3.2

### Updated

* Updated to Substreams version `v0.2.0` please refer to [release page](https://github.com/streamingfast/substreams/releases/tag/v0.2.0) for further info about Substreams changes.

### Changed

* **Breaking** Config value `substreams-stores-save-interval` and `substreams-output-cache-save-interval` have been merged together as a single value to avoid potential bugs that would arise when the value is different for those two. The new configuration value is called `substreams-cache-save-interval`.

    *  To migrate, remove usage of `substreams-stores-save-interval: <number>` and `substreams-output-cache-save-interval: <number>` if defined in your config file and replace with `substreams-cache-save-interval: <number>`, if you had two different value before, pick the biggest of the two as the new value to put. We are currently setting to `1000` for Ethereum Mainnet.

### Fixed

* Fixed various issues with `fireeth tools check merged-blocks`
    * The `stopWalk` error is not reported as a real `error` anymore.
    * `Incomplete range` should now be printed more accurately.

## v1.3.1

* Release made to fix our building workflows, nothing different than [v1.3.0](#v130).

## v1.3.0

### Changed

* Updated to Substreams `v0.1.0`, please refer to [release page](https://github.com/streamingfast/substreams/releases/tag/v0.1.0) for further info about Substreams changes.

    > **Warning** The state output format for `map` and `store` modules has changed internally to be more compact in Protobuf format. When deploying this new version and using Substreams feature, previous existing state files should be deleted or deployment updated to point to a new store location. The state output store is defined by the flag `--substreams-state-store-url` flag.

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
