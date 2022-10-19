## Syncing Goerli

Now that The Merge happen in Goerli, and with recent version of Geth, it appears a consensus client is required to properly sync the chain even for blocks that happened before The Merge.

This means that starting `firehose-ethereum` alone is not enough to sync a chain, you also need a Beacon Chain client also called an Ethereum Consensus Client. We use here https://github.com/sigp/lighthouse as our consensus client.

In this `sync-goerli` config, you will find two scripts:

- [consensus.sh](./consensus.sh)
- [start.sh](./start.sh)

The file `jwt.txt` is used by both end to communicate with each other, it's hard-coded right now to show case proper syncing.

> **Important** This file being public, it's a compromised secret so it should not be used in your production environment, generate a new one.

#### Consensus Client (`lighthouse`)

The first one is there to have the Ethereum Consensus Client working, in a first terminal, you can start it with:

```
./consensus.sh
```

Two environment variables can tweak it:

| Environment Variable | Description |
| - | - |
| `LIGHTHOUSE_BIN` | Absolute path where to find `lighthouse` binary, defaults to `lighthouse` which will be picked in your `PATH` |
| `CHECKPOINT_SYNC_URL` | URL where `lighthouse` can reach to bootstrap the consensus client database from a pre-made checkpoint, sets `--checkpoint-sync-url` flag on `lighthouse` startup. A community maintained list of public checkpoint sync URLs can be seen https://eth-clients.github.io/checkpoint-sync-endpoints/. **Important** if you already have some data for your consensus client, `--checkpoint-sync-url` has no effect, delete `cs-data` folder first and then start it back with the environment variable |

Wait until the consensus client is synced properly, then proceed to launch execution client syncing.

#### Execution Client (`geth`)

Once you have the consensus client synced, the `sync-goerli.yaml` is already configured so the the consensus client can connect to it. Simply run:

```
./start.sh
```

It will take some time initially to properly synchronize the execution client and the consensus client, you should see in the logs line like:

```
2022-10-19T11:33:45.338-0400 INFO (reader.geth) syncing beacon headers                   downloaded=2,276,864 left=5,460,606 eta=30m16.659s
```

This indicates that `geth` is still synchronizing and it's not ready to start synchronizing. When this is completed, execution client should start sync execution blocks and Firehose blocks will start to generate as well.
