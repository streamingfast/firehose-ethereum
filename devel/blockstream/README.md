# Environment

This devel env is to boot the `blockstream` service

It requires a port-forward like this:

    kc port-forward svc/relayer 9002:9000


### grpcurl invokations

```
grpcurl -import-path ~/streamingfast/proto-ethereum -import-path ~/streamingfast/proto -proto dfuse/ethereum/codec/v1/codec.proto -proto dfuse/bstream/v1/bstream.proto -plaintext -d '{"start_block_num": -1, "decoded": true}' localhost:13044 dfuse.bstream.v1.BlockStreamV2.Blocks
```

### Through the dev1 endpoint:

```
grpcurl -import-path ~/streamingfast/proto-ethereum -import-path ~/streamingfast/proto -proto dfuse/ethereum/codec/v1/codec.proto -proto dfuse/bstream/v1/bstream.proto -d '{"start_block_num": -1, "decoded": true}' dev1-eth.api.streamingfast.dev:443 dfuse.bstream.v1.BlockStreamV2.Blocks
```
