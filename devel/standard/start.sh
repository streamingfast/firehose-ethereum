#!/usr/bin/env bash

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

clean=
fireeth="$ROOT/../fireeth"

main() {
  pushd "$ROOT" &> /dev/null

  while getopts "hc" opt; do
    case $opt in
      h) usage && exit 0;;
      c) clean=true;;
      \?) usage_error "Invalid option: -$OPTARG";;
    esac
  done
  shift $((OPTIND-1))
  [[ $1 = "--" ]] && shift

  fh_data_dir="$ROOT/firehose-data"
  miner_data_dir="$fh_data_dir/miner"

  # If nodekey is changed, the enode must be updated too because it's generated from the nodekey (put here for reference purposes, do not remove)
  miner_enode="enode://e4d5433fd9e84930cd38028f2fdb1ca8d55bdb7b6a749da57e8aa7fc5d3c146c44c0d129a14cecc7b0f13bb98700bf392dd4fd7c31bf2fe26038d4ba8f5a8e32@[127.0.0.1]:30303"
  miner_nodekey="f5f0aadf436e6b35c5fc00a1b0dbc181113ce4f3c448b73b954fe932c00a1b0d"

  set -e

  if [[ $clean == "true" ]]; then
    rm -rf "$fh_data_dir" &> /dev/null || true

    echo "Creating miner Geth directory from 'bootstrap.tar.gz'"
    mkdir -p "$miner_data_dir"
    tar -C "$miner_data_dir" -xzf "$ROOT/miner/bootstrap.tar.gz"
  fi

  # The sleep is here to give 1s for the process to exists, too fast and the 'kill' is a noop
  trap "trap - SIGTERM && sleep 1 && kill -- -$$" SIGINT SIGTERM EXIT
  geth \
    --config="$ROOT/miner/config.toml" \
    --datadir="$miner_data_dir" \
    --mine \
    --miner.etherbase=0x821b55d8abe79bc98f05eb675fdc50dfe796b7ab \
    --nodekeyhex="$miner_nodekey" \
    --allow-insecure-unlock \
    --password=/dev/null  \
    --unlock=0x821b55d8abe79bc98f05eb675fdc50dfe796b7ab &

  $fireeth -c $(basename $ROOT).yaml start "$@"
}

usage_error() {
  message="$1"
  exit_code="$2"

  echo "ERROR: $message"
  echo ""
  usage
  exit ${exit_code:-1}
}

usage() {
  echo "usage: start.sh [-c]"
  echo ""
  echo "Start $(basename $ROOT) environment."
  echo ""
  echo "Options"
  echo "    -c             Clean actual data directory first"
  echo ""
  echo "Examples"
  echo "   Stream blocks    grpcurl -plaintext -d '{\"start_block_num\": -1}' localhost:8089 sf.firehose.v2.Stream/Blocks"
}

main "$@"
