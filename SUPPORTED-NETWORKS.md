# Firehose Ethereum Instrumentation

Firehose provides TWO mechanisms to extract data from an ethereum-like blockchain to `sf.ethereum.type.v2.Block` message:

1. Running an instrumented node (data is extracted while the EVM actually executes the transactions) -> produces EXTENDED blocks
2. Polling an standard node with a series of RPC calls (ex: get_block) -> produces BASE blocks

The "Extended" vs "Base" distinction appears in the "DetailLevel" field of the Block message 
  ref: https://github.com/streamingfast/firehose-ethereum/blob/develop/proto/sf/ethereum/type/v2/type.proto#L85

## Instrumented nodes

### Description 

Most instrumented nodes for ethereum-like networks have parts of their codebase coming from geth (https://github.com/ethereum/go-ethereum)

Nodes that are based on geth versions prior to [v1.14.0](https://github.com/ethereum/go-ethereum/releases/tag/v1.14.0) contain a lot of firehose-specific instrumentation hooks send each 'event' to the firehose reader which then assembles the block. (firehose protocol < fh3)

Nodes that are based on geth versions >= v1.14.0 can use the "Live tracing" feature (https://github.com/ethereum/go-ethereum/blob/master/core/tracing/CHANGELOG.md) to send fully-formed blocks to the firehose reader (firehose protocol == fh3)

### Supported networks status

| Protocol | Networks                             | Upstream has v1.14 tracing | Firehose protocol version |
| -------- | ------------------------------------ | -------------------------- | ------------------------- |
| Ethereum | Mainnet<br>Sepolia<br>Holesky        | Yes                        | fh2.4                     |
| Polygon  | Matic (mainnet)<br>Mumbai (testnet)  | Yes (Coming in v1.5.0)     | fh2.4                     | 
| Arbitrum | arb-one*<br>sepolia<br>nova          | No (Patched in SF github ) | fh3                       |
| Optimism | Mainnet**<br>Sepolia<br>Base         | Yes                        | fh3                       |

 * `*` arb-one is not supported by the instrumented node below block 22207818
 * `**` optimism mainnet is not supported by the instrumented node below block 105235064

## RPC-polled

Most ethereum-like networks can work directly using the RPC poller
