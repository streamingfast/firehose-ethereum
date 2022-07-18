#!/usr/bin/env bash

docker build --no-cache -f Dockerfile --tag sfeth-local:latest .
docker run --rm --env ETH_MAINNET_RPC="$ETH_MAINNET_RPC" -p 9000:9000 sfeth-local:latest
