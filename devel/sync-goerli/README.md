## Syncing Goerli

To sync Goerli, an Ethereum Beacon Chain client is required.

This means that starting `firehose-ethereum` alone is not enough to sync a chain, you also need a Beacon Chain client also called an Ethereum Consensus Client. We use here https://github.com/sigp/lighthouse as our consensus client.

In this folder, you will a script to run the consensus client [consensus.sh](./consensus.sh) and a config file [sync-goerli.yaml](./sync-goerli.yaml) that starts the full Firehose on Ethereum stack locally.

The file [jwt.txt](./jwt.txt) is used by both end to communicate with each other, it's hard-coded right now to show case proper syncing.

> **Warning** This file being public, it's a compromised secret so it should not be used in your production environment, generate a new one.

#### Consensus Client (`lighthouse`)

The first one is there to have the Ethereum Consensus Client working, in a first terminal, you can start it with:

```bash
./consensus.sh
```

Two environment variables can tweak it:

| Environment Variable | Description |
| - | - |
| `LIGHTHOUSE_BIN` | Absolute path where to find `lighthouse` binary, defaults to `lighthouse` which will be picked in your `PATH` |
| `CHECKPOINT_SYNC_URL` | URL where `lighthouse` can reach to bootstrap the consensus client database from a pre-made checkpoint, sets `--checkpoint-sync-url` flag on `lighthouse` startup. A community maintained list of public checkpoint sync URLs can be seen https://eth-clients.github.io/checkpoint-sync-endpoints/. **Important** if you already have some data for your consensus client, `--checkpoint-sync-url` has no effect, delete `cs-data` folder first and then start it back with the environment variable |

Wait until the consensus client is synced properly, then proceed to launch execution client syncing.

> **Note** If you want to start "fresh" again, stop current running instance, delete the folder `cs-data` and restart the [consensus.sh](./consensus.sh) script.

#### Execution Client (`geth`)

You need to have Firehose instrumented `geth` binary to properly sync with the network and produce Firehose blocks. Go to https://github.com/streamingfast/go-ethereum/releases, find the most recent release tagged `geth-<version>-fh2.2` and download it (it pre-built binary is not available for your platform, you will need to compile it from source using branch https://github.com/streamingfast/go-ethereum/tree/release/geth-1.11.x-fh2).

Once downloaded, ensure it's available in your `PATH` environment variable.

### Firehose Ethereum (`fireeth`)

Once you have the consensus client synced and `geth` available, download latest release of `fireeth` binary at https://github.com/streamingfast/firehose-ethereum/releases. Again, once downloaded, ensure it's available in your `PATH` environment variable.

The [sync-goerli.yaml](./sync-goerli.yaml) is already configured so the the consensus client can connect to it. Go in the folder where this readme is contained (so [here](.)) and run:

```bash
fireeth -c sync-goerli.yaml start
```

> **Note** If you want to start "fresh" again, stop current running instance, delete the folder `sf-data` and restart the command above.

It will take some time initially to properly synchronize the execution client and the consensus client, you should see in the logs line like:

```
2022-10-19T11:33:45.338-0400 INFO (reader.geth) syncing beacon headers                   downloaded=2,276,864 left=5,460,606 eta=30m16.659s
```

This indicates that `geth` is still synchronizing and it's not ready to start synchronizing. When this is completed, execution client should start sync execution blocks and Firehose blocks will start to generate as well.

> **Note** If you do not see `syncing beacon headers` log line, it means you have problem finding peer(s) that are willing to connect to you, this is especially true on test network where usually less peers are available. Read https://geth.ethereum.org/docs/fundamentals/peer-to-peer for some potential solutions and workaround.

#### `stdin` mode

Firehose on Ethereum by default starts, supervises and manages the `geth` process required to sync with the Ethereum network. But `fireeth` also have a mode called `stdin` where data is actually received from the standard input pipe directly which means that you manage `geth` yourself. To switch to `stdin` mode, in the [sync-goerli.yaml](./sync-goerli.yaml), change the line `- reader-node` (within the `args` list) to `- reader-node-stdin` and also remove the `reader-node-arguments` parameter (so remove from line 11 to end of file) . Then instead of using `fireeth -c sync-goerli.yaml start` to start everything, use:

```bash
geth --networkid=1 --datadir=./sf-data/reader/data --ipcpath=./sf-data/reader/ipc --port=30305 --http --http.api=eth,net,web3 --http.port=8547 --http.addr=0.0.0.0 "--http.vhosts=*" --goerli --authrpc.jwtsecret=./jwt.txt --authrpc.addr=0.0.0.0 --authrpc.port=9551 "--authrpc.vhosts=*" --http.addr=0.0.0.0 --http.port=9545 "--http.vhosts=*" --ws.port=9546 --port=40303 --firehose-enabled | fireeth -c sync-goerli.yaml start
```