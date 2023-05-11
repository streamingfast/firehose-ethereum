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

  if [[
    -z "$FIREETH_COMMON_ONE_BLOCK_STORE_URL" ||
    -z "$FIREETH_COMMON_FORKED_BLOCKS_STORE_URL" ||
    -z "$FIREETH_COMMON_MERGED_BLOCKS_STORE_URL" ||
    -z "$FIREETH_SUBSTREAMS_RPC_ENDPOINTS"
  ]]; then
    echo 'To use this config, you must define:'
    echo '- FIREETH_COMMON_ONE_BLOCK_STORE_URL (defines common-one-block-store-url)'
    echo '- FIREETH_COMMON_FORKED_BLOCKS_STORE_URL (defines common-forked-blocks-store-url)'
    echo '- FIREETH_COMMON_MERGED_BLOCKS_STORE_URL (defines common-merged-blocks-store-url)'
    echo '- FIREETH_SUBSTREAMS_RPC_ENDPOINTS (defines substreams-rpc-endpoint)'
    exit 1
  fi

  if [[ $clean == "true" ]]; then
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
}

main "$@"