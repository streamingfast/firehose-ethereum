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

  set -e

  if [[ $clean == "true" ]]; then
    printf "Are you sure you want to clean the data directory? [y/N] "
    read -r answer
    if [[ $answer != "y" ]]; then
      echo "Aborting"
      exit 0
    fi

    rm -rf sf-data &> /dev/null || true
  fi

  exec $fireeth -c $(basename $ROOT).yaml start "$@"
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
  echo "   Stream blocks    grpcurl -insecure -import-path ../proto -import-path ../proto-ethereum -proto dfuse/ethereum/codec/v1/codec.proto -proto dfuse/bstream/v1/bstream.proto -d '{\"start_block_num\": -1}' localhost:13042 dfuse.bstream.v1.BlockStreamV2.Blocks"
}

main "$@"
