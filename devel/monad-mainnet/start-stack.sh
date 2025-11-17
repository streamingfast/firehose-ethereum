#!/usr/bin/env bash
DATA_DIR="./firehose-data-monad"

mkdir -p "$DATA_DIR/storage/one-blocks"
mkdir -p "$DATA_DIR/storage/merged-blocks"
mkdir -p "$DATA_DIR/storage/forked-blocks"

fireeth start \
  --config-file=./monad-mainnet.yaml \
  --reader-node-arguments="" \
  --log-to-file=false
