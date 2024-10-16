# Firehose Ethereum Instrumentation

Firehose provides **two** mechanisms to extract data from an ethereum-like blockchain to `sf.ethereum.type.v2.Block` message:

1. Running an instrumented node (data is extracted while the node actually executes the transactions) -> produces `EXTENDED` blocks
2. Polling an standard node with a series of RPC calls (ex: `eth_getBlock`) -> produces BASE blocks

The "Extended" vs "Base" distinction appears in the "DetailLevel" field of the Block message 
  ref: https://github.com/streamingfast/firehose-ethereum/blob/develop/proto/sf/ethereum/type/v2/type.proto#L85

## Instrumented nodes

### Description 

Most instrumented nodes for Ethereum-like networks have parts of their codebase coming from geth (https://github.com/ethereum/go-ethereum)

Nodes that are based on geth versions prior to [v1.14.0](https://github.com/ethereum/go-ethereum/releases/tag/v1.14.0) contain a lot of Firehose-specific instrumentation hooks send each 'event' to the firehose reader which then assembles the block. (firehose protocol < `3.0`)

Nodes that are based on `geth` versions >= v1.14.0 can use the "Live tracing" feature (https://github.com/ethereum/go-ethereum/blob/master/core/tracing/CHANGELOG.md) to send fully-formed blocks to the firehose reader (firehose protocol == `3.0`)

### Supported networks status

| Protocol | Networks (not exhaustive)            | Upstream has v1.14 tracing   | Firehose protocol version |
| -------- | ------------------------------------ | ----------------------------------------- | -------------|
| Ethereum | mainnet<br>sepolia (testnet)<br>holesky (testnet) | Yes                          | fh2.4        |
| BSC      | bsc (aka bnb, mainnet)<br>chapel (testnet) | No (Patched in SF github)           | fh2.5        | 
| Polygon  | matic (mainnet)<br>amoy (testnet)  | Yes (Coming in v1.5.0)                      | fh2.4        | 
| Arbitrum | arbitrum-one*<br>arbitrum-nova<br>arbitrum-sepolia  | No (Patched in SF github)  | fh3.0        |
| Optimism | optimism (mainnet)**<br>optimism-sepolia<br>base<br>base-sepolia | Yes           | fh3.0        |
| SEI      | sei-mainnet                          | No (Patched in SEI codebase)              | fh3.0        |

 * `*` arbitrum-one is not supported by the instrumented node below block 22207818 (pre-nitro)
 * `**` optimism mainnet is not supported by the instrumented node below block 105235064

## RPC-polled

Most ethereum-like networks can work directly using the RPC poller.

This is useful for 
* networks without a full instrumentation (ex: Avalanche, fuji)
* segments of networks that don't support instrumented nodes (ex: early arb-one and optimism-mainnet)