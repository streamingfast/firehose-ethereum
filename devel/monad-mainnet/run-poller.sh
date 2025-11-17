#!/usr/bin/env bash

if [ -z "$RPC_ENDPOINT" ]; then
  echo "RPC_ENDPOINT environment variable is not set"
  exit 1
fi

FIRST_BLOCK=36215077
DATA_DIR="./firehose-data-monad"

mkdir -p "$DATA_DIR/storage/one-blocks"
mkdir -p "$DATA_DIR/storage/merged-blocks"
mkdir -p "$DATA_DIR/storage/forked-blocks"

fireeth tools poller generic-evm \
  "$RPC_ENDPOINT" \
  "$FIRST_BLOCK" \
  --interval-between-fetch=500ms \
  --parallel-workers=10 \
  -vv \
  | ./start-stack.sh
