# Ethereum on StreamingFast
[![reference](https://img.shields.io/badge/godoc-reference-5272B4.svg?style=flat-square)](https://pkg.go.dev/github.com/streamingfast/sf-ethereum)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## Requirements (clone repos, build stuff...)

### Install Geth

```
git clone git@github.com:streamingfast/go-ethereum.git
cd go-ethereum
git checkout release/geth-1.10.x-dm
go install ./cmd/geth
go install ./cmd/bootnode
```

### Install sfeth

```
git clone git@github.com:streamingfast/sf-ethereum.git
cd sf-ethereum
go install ./cmd/sfeth
```

## Quickstart, connecting to an existing chain

* Start from a clean folder

* Create a file named `sf.yaml` and put the following content:

  ```
  start:
  args:
  - merger
  - firehose
  - mindreader-node
  - relayer
  flags:
    common-chain-id: "1"
    common-network-id: "1"
    mindreader-node-bootstrap-data-url: ./mindreader/genesis.json
    mindreader-node-log-to-zap: false
    mindreader-node-arguments: "+--bootnodes=enode://<enode1>@<ip>:<port>,enode://<enode2>@<ip>:<port>"
  ```

  **Note** Up to date boot nodes info for Geth supported network(s) can be found [here](https://github.com/ethereum/go-ethereum/blob/master/params/bootnodes.go).

* Create a folder `mindreader`

* Copy the `genesis.json` file of the chain into the `mindreader` folder.

  **Note** It's possible to use `geth dumpgenesis` to dump actual genesis file to disk
    * Mainnet - `geth --mainnet dumpgenesis > ./mindreader/genesis.json`
    * Ropsten - `geth --ropsten dumpgenesis > ./mindreader/genesis.json`
    * Goerli - `geth --goerli dumpgenesis > ./mindreader/genesis.json`
    * Rinkeby - `geth --rinkeby dumpgenesis > ./mindreader/genesis.json`

* `sfeth start -vv`

  **Note** It's recommended to launch with `-vv` the first time to more easily see what's happening under the hood.

* Wait around a minute leaving enough time for the `Geth` process to start the syncing process. You should then have some merged blocks under `./sf-data/storage/merged-blocks`. You should also be able to test that Firehose is able to stream some blocks to you.

  `grpcurl -insecure -import-path ../proto -import-path ../proto-ethereum -proto dfuse/ethereum/codec/v1/codec.proto -proto dfuse/bstream/v1/bstream.proto -d '{"start_block_num": -1}' 127.0.0.1:13042 dfuse.bstream.v1.BlockStreamV2.Blocks`

  **Note** You will need to have [grpcurl](https://github.com/fullstorydev/grpcurl) and a clone of both https://github.com/streamingfast/proto and https://github.com/streamingfast/proto-ethereum, we assume they are sibling of the folder you are currently in, adjust `-import-path ...` flags in the command above to where the files are located.

## Contributing

**Issues and PR in this repo related strictly to the Ethereum on StreamingFast.**

Report any protocol-specific issues in their
[respective repositories](https://github.com/streamingfast/streamingfast#protocols)

**Please first refer to the general
[StreamingFast contribution guide](https://github.com/streamingfast/streamingfast/blob/master/CONTRIBUTING.md)**,
if you wish to contribute to this code base.

This codebase uses unit tests extensively, please write and run tests.

## License

[Apache 2.0](LICENSE)
