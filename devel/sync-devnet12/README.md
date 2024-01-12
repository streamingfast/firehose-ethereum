## Syncing

Sync devnet12 by running: 

```
./start.sh
```

## Requirements

### Execution Client (`geth`)

You need to have Firehose instrumented `geth` binary to properly sync with the network and produce Firehose blocks. Go to https://github.com/streamingfast/go-ethereum/releases, find the most recent release tagged `geth-<version>-fh2.4` and download it (it pre-built binary is not available for your platform, you will need to compile it from source using branch https://github.com/streamingfast/go-ethereum/tree/release/geth-1.11.x-fh2).

Once downloaded, ensure it's available in your `PATH` environment variable.

### Firehose Ethereum (`fireeth`)

Once you have the consensus client synced and `geth` available, download latest release of `fireeth` binary at https://github.com/streamingfast/firehose-ethereum/releases. Again, once downloaded, ensure it's available in your `PATH` environment variable.
