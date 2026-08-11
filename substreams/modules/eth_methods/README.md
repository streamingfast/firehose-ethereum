# eth_methods

Diagnostic Substreams package exercising the two RPC WASM extensions a `fireeth` instance
registers, `rpc::eth_call` and `rpc::eth_get_balance`.

Use it to answer "are my Substreams RPC endpoints actually working, and how do they behave under
batching" without having to reach for a real indexing package that happens to make RPC calls.

## Use the packed artifact

The built package is committed at [`substreams/spkgs/eth-methods-v0.1.0.spkg`](../../spkgs/eth-methods-v0.1.0.spkg).
Tools and operators are expected to use that file; the sources here only exist to rebuild it.

```bash
substreams run -e <your-firehose-endpoint> \
  substreams/spkgs/eth-methods-v0.1.0.spkg map_eth_methods \
  -s 21000000 -t +10 -o jsonl
```

`map_eth_methods` needs both extensions. When only one endpoint flag is configured on the serving
instance, run `map_eth_call` or `map_eth_get_balance` on their own instead.

### Caching will fool you

Substreams caches module outputs. Replaying a range that was already processed returns the stored
reports and issues **no RPC at all**, which looks like a passing run against an endpoint that is in
fact down. When measuring RPC behaviour, always move to a range that has never been processed, or
clear the module cache first.

## Modules

| Module | Needs | Output |
| --- | --- | --- |
| `map_transfers` | nothing | ERC-20 and ERC-721 `Transfer` events, decoded. No RPC. |
| `map_eth_call` | `--substreams-rpc-endpoints` | `balanceOf(address)` results |
| `map_eth_get_balance` | `--substreams-rpc-get-balance-endpoints` | account balance results |
| `map_eth_methods` | both flags | both reports side by side |

`map_transfers` is the one to run first when something is off: it decodes the same blocks without
touching an endpoint, so a failure there is a block or decoding problem rather than an RPC one.

### One binary per extension

The `Needs` column above is enforced by the packaging rather than merely documented. A WASM module
resolves its imports when it is **instantiated**, not when it calls them, so an instance that does
not register `rpc::eth_get_balance` refuses to instantiate any binary mentioning it, and refuses it
for every module that binary carries:

```
unknown import: `rpc::eth_get_balance` has not been defined
```

The package therefore ships three binaries, and `substreams.yaml` points each module at the one it
belongs to:

| Binary | Modules | Imports |
| --- | --- | --- |
| `no_rpc.wasm` | `map_transfers`, `map_eth_methods` | neither extension |
| `eth_call.wasm` | `map_eth_call` | `rpc::eth_call` |
| `eth_get_balance.wasm` | `map_eth_get_balance` | `rpc::eth_get_balance` |

A missing endpoint flag then fails the module that needs it and nothing else. `map_eth_methods` is
the exception, and not because of its own binary: it consumes the output of both RPC modules, so it
fails whenever either of them does.

## What gets called

Both methods derive their targets from the block being processed, which is what lets the package
run unchanged against any EVM chain instead of only the ones an address was hardcoded for.

- `eth_getBalance` queries every distinct address the block touches, senders and recipients of
  both transactions and decoded transfers.
- `eth_call` calls `balanceOf(address)` on the ERC-20 contract that emitted the most `Transfer`
  events in the block, for the accounts seen transferring that token. Those accounts hold a
  non-zero balance, so a working call is distinguishable from one answering an all-zeroes payload.

Each *round* performs one single-request invocation **and** one batched invocation, so the default
settings hit at least one call and one batch per block, for each method.

## Parameters

Written as a query string, `call_ratio=0.25&call_batch_size=20`. An unknown key or an unparseable
value is an error rather than a silently ignored default: a typo quietly disabling the calls would
defeat the purpose of the package.

| Parameter | Default | Meaning |
| --- | --- | --- |
| `call_ratio` | `1` | `eth_call` rounds per block. Below 1 it is the inverse of a period instead: `0.25` runs a single round every 4 blocks. `0` disables the method. |
| `balance_ratio` | `1` | Same, for `eth_getBalance`. |
| `call_batch_size` | `10` | Requests packed in the batched `eth_call` invocation. `0` skips that invocation. |
| `balance_batch_size` | `10` | Same, for `eth_getBalance`. |
| `call_to` | *(empty)* | Contract `balanceOf(address)` is called on. Left empty, resolved per block as described above. |
| `balance_block` | `number` | `number` queries the block being processed, which needs an archive node when replaying history. `latest` reaches any node but answers for the chain head instead. |

Block selection is a plain modulo on the block number, so a given range produces the same requests
no matter how it gets split across parallel workers.

```bash
# One eth_call round every 4 blocks, batches of 50, no eth_getBalance at all.
substreams run -e <endpoint> substreams/spkgs/eth-methods-v0.1.0.spkg map_eth_methods \
  -p map_eth_call="call_ratio=0.25&call_batch_size=50" \
  -p map_eth_get_balance="balance_ratio=0" \
  -s 21000000 -t +100 -o jsonl
```

## Networks

`--network` pins `call_to` to that chain's canonical USDC contract, USDC being among the busiest
ERC-20 of each of them:

```bash
substreams run --network base -e <endpoint> substreams/spkgs/eth-methods-v0.1.0.spkg map_eth_call ...
```

| Network | `call_to` | Contract |
| --- | --- | --- |
| `mainnet` | `0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48` | USDC, 6 decimals |
| `base` | `0x833589fcd6edb6e08f4c7c32d4f71b54bda02913` | USDC, 6 decimals |
| `arbitrum-one` | `0xaf88d065e77c8cc2239327c5edb3a432268e5831` | USDC native issuance, 6 decimals |
| `optimism` | `0x0b2c639c533813f4aa9d7837caf62653d097ff85` | USDC native issuance, 6 decimals |
| `matic` | `0x3c499c542cef5e3811e1192ce70d8cc03d5c3359` | USDC native issuance, 6 decimals |
| `bsc` | `0x8ac76a51cc950d9822d68b83fe1ad97b32cd580d` | Binance-Peg USDC, **18** decimals |
| `sepolia` | `0x1c7d4b196cb0c7b01d743fbc6116a902379c7238` | USDC, Circle testnet issuance, 6 decimals |
| `hoodi` | *(unpinned)* | resolved per block, see below |

An unknown `--network` name is a hard error rather than a fallback, which is why the testnets are
listed. `hoodi` is listed without an address on purpose: it has no canonical stablecoin issuance to
point at, and pinning a community token that later goes idle is worse than resolving the target
from each block.

Any other chain is still supported, it just resolves its target per block.

### Quiet chains

A testnet regularly produces blocks with no ERC-20 transfer, and sometimes no transaction at all.
Rather than silently doing nothing there, the modules degrade in steps and say which one they
reached, through `status`, `target_source` and `hints`:

| `target_source` | Meaning |
| --- | --- |
| `PARAMETER` | `call_to` or `--network` pinned it |
| `BUSIEST_ERC20` | the token with the most `Transfer` events in the block |
| `CALLED_CONTRACT` | no transfer in the block, so an address it sent a payload to, hence one holding code. An empty response is expected if that contract is not a token |

`eth_getBalance` degrades the same way, ending on the block's fee recipient, which even an empty
block carries. A block that genuinely offers nothing reports `status: INSUFFICIENT_BLOCK_DATA` with
a `hints` entry naming what was missing — that is a property of the chain, not a failing endpoint,
and the two are never reported the same way.

## Reading the output

`stats` on each report totals the invocations, requests, successes and failures. Each `RpcResult`
carries the queried address, the `failed` flag, the decoded decimal value and the raw hexadecimal
payload, which is enough to tell an unconfigured extension from an endpoint rejecting the call from
one returning garbage.

`status` says whether the block performed requests at all: `PERFORMED`, `SKIPPED_BY_RATIO` when the
ratio excluded it, or `INSUFFICIENT_BLOCK_DATA` when the block carried nothing to build a request
from. `hints` carries plain sentences for anything worth knowing to read the rest, including which
flag is likely missing when every request failed, and the archive node requirement behind
`balance_block=number`.

## Rebuilding

```bash
make          # substreams build, then repack the committed .spkg
make test     # parameter parsing unit tests
```

The handlers live in `src/bin`, one file per binary, and `src/lib.rs` holds only what they share.
Cargo builds every `src/bin` target of a package on a plain `cargo build`, which is what
`substreams build` runs, so the three wasm files come out of a single invocation and no extra
build step is needed. Nothing in the library may reference either extension, directly or
transitively: it is linked into all three binaries, and a reference would put the import back in
every one of them.

`substreams build` regenerates `src/pb` through the buf remote plugins, so it needs network access.
Those bindings are committed precisely so that a plain `cargo build` does not. The ABI bindings
under `src/abi` are the other way around: `build.rs` regenerates them offline on every build and
they are not committed.

Bumping `package.version` in `substreams.yaml` changes the artifact filename; delete the previous
one and update the paths above.
