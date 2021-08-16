# Ethereum on StreamingFast
[![reference](https://img.shields.io/badge/godoc-reference-5272B4.svg?style=flat-square)](https://pkg.go.dev/github.com/streamingfast/sf-ethereum)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## Requirements (clone repos, build stuff...)

### Install Geth

```
git clone git@github.com:streamingfast/go-ethereum.git
cd go-ethereum
git checkout deep-mind
go install ./cmd/geth
go install ./cmd/bootnode
```

### Install sfeth

```
git clone git@github.com:streamingfast/sf-ethereum.git
cd sf-ethereum
go install ./cmd/sfeth
```

## Quickstart, with an included miner on a local chain

* Start from a clean folder

* `sfeth init` creates the following structure:
```
./sf.yaml
./miner/
./miner/genesis.json
./miner/password
./mindreader/
./mindreader/genesis.json
```

* `sfeth start`
  * Calls bootstrap/Initiate() which will bootstrap by running `geth` for a few seconds (runs only once by checking that the files {sf-data-dir}/mindreader/data/geth/chaindata/CURRENT and {sf-data-dir}/node/data/geth/chaindata/CURRENT don't exist)
  * Starts the stack

* connect to your chain, create an account, push a transaction

```
geth attach --datadir sf-data/node/data
> personal.newAccount()
> eth.getAccounts(console.log) # note the two account IDs (one of them is the default miner account, the other was just created)
> personal.unlockAccount("0x_previous existing account") # default password is 'secure'
> eth.sendTransaction({from: "0x_previous_existing_account",to: "0x_newly_created_account", value: "74000000000000000"})
```
* Open your browser to Dgraphql and write the correct trx hash in the variables: http://localhost:13023/graphiql/?query=cXVlcnkgKCRoYXNoOiBTdHJpbmchKSB7CiAgdHJhbnNhY3Rpb24oaGFzaDogJGhhc2gpIHsKICAgIGhhc2gKICAgIGZyb20KICAgIHRvCiAgICB2YWx1ZShlbmNvZGluZzogRVRIRVIpCiAgICBnYXNMaW1pdAogICAgZ2FzUHJpY2UoZW5jb2Rpbmc6IEVUSEVSKQogICAgZmxhdENhbGxzIHsKICAgICAgZnJvbQogICAgICB0bwogICAgICB2YWx1ZQogICAgICBpbnB1dERhdGEKICAgICAgcmV0dXJuRGF0YQogICAgICBiYWxhbmNlQ2hhbmdlcyB7CiAgICAgICAgYWRkcmVzcwogICAgICAgIG5ld1ZhbHVlKGVuY29kaW5nOiBFVEhFUikKICAgICAgICByZWFzb24KICAgICAgfQogICAgICBsb2dzIHsKICAgICAgICBhZGRyZXNzCiAgICAgICAgdG9waWNzCiAgICAgICAgZGF0YQogICAgICB9CiAgICAgIHN0b3JhZ2VDaGFuZ2VzIHsKICAgICAgICBrZXkKICAgICAgICBvbGRWYWx1ZQogICAgICAgIG5ld1ZhbHVlCiAgICAgIH0KICAgIH0KICB9Cn0K&variables=eyJoYXNoIjoiZjE2NGIwYWY3ZmRlYzc1ZWEwMDVlODQ1MzY4MTdhMDI1OWM1NjU3ZDhhZTgyYjE0Njg0ZTEyMzU5YmRhODBiOSJ9

## Quickstart, connecting to an existing chain

* Start from a clean folder

* `sfeth init`
  * select 'no'
  * enter the networkID
  * enter the complete enode URL(s) to connect to

* this will create the following structure:

```
./sf.yaml # contains the enodes, networkID, and the default exclusion of 'node' app
./mindreader/ # empty
```

* you MUST copy the genesis.json file from the chain that you want to sync to the ./mindreader folder

* `sfeth start`
  * calls

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
